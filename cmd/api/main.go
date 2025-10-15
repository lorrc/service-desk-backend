package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	// Import local packages
	httpAdapter "github.com/lorrc/service-desk-backend/internal/adapters/primary/http"
	httpMiddleware "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
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

	// 3. Initialize Security Components
	tokenManager := auth.NewTokenManager(cfg.JWTSecret)

	// 4. Dependency Injection (Wiring the Hexagon)
	// Repositories (Secondary Adapters)
	userRepo := postgres.NewUserRepository(pool)
	ticketRepo := postgres.NewTicketRepository(pool)

	// Services (Core)
	authService := services.NewAuthService(userRepo)
	ticketService := services.NewTicketService(ticketRepo)

	// Handlers (Primary Adapters)
	authHandler := httpAdapter.NewAuthHandler(authService, tokenManager)
	ticketHandler := httpAdapter.NewTicketHandler(ticketService)

	// 5. Setup Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api/v1", func(r chi.Router) {
		// Public routes for authentication
		r.Route("/auth", authHandler.RegisterRoutes)

		// Protected routes for tickets
		r.Group(func(r chi.Router) {
			r.Use(httpMiddleware.JWTMiddleware(tokenManager))
			r.Route("/tickets", ticketHandler.RegisterRoutes)
		})
	})

	// 6. Start Server
	log.Printf("Server starting on %s", cfg.ServerPort)
	if err := http.ListenAndServe(cfg.ServerPort, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
