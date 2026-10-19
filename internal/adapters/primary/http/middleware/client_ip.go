package middleware

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type clientIPKey struct{}

// ClientIPMiddleware resolves forwarded addresses only through configured trusted proxies.
// The socket peer remains in RemoteAddr for handlers that need the original connection identity.
func ClientIPMiddleware(trusted []netip.Prefix) func(http.Handler) http.Handler {
	prefixes := append([]netip.Prefix(nil), trusted...)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := resolveClientIP(r, prefixes)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientIPKey{}, ip)))
		})
	}
}

func getClientIP(r *http.Request) string {
	if ip, ok := r.Context().Value(clientIPKey{}).(string); ok {
		return ip
	}
	return peerIP(r.RemoteAddr)
}

func peerIP(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.Unmap().String()
	}
	return host
}

func resolveClientIP(r *http.Request, trusted []netip.Prefix) string {
	peer := peerIP(r.RemoteAddr)
	current, err := netip.ParseAddr(peer)
	if err != nil || !isTrustedProxy(current, trusted) {
		return peer
	}

	forwarded := r.Header.Values("X-Forwarded-For")
	chain := strings.Join(forwarded, ",")
	if chain == "" {
		chain = r.Header.Get("X-Real-IP")
	}
	if chain == "" {
		return peer
	}

	addresses := strings.Split(chain, ",")
	for i := len(addresses) - 1; i >= 0; i-- {
		if !isTrustedProxy(current, trusted) {
			return current.String()
		}
		next, err := netip.ParseAddr(strings.TrimSpace(addresses[i]))
		if err != nil || next.Zone() != "" {
			return peer
		}
		current = next.Unmap()
	}
	return current.String()
}

func isTrustedProxy(ip netip.Addr, trusted []netip.Prefix) bool {
	for _, prefix := range trusted {
		if prefix.Contains(ip.Unmap()) {
			return true
		}
	}
	return false
}
