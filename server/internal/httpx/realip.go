package httpx

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type ipKey struct{}

func ClientIP(ctx context.Context) string { value, _ := ctx.Value(ipKey{}).(string); return value }
func trusted(address netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(address.Unmap()) {
			return true
		}
	}
	return false
}
func RealIP(getProxies func() []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		peer, err := netip.ParseAddr(host)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		prefixes := []netip.Prefix{}
		for _, raw := range getProxies() {
			if prefix, err := netip.ParsePrefix(raw); err == nil {
				prefixes = append(prefixes, prefix)
			}
		}
		address := peer.Unmap()
		if trusted(address, prefixes) {
			chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
			if len(chain) <= 32 {
				for i := len(chain) - 1; i >= 0; i-- {
					candidate, err := netip.ParseAddr(strings.TrimSpace(chain[i]))
					if err != nil {
						address = peer.Unmap()
						break
					}
					address = candidate.Unmap()
					if !trusted(address, prefixes) {
						break
					}
				}
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ipKey{}, address.String())))
	})
}
