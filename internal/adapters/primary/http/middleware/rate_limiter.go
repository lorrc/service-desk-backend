package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter provides IP-based rate limiting
type RateLimiter struct {
	visitors  map[string]*visitor
	mu        sync.RWMutex
	rate      rate.Limit
	burst     int
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiterConfig holds rate limiter configuration
type RateLimiterConfig struct {
	RequestsPerSecond float64       // Requests allowed per second
	BurstSize         int           // Maximum burst size
	CleanupInterval   time.Duration // How often to clean up old visitors
	TTL               time.Duration // How long to keep inactive visitors
}

// DefaultRateLimiterConfig returns a sensible default configuration
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		RequestsPerSecond: 10,              // 10 requests per second
		BurstSize:         20,              // Allow burst of 20
		CleanupInterval:   time.Minute,     // Clean up every minute
		TTL:               3 * time.Minute, // Remove after 3 minutes of inactivity
	}
}

// AuthRateLimiterConfig returns a stricter config for auth endpoints
func AuthRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		RequestsPerSecond: 1,               // 1 request per second
		BurstSize:         5,               // Allow burst of 5
		CleanupInterval:   time.Minute,     // Clean up every minute
		TTL:               5 * time.Minute, // Remove after 5 minutes
	}
}

// NewRateLimiter creates a new rate limiter with the given configuration
func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate.Limit(cfg.RequestsPerSecond),
		burst:    cfg.BurstSize,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}

	// Start background cleanup goroutine
	go rl.cleanupVisitors(cfg.CleanupInterval, cfg.TTL)

	return rl
}

// getVisitor returns the rate limiter for the given IP, creating one if necessary
func (rl *RateLimiter) getVisitor(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rl.rate, rl.burst)
		rl.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupVisitors removes old visitors that haven't been seen recently
func (rl *RateLimiter) cleanupVisitors(interval, ttl time.Duration) {
	defer close(rl.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stop:
			return
		case now := <-ticker.C:
			rl.removeExpiredVisitors(now, ttl)
		}
	}
}

func (rl *RateLimiter) removeExpiredVisitors(now time.Time, ttl time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for ip, v := range rl.visitors {
		if now.Sub(v.lastSeen) > ttl {
			delete(rl.visitors, ip)
		}
	}
}

// Close stops the background cleanup worker and waits for it to exit.
// Concurrent or repeated calls are safe.
func (rl *RateLimiter) Close() {
	rl.closeOnce.Do(func() { close(rl.stop) })
	<-rl.done
}

// Allow checks if a request from the given IP is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	return rl.getVisitor(ip).Allow()
}

// Middleware returns an HTTP middleware that rate limits requests
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)

		if !rl.Allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"Too many requests. Please try again later.","code":"RATE_LIMITED"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
