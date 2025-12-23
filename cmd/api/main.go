package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware" // Import standard middleware
	"github.com/go-chi/cors"              // FIX: Import CORS
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	httpAdapter "github.com/lorrc/service-desk-backend/internal/adapters/primary/http"
	mw "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
	"github.com/lorrc/service-desk-backend/internal/adapters/primary/websocket"
	"github.com/lorrc/service-desk-backend/internal/adapters/secondary/email"
	"github.com/lorrc/service-desk-backend/internal/adapters/secondary/postgres"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/config"
	"github.com/lorrc/service-desk-backend/internal/core/ports" // Assuming interface exists here
	"github.com/lorrc/service-desk-backend/internal/core/services"
	"github.com/lorrc/service-desk-backend/internal/infrastructure/logging"
)

func main() {
	// FIX: Wrap logic in run() so defer statements execute properly
	if err := run(); err != nil {
		slog.Error("application startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Load Configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// 2. Initialize Structured Logger
	logger := logging.NewLogger(logging.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Output:      os.Stdout,
		ServiceName: cfg.App.Name,
		Environment: cfg.App.Environment,
	})

	logger.Info("starting service", "version", cfg.App.Version)

	// 3. Initialize Database Pool
	// FIX: Use timeout to prevent hanging if DB is down
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("failed to parse DB URL: %w", err)
	}

	// Apply database configuration
	poolConfig.MaxConns = int32(cfg.Database.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.Database.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.Database.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = cfg.Database.ConnMaxIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to DB: %w", err)
	}
	// FIX: This defer will now actually run because we return error instead of os.Exit
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	logger.Info("database connection established")

	// 4. Initialize Components
	tokenManager := auth.NewTokenManager(cfg.JWT.Secret)
	hub := websocket.NewHub(logger)
	go hub.Run()

	// 5. Rate Limiters
	var generalRateLimiter, authRateLimiter *mw.RateLimiter
	if cfg.RateLimit.Enabled {
		// ... (keep your existing rate limiter config) ...
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

	// 6. Dependency Injection
	errorHandler := httpAdapter.NewErrorHandler(logger)
	defaultOrgID, err := uuid.Parse(cfg.App.DefaultOrgID)
	if err != nil {
		return fmt.Errorf("invalid default org ID: %w", err)
	}

	userRepo := postgres.NewUserRepository(pool)
	ticketRepo := postgres.NewTicketRepository(pool)
	authzRepo := postgres.NewAuthorizationRepository(pool)
	commentRepo := postgres.NewCommentRepository(pool)

	// FIX: Don't use Mock in production
	var notifier ports.Notifier // Use your interface type
	if cfg.App.Environment == "production" {
		// notifier = email.NewSMTPNotifier(cfg.SMTP) // TODO: Implement real SMTP
		logger.Warn("using mock notifier in production")
		notifier = email.NewMockSMTPNotifier(userRepo)
	} else {
		notifier = email.NewMockSMTPNotifier(userRepo)
	}

	authService := services.NewAuthService(userRepo, defaultOrgID)
	authzService := services.NewAuthorizationService(authzRepo)
	ticketService := services.NewTicketService(ticketRepo, authzService, notifier, hub)
	commentService := services.NewCommentService(commentRepo, ticketService, authzService, notifier, hub)

	authHandler := httpAdapter.NewAuthHandler(authService, tokenManager, errorHandler, logger)
	commentHandler := httpAdapter.NewCommentHandler(commentService, errorHandler, logger)
	ticketHandler := httpAdapter.NewTicketHandler(ticketService, commentHandler, errorHandler, logger)
	wsHandler := httpAdapter.NewWebSocketHandler(hub, tokenManager, cfg, logger)
	healthHandler := httpAdapter.NewHealthHandler(pool, cfg.App.Version)

	// 7. Setup Router
	r := chi.NewRouter()

	// FIX: Middleware Order
	r.Use(middleware.RealIP) // 1. Important for Rate Limiting behind proxy
	r.Use(mw.RequestID)
	r.Use(mw.RequestLogger(logger))
	r.Use(mw.RecoveryLogger(logger))

	// FIX: Add CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // TODO: Restrict in production
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	if generalRateLimiter != nil {
		r.Use(generalRateLimiter.Middleware)
	}

	r.Get("/health", healthHandler.HandleHealth)
	r.Get("/health/live", healthHandler.HandleLiveness)
	r.Get("/health/ready", healthHandler.HandleReadiness)

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			if authRateLimiter != nil {
				r.Use(authRateLimiter.Middleware)
			}
			r.Route("/auth", authHandler.RegisterRoutes)
		})

		r.Get("/ws", wsHandler.ServeHTTP)

		r.Group(func(r chi.Router) {
			r.Use(mw.JWTMiddleware(tokenManager))
			r.Route("/tickets", ticketHandler.RegisterRoutes)
		})
	})

	srv := &http.Server{
		Addr:              cfg.Server.Port,
		Handler:           r,
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: 5 * time.Second, // FIX: Prevent Slowloris attacks
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	// 8. Start Server
	go func() {
		logger.Info("server starting", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// 9. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("shutdown signal received", "signal", sig.String())

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
		// We don't exit here, we try to close other resources
	}

	logger.Info("waiting for background tasks to finish...")
	ticketService.Shutdown()
	// hub.Shutdown() // Recommendation: Implement shutdown for websockets

	logger.Info("server shutdown complete")
	return nil
}
