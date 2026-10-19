package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func TestClientIPTrustedProxies(t *testing.T) {
	t.Parallel()
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	tests := []struct{ name, peer, xff, real, want string }{
		{"direct ignores spoof", "192.0.2.1:8000", "203.0.113.1", "203.0.113.2", "192.0.2.1"},
		{"trusted proxy", "10.0.0.1:8000", "192.0.2.1", "", "192.0.2.1"},
		{"stop at untrusted hop", "10.0.0.1:8000", "203.0.113.9, 192.0.2.1, 10.0.0.2", "", "192.0.2.1"},
		{"malformed chain", "10.0.0.1:8000", "spoof, malformed", "", "10.0.0.1"},
		{"single trusted header", "10.0.0.1:8000", "", "192.0.2.1", "192.0.2.1"},
		{"mapped ipv4", "[::ffff:192.0.2.1]:8000", "203.0.113.1", "", "192.0.2.1"},
		{"ipv6 peer", "[2001:db8::1]:8000", "203.0.113.1", "", "2001:db8::1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tc.peer
			r.Header.Set("X-Forwarded-For", tc.xff)
			r.Header.Set("X-Real-IP", tc.real)
			handler := ClientIPMiddleware(trusted)(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if got := getClientIP(request); got != tc.want {
					t.Errorf("client IP = %q, want %q", got, tc.want)
				}
				if request.RemoteAddr != tc.peer {
					t.Error("socket peer was overwritten")
				}
			}))
			handler.ServeHTTP(httptest.NewRecorder(), r)
		})
	}
}

func TestRateLimiterRejectsRotatingForwardedHeaders(t *testing.T) {
	limiter := NewRateLimiter(RateLimiterConfig{RequestsPerSecond: 0, BurstSize: 1, CleanupInterval: time.Hour, TTL: time.Hour})
	t.Cleanup(limiter.Close)
	handler := ClientIPMiddleware(nil)(limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })))
	for index, header := range []string{"198.51.100.1", "198.51.100.2", "arbitrary-bucket"} {
		r := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		r.RemoteAddr = "192.0.2.1:1000"
		r.Header.Set("X-Forwarded-For", header)
		r.Header.Set("X-Real-IP", header)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		want := http.StatusTooManyRequests
		if index == 0 {
			want = http.StatusNoContent
		}
		if w.Code != want {
			t.Errorf("request %d: status %d, want %d", index, w.Code, want)
		}
	}
}
