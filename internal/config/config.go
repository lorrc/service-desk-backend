package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	// Server configuration
	Server ServerConfig

	// Database configuration
	Database DatabaseConfig

	// JWT configuration
	JWT JWTConfig

	// Rate limiting configuration
	RateLimit RateLimitConfig

	// WebSocket configuration
	WebSocket WebSocketConfig

	// Logging configuration
	Logging LoggingConfig

	// Application metadata
	App AppConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled           bool
	RequestsPerSecond float64
	BurstSize         int
	AuthRPS           float64 // Stricter limit for auth endpoints
	AuthBurst         int
}

// WebSocketConfig holds WebSocket configuration
type WebSocketConfig struct {
	AllowedOrigins  []string
	ReadBufferSize  int
	WriteBufferSize int
	PingInterval    time.Duration
	PongWait        time.Duration
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string // debug, info, warn, error
	Format string // json, text
}

// AppConfig holds application metadata
type AppConfig struct {
	Name         string
	Version      string
	Environment  string
	DefaultOrgID string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists (for local development)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:            getEnvOrDefault("SERVER_PORT", ":8080"),
			ReadTimeout:     getDurationOrDefault("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:    getDurationOrDefault("SERVER_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:     getDurationOrDefault("SERVER_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: getDurationOrDefault("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Database: DatabaseConfig{
			URL:             os.Getenv("DATABASE_URL"),
			MaxOpenConns:    getIntOrDefault("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getIntOrDefault("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getDurationOrDefault("DB_CONN_MAX_LIFETIME", 5*time.Minute),
			ConnMaxIdleTime: getDurationOrDefault("DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
		},
		JWT: JWTConfig{
			Secret:          os.Getenv("JWT_SECRET"),
			AccessTokenTTL:  getDurationOrDefault("JWT_ACCESS_TOKEN_TTL", 1*time.Hour),
			RefreshTokenTTL: getDurationOrDefault("JWT_REFRESH_TOKEN_TTL", 7*24*time.Hour),
		},
		RateLimit: RateLimitConfig{
			Enabled:           getBoolOrDefault("RATE_LIMIT_ENABLED", true),
			RequestsPerSecond: getFloatOrDefault("RATE_LIMIT_RPS", 10),
			BurstSize:         getIntOrDefault("RATE_LIMIT_BURST", 20),
			AuthRPS:           getFloatOrDefault("RATE_LIMIT_AUTH_RPS", 1),
			AuthBurst:         getIntOrDefault("RATE_LIMIT_AUTH_BURST", 5),
		},
		WebSocket: WebSocketConfig{
			AllowedOrigins:  getStringSliceOrDefault("WS_ALLOWED_ORIGINS", []string{}),
			ReadBufferSize:  getIntOrDefault("WS_READ_BUFFER_SIZE", 1024),
			WriteBufferSize: getIntOrDefault("WS_WRITE_BUFFER_SIZE", 1024),
			PingInterval:    getDurationOrDefault("WS_PING_INTERVAL", 54*time.Second),
			PongWait:        getDurationOrDefault("WS_PONG_WAIT", 60*time.Second),
		},
		Logging: LoggingConfig{
			Level:  getEnvOrDefault("LOG_LEVEL", "info"),
			Format: getEnvOrDefault("LOG_FORMAT", "json"),
		},
		App: AppConfig{
			Name:         getEnvOrDefault("APP_NAME", "service-desk"),
			Version:      getEnvOrDefault("APP_VERSION", "dev"),
			Environment:  getEnvOrDefault("APP_ENV", "development"),
			DefaultOrgID: getEnvOrDefault("DEFAULT_ORG_ID", "00000000-0000-0000-0000-000000000001"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	var errs []string

	// Required fields
	if c.Database.URL == "" {
		errs = append(errs, "DATABASE_URL is required")
	}

	if c.JWT.Secret == "" {
		errs = append(errs, "JWT_SECRET is required")
	}

	// Security validations
	if c.App.Environment == "production" {
		if len(c.JWT.Secret) < 32 {
			errs = append(errs, "JWT_SECRET must be at least 32 characters in production")
		}

		if len(c.WebSocket.AllowedOrigins) == 0 {
			errs = append(errs, "WS_ALLOWED_ORIGINS must be set in production")
		}
	}

	// Logical validations
	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		errs = append(errs, "DB_MAX_IDLE_CONNS cannot be greater than DB_MAX_OPEN_CONNS")
	}

	if len(errs) > 0 {
		return errors.New("configuration errors:\n  - " + strings.Join(errs, "\n  - "))
	}

	return nil
}

// IsDevelopment returns true if running in development environment
func (c *Config) IsDevelopment() bool {
	return c.App.Environment == "development"
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.App.Environment == "production"
}

// Helper functions

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getFloatOrDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getStringSliceOrDefault(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

// String returns a redacted string representation of the config (safe for logging)
func (c *Config) String() string {
	return fmt.Sprintf(
		"Config{Server: %s, DB: %s, JWT: [REDACTED], RateLimit: %v, Environment: %s}",
		c.Server.Port,
		redactURL(c.Database.URL),
		c.RateLimit.Enabled,
		c.App.Environment,
	)
}

// redactURL redacts sensitive parts of a database URL
func redactURL(url string) string {
	if url == "" {
		return ""
	}
	// Very basic redaction - in production you'd want something more robust
	if idx := strings.Index(url, "@"); idx > 0 {
		return "[REDACTED]" + url[idx:]
	}
	return "[REDACTED]"
}
