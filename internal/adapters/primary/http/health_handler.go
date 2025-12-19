package http

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

// HealthChecker defines the interface for health check dependencies
type HealthChecker interface {
	Ping(ctx context.Context) error
}

// HealthHandler handles health check requests
type HealthHandler struct {
	db        HealthChecker
	startTime time.Time
	version   string
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db HealthChecker, version string) *HealthHandler {
	return &HealthHandler{
		db:        db,
		startTime: time.Now(),
		version:   version,
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string           `json:"status"`
	Timestamp string           `json:"timestamp"`
	Version   string           `json:"version,omitempty"`
	Uptime    string           `json:"uptime,omitempty"`
	Checks    map[string]Check `json:"checks,omitempty"`
}

// Check represents an individual health check result
type Check struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// HandleLiveness handles liveness probe requests (is the service running?)
// Used by Kubernetes to know when to restart a container
func (h *HealthHandler) HandleLiveness(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// HandleReadiness handles readiness probe requests (can the service accept traffic?)
// Used by Kubernetes to know when to add the pod to the service
func (h *HealthHandler) HandleReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]Check)
	overallStatus := "healthy"

	// Check database connectivity
	dbCheck := h.checkDatabase(ctx)
	checks["database"] = dbCheck
	if dbCheck.Status != "healthy" {
		overallStatus = "unhealthy"
	}

	response := HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   h.version,
		Uptime:    time.Since(h.startTime).Round(time.Second).String(),
		Checks:    checks,
	}

	statusCode := http.StatusOK
	if overallStatus != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}

// HandleHealth handles detailed health check requests (for monitoring/debugging)
func (h *HealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]Check)
	overallStatus := "healthy"

	// Check database
	dbCheck := h.checkDatabase(ctx)
	checks["database"] = dbCheck
	if dbCheck.Status != "healthy" {
		overallStatus = "degraded"
	}

	// Add memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	response := struct {
		HealthResponse
		Memory struct {
			Alloc      uint64 `json:"alloc_bytes"`
			TotalAlloc uint64 `json:"total_alloc_bytes"`
			Sys        uint64 `json:"sys_bytes"`
			NumGC      uint32 `json:"num_gc"`
		} `json:"memory"`
		Goroutines int `json:"goroutines"`
	}{
		HealthResponse: HealthResponse{
			Status:    overallStatus,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Version:   h.version,
			Uptime:    time.Since(h.startTime).Round(time.Second).String(),
			Checks:    checks,
		},
		Goroutines: runtime.NumGoroutine(),
	}
	response.Memory.Alloc = memStats.Alloc
	response.Memory.TotalAlloc = memStats.TotalAlloc
	response.Memory.Sys = memStats.Sys
	response.Memory.NumGC = memStats.NumGC

	statusCode := http.StatusOK
	if overallStatus == "degraded" {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}

// checkDatabase checks the database connection
func (h *HealthHandler) checkDatabase(ctx context.Context) Check {
	start := time.Now()

	if h.db == nil {
		return Check{
			Status:  "unhealthy",
			Message: "Database not configured",
		}
	}

	err := h.db.Ping(ctx)
	latency := time.Since(start)

	if err != nil {
		return Check{
			Status:  "unhealthy",
			Message: err.Error(),
			Latency: latency.String(),
		}
	}

	return Check{
		Status:  "healthy",
		Latency: latency.String(),
	}
}

// RegisterRoutes registers health check routes
func (h *HealthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.HandleHealth)
	mux.HandleFunc("/health/live", h.HandleLiveness)
	mux.HandleFunc("/health/ready", h.HandleReadiness)
}
