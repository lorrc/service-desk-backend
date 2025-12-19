package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	httpAdapter "github.com/lorrc/service-desk-backend/internal/adapters/primary/http"
	mw "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
	"github.com/lorrc/service-desk-backend/internal/adapters/primary/websocket"
	"github.com/lorrc/service-desk-backend/internal/adapters/secondary/email"
	"github.com/lorrc/service-desk-backend/internal/adapters/secondary/postgres"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/config"
	"github.com/lorrc/service-desk-backend/internal/core/services"
	"github.com/lorrc/service-desk-backend/internal/infrastructure/logging"
)

func main() {
	// 1. Load Configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// 2. Initialize Structured Logger
	logger := logging.NewLogger(logging.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Output:      os.Stdout,
		ServiceName: cfg.App.Name,
		Environment: cfg.App.Environment,
	})

	logger.Info("starting service",
		"version", cfg.App.Version,
		"environment", cfg.App.Environment,
	)

	// 3. Initialize Database Pool
	ctx := context.Background()
	poolConfig, err := pgxpool.ParseConfig(cfg.Database.URL)
	if err != nil {
		logger.Error("failed to parse database URL", "error", err)
		os.Exit(1)
	}

	// Apply database configuration
	poolConfig.MaxConns = int32(cfg.Database.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.Database.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.Database.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = cfg.Database.ConnMaxIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("database ping failed", "error", err)
		os.Exit(1)
	}
	logger.Info("database connection established")

	// 4. Initialize Security & Real-time Components
	tokenManager := auth.NewTokenManager(cfg.JWT.Secret)
	hub := websocket.NewHub(logger)
	go hub.Run()

	// 5. Initialize Rate Limiters
	var generalRateLimiter, authRateLimiter *mw.RateLimiter
	if cfg.RateLimit.Enabled {
		generalRateLimiter = mw.NewRateLimiter(mw.RateLimiterConfig{
			RequestsPerSecond: cfg.RateLimit.RequestsPerSecond,
			BurstSize:         cfg.RateLimit.BurstSize,
			CleanupInterval:   time.Minute,
			TTL:               3 * time.Minute,
		})

		authRateLimiter = mw.NewRateLimiter(mw.RateLimiterConfig{
			RequestsPerSecond: cfg.RateLimit.AuthRPS,
			BurstSize:         cfg.RateLimit.AuthBurst,
			CleanupInterval:   time.Minute,
			TTL:               5 * time.Minute,
		})
	}

	// 6. Dependency Injection (Wiring the Hexagon)

	// Error Handler
	errorHandler := httpAdapter.NewErrorHandler(logger)

	// Parse Default Org ID from config
	defaultOrgID, err := uuid.Parse(cfg.App.DefaultOrgID)
	if err != nil {
		logger.Error("invalid default organization id in configuration", "error", err)
		os.Exit(1)
	}
	// Repositories (Secondary Adapters)
	userRepo := postgres.NewUserRepository(pool)
	ticketRepo := postgres.NewTicketRepository(pool)
	authzRepo := postgres.NewAuthorizationRepository(pool)
	commentRepo := postgres.NewCommentRepository(pool)

	// Notifier (Secondary Adapter)
	notifier := email.NewMockSMTPNotifier(userRepo)

	// Services (Core)
	authService := services.NewAuthService(userRepo, defaultOrgID)
	authzService := services.NewAuthorizationService(authzRepo)
	ticketService := services.NewTicketService(ticketRepo, authzService, notifier, hub)
	commentService := services.NewCommentService(commentRepo, ticketService, authzService, notifier, hub)

	// Handlers (Primary Adapters)
	authHandler := httpAdapter.NewAuthHandler(authService, tokenManager, errorHandler, logger)
	commentHandler := httpAdapter.NewCommentHandler(commentService, errorHandler, logger)
	ticketHandler := httpAdapter.NewTicketHandler(ticketService, commentHandler, errorHandler, logger)
	wsHandler := httpAdapter.NewWebSocketHandler(hub, tokenManager, cfg, logger)
	healthHandler := httpAdapter.NewHealthHandler(pool, cfg.App.Version)

	// 7. Setup Router
	r := chi.NewRouter()

	// Global middleware
	r.Use(mw.RequestID)
	r.Use(mw.RequestLogger(logger))
	r.Use(mw.RecoveryLogger(logger))

	// Apply general rate limiting if enabled
	if generalRateLimiter != nil {
		r.Use(generalRateLimiter.Middleware)
	}

	// Health check endpoints (outside /api/v1 for standard probe paths)
	r.Get("/health", healthHandler.HandleHealth)
	r.Get("/health/live", healthHandler.HandleLiveness)
	r.Get("/health/ready", healthHandler.HandleReadiness)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public auth routes with stricter rate limiting
		r.Group(func(r chi.Router) {
			if authRateLimiter != nil {
				r.Use(authRateLimiter.Middleware)
			}
			r.Route("/auth", authHandler.RegisterRoutes)
		})

		// WebSocket route (Authentication is handled inside the handler)
		r.Get("/ws", wsHandler.ServeHTTP)

		// Protected REST routes
		r.Group(func(r chi.Router) {
			r.Use(mw.JWTMiddleware(tokenManager))
			r.Route("/tickets", ticketHandler.RegisterRoutes)
		})
	})

	// 8. Start Server with Graceful Shutdown
	srv := &http.Server{
		Addr:         cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.Info("server starting", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("shutdown signal received", "signal", sig.String())

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	// Graceful shutdown
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
		os.Exit(1)
	}

	logger.Info("server shutdown complete")
}
