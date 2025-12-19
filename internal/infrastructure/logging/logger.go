package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime"
	"time"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// RequestIDKey is the context key for request IDs
	RequestIDKey contextKey = "request_id"
	// UserIDKey is the context key for user IDs
	UserIDKey contextKey = "user_id"
	// OrgIDKey is the context key for organization IDs
	OrgIDKey contextKey = "org_id"
)

// Config holds logger configuration
type Config struct {
	Level       string // debug, info, warn, error
	Format      string // json, text
	Output      io.Writer
	AddSource   bool
	ServiceName string
	Environment string
}

// DefaultConfig returns a default logger configuration
func DefaultConfig() Config {
	return Config{
		Level:       "info",
		Format:      "json",
		Output:      os.Stdout,
		AddSource:   false,
		ServiceName: "service-desk",
		Environment: "development",
	}
}

// NewLogger creates a new structured logger with the given configuration
func NewLogger(cfg Config) *slog.Logger {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize time format
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   a.Key,
					Value: slog.StringValue(a.Value.Time().Format(time.RFC3339Nano)),
				}
			}
			return a
		},
	}

	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}

	var handler slog.Handler
	if cfg.Format == "text" {
		handler = slog.NewTextHandler(output, opts)
	} else {
		handler = slog.NewJSONHandler(output, opts)
	}

	// Wrap with custom handler that adds service metadata
	handler = &contextHandler{
		handler:     handler,
		serviceName: cfg.ServiceName,
		environment: cfg.Environment,
	}

	return slog.New(handler)
}

// contextHandler wraps a slog.Handler to add context values and service metadata
type contextHandler struct {
	handler     slog.Handler
	serviceName string
	environment string
}

func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	// Add service metadata
	r.AddAttrs(
		slog.String("service", h.serviceName),
		slog.String("environment", h.environment),
	)

	// Add context values if present
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		r.AddAttrs(slog.String("request_id", requestID))
	}
	if userID, ok := ctx.Value(UserIDKey).(string); ok && userID != "" {
		r.AddAttrs(slog.String("user_id", userID))
	}
	if orgID, ok := ctx.Value(OrgIDKey).(string); ok && orgID != "" {
		r.AddAttrs(slog.String("org_id", orgID))
	}

	return h.handler.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{
		handler:     h.handler.WithAttrs(attrs),
		serviceName: h.serviceName,
		environment: h.environment,
	}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{
		handler:     h.handler.WithGroup(name),
		serviceName: h.serviceName,
		environment: h.environment,
	}
}

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithUserID adds a user ID to the context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// WithOrgID adds an organization ID to the context
func WithOrgID(ctx context.Context, orgID string) context.Context {
	return context.WithValue(ctx, OrgIDKey, orgID)
}

// GetRequestID retrieves the request ID from context
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// LoggerFromContext returns a logger with context values pre-populated
func LoggerFromContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	attrs := []any{}

	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		attrs = append(attrs, "request_id", requestID)
	}
	if userID, ok := ctx.Value(UserIDKey).(string); ok && userID != "" {
		attrs = append(attrs, "user_id", userID)
	}
	if orgID, ok := ctx.Value(OrgIDKey).(string); ok && orgID != "" {
		attrs = append(attrs, "org_id", orgID)
	}

	if len(attrs) > 0 {
		return logger.With(attrs...)
	}
	return logger
}

// LogPanic logs panic information and stack trace
func LogPanic(logger *slog.Logger, panicValue any) {
	// Capture stack trace
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	stackTrace := string(buf[:n])

	logger.Error("panic recovered",
		"panic", panicValue,
		"stack_trace", stackTrace,
	)
}

// HTTPRequestLogger provides a logger for HTTP request logging
type HTTPRequestLogger struct {
	Logger *slog.Logger
}

// LogRequest logs an HTTP request
func (l *HTTPRequestLogger) LogRequest(
	ctx context.Context,
	method string,
	path string,
	statusCode int,
	duration time.Duration,
	bytesWritten int64,
	clientIP string,
	userAgent string,
) {
	attrs := []any{
		"method", method,
		"path", path,
		"status_code", statusCode,
		"duration_ms", duration.Milliseconds(),
		"bytes_written", bytesWritten,
		"client_ip", clientIP,
		"user_agent", userAgent,
	}

	// Determine log level based on status code
	switch {
	case statusCode >= 500:
		l.Logger.ErrorContext(ctx, "http request", attrs...)
	case statusCode >= 400:
		l.Logger.WarnContext(ctx, "http request", attrs...)
	default:
		l.Logger.InfoContext(ctx, "http request", attrs...)
	}
}
