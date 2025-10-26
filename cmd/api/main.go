package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	httpAdapter "github.com/lorrc/service-desk-backend/internal/adapters/primary/http"
	httpMiddleware "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
	"github.com/lorrc/service-desk-backend/internal/adapters/primary/websocket" // <-- New import
	"github.com/lorrc/service-desk-backend/internal/adapters/secondary/email"
	"github.com/lorrc/service-desk-backend/internal/adapters/secondary/postgres"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/config"
	"github.com/lorrc/service-desk-backend/internal/core/services"
)

func main() {
	ctx := context.Background()

	// 1. Load Configuration
	cfg := config.Load()

	// 2. Initialize Database Pool
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Database ping failed: %v\n", err)
	}

	// 3. Initialize Security & Real-time Components
	tokenManager := auth.NewTokenManager(cfg.JWTSecret)
	hub := websocket.NewHub() // <-- New WebSocket Hub
	go hub.Run()              // <-- Run the hub in the background

	// 4. Dependency Injection (Wiring the Hexagon)
	// Repositories (Secondary Adapters)
	userRepo := postgres.NewUserRepository(pool)
	ticketRepo := postgres.NewTicketRepository(pool)
	authzRepo := postgres.NewAuthorizationRepository(pool)
	commentRepo := postgres.NewCommentRepository(pool)

	// Notifier (Secondary Adapter)
	notifier := email.NewMockSMTPNotifier(userRepo)

	// Services (Core)
	authService := services.NewAuthService(userRepo)
	authzService := services.NewAuthorizationService(authzRepo)
	ticketService := services.NewTicketService(ticketRepo, authzService, notifier, hub)                   // <-- Injected hub
	commentService := services.NewCommentService(commentRepo, ticketService, authzService, notifier, hub) // <-- Injected hub

	// Handlers (Primary Adapters)
	authHandler := httpAdapter.NewAuthHandler(authService, tokenManager)
	commentHandler := httpAdapter.NewCommentHandler(commentService)
	ticketHandler := httpAdapter.NewTicketHandler(ticketService, commentHandler)
	wsHandler := httpAdapter.NewWebSocketHandler(hub, tokenManager) // <-- New WebSocket Handler

	// 5. Setup Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api/v1", func(r chi.Router) {
		// Public routes for authentication
		r.Route("/auth", authHandler.RegisterRoutes)

		// WebSocket route (Authentication is handled inside the handler)
		r.Get("/ws", wsHandler.ServeHTTP)

		// Protected REST routes for tickets
		r.Group(func(r chi.Router) {
			r.Use(httpMiddleware.JWTMiddleware(tokenManager))
			r.Route("/tickets", ticketHandler.RegisterRoutes)
		})
	})

	// 6. Start Server with Graceful Shutdown
	srv := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: r,
	}

	go func() {
		log.Printf("Server starting on %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
	log.Println("Server exited properly")
}
