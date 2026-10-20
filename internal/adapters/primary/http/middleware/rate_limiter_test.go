package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRateLimiterEvictsOnlyInactiveVisitors(t *testing.T) {
	t.Parallel()
	limiter := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 0, BurstSize: 1, CleanupInterval: time.Hour, TTL: time.Minute,
	})
	t.Cleanup(limiter.Close)
	for _, ip := range []string{"expired", "active", "boundary"} {
		require.True(t, limiter.Allow(ip))
	}

	now := time.Now()
	limiter.mu.Lock()
	limiter.visitors["expired"].lastSeen = now.Add(-time.Minute - time.Nanosecond)
	limiter.visitors["active"].lastSeen = now.Add(-time.Hour)
	limiter.visitors["boundary"].lastSeen = now.Add(-time.Minute)
	limiter.mu.Unlock()

	// Rejected requests still count as activity and must not gain a fresh bucket
	// through eviction while they continue sending requests.
	require.False(t, limiter.Allow("active"))
	limiter.removeExpiredVisitors(now, time.Minute)
	limiter.mu.RLock()
	_, expiredExists := limiter.visitors["expired"]
	_, activeExists := limiter.visitors["active"]
	_, boundaryExists := limiter.visitors["boundary"]
	limiter.mu.RUnlock()
	require.False(t, expiredExists)
	require.True(t, activeExists)
	require.True(t, boundaryExists)
	require.False(t, limiter.Allow("active"))
	require.False(t, limiter.Allow("boundary"))
	require.True(t, limiter.Allow("expired"), "an inactive client gets a new bucket after eviction")
}

func TestRateLimiterCleanupWorkerEvictsVisitors(t *testing.T) {
	t.Parallel()
	limiter := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 0, BurstSize: 1, CleanupInterval: time.Millisecond, TTL: time.Hour,
	})
	t.Cleanup(limiter.Close)
	require.True(t, limiter.Allow("expired"))
	require.True(t, limiter.Allow("active"))
	limiter.mu.Lock()
	limiter.visitors["expired"].lastSeen = time.Now().Add(-2 * time.Hour)
	limiter.mu.Unlock()

	require.Eventually(t, func() bool {
		limiter.mu.RLock()
		defer limiter.mu.RUnlock()
		_, exists := limiter.visitors["expired"]
		return !exists
	}, time.Second, time.Millisecond)
	require.False(t, limiter.Allow("active"), "cleanup must preserve the active client's exhausted bucket")
}

func TestRateLimiterConcurrentRequestsRespectSharedBurst(t *testing.T) {
	t.Parallel()
	limiter := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 0, BurstSize: 3, CleanupInterval: time.Millisecond, TTL: time.Hour,
	})
	t.Cleanup(limiter.Close)
	var allowed atomic.Int64
	var requests sync.WaitGroup
	start := make(chan struct{})
	for range 100 {
		requests.Go(func() {
			<-start
			if limiter.Allow("192.0.2.1") {
				allowed.Add(1)
			}
			limiter.removeExpiredVisitors(time.Now(), time.Hour)
		})
	}
	close(start)
	requests.Wait()
	require.Equal(t, int64(3), allowed.Load())
	require.True(t, limiter.Allow("192.0.2.2"), "another client has an independent bucket")
}

func TestRateLimiterCloseWaitsForWorkerAndIsIdempotent(t *testing.T) {
	t.Parallel()
	limiter := NewRateLimiter(DefaultRateLimiterConfig())
	t.Cleanup(limiter.Close)
	var calls sync.WaitGroup
	var returnedBeforeWorker atomic.Int64
	for range 100 {
		calls.Go(func() {
			limiter.Close()
			select {
			case <-limiter.done:
			default:
				returnedBeforeWorker.Add(1)
			}
		})
	}
	calls.Wait()
	require.Zero(t, returnedBeforeWorker.Load())
	limiter.Close()
}

func TestRateLimiterMiddlewareReturnsRetryableJSON(t *testing.T) {
	t.Parallel()
	limiter := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 0, BurstSize: 1, CleanupInterval: time.Hour, TTL: time.Hour,
	})
	t.Cleanup(limiter.Close)
	var downstreamCalls int
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalls++
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	request.RemoteAddr = "192.0.2.1:12345"
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, request)
	require.Equal(t, http.StatusNoContent, first.Code)

	blocked := httptest.NewRecorder()
	handler.ServeHTTP(blocked, request)
	require.Equal(t, http.StatusTooManyRequests, blocked.Code)
	require.Equal(t, "application/json", blocked.Header().Get("Content-Type"))
	require.Equal(t, "1", blocked.Header().Get("Retry-After"))
	var body struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(blocked.Body.Bytes(), &body))
	require.Equal(t, "RATE_LIMITED", body.Code)
	require.NotEmpty(t, body.Error)
	require.Equal(t, 1, downstreamCalls)
}
