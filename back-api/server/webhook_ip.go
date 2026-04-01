package server

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
)

// yooKassaCIDRs is the list of IP ranges that YooKassa sends webhooks from.
// https://yookassa.ru/developers/using-api/webhooks#ip
var yooKassaCIDRs = []string{
	"185.71.76.0/27",
	"185.71.77.0/27",
	"77.75.153.0/25",
	"77.75.156.11/32",
	"77.75.156.35/32",
	"77.75.154.128/25",
	"2a02:5180::/32",
}

// yooKassaNets is the parsed list, built once at init.
var yooKassaNets []*net.IPNet

func init() {
	for _, cidr := range yooKassaCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic("invalid YooKassa CIDR: " + cidr)
		}
		yooKassaNets = append(yooKassaNets, network)
	}
}

// WebhookIPWhitelist returns middleware that only allows requests from YooKassa IPs.
// In development mode (when shopID is empty), all IPs are allowed.
func WebhookIPWhitelist(shopID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip IP check in dev mode (no YooKassa configured).
			if shopID == "" {
				next.ServeHTTP(w, r)
				return
			}

			ipStr := extractIP(r)
			ip := net.ParseIP(ipStr)
			if ip == nil {
				slog.Warn("Webhook rejected: invalid IP", "ip", ipStr)
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			for _, network := range yooKassaNets {
				if network.Contains(ip) {
					next.ServeHTTP(w, r)
					return
				}
			}

			slog.Warn("Webhook rejected: IP not in YooKassa whitelist", "ip", ipStr)
			http.Error(w, "forbidden", http.StatusForbidden)
		})
	}
}

// extractIP gets the client IP from X-Forwarded-For, X-Real-IP, or RemoteAddr.
func extractIP(r *http.Request) string {
	// chi middleware.RealIP already sets RemoteAddr, but check headers as fallback.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first (leftmost) IP — the original client.
		if first, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
