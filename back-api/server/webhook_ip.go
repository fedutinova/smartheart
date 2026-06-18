package server

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
)

// yooKassaCIDRs is the list of IP ranges that YooKassa sends webhooks from.
// https://yookassa.ru/developers/using-api/webhooks#ip
//
// NOTE: this list goes stale when YooKassa adds sender IPs — when that happens
// real webhooks get 403'd and payments silently never confirm. Prefer adding
// new ranges via the YOOKASSA_WEBHOOK_EXTRA_CIDRS env (no redeploy) and keep
// this list in sync with the official docs. The webhook is additionally
// protected by re-fetching the payment status from the YooKassa API before
// confirming, so the IP check is defense-in-depth, not the sole gate.
var yooKassaCIDRs = []string{
	"185.71.76.0/27",
	"185.71.77.0/27",
	"77.75.153.0/25",
	"77.75.156.11/32",
	"77.75.156.35/32",
	"77.75.154.128/25",
	"2a02:5180::/32",
	// Observed in production 2026-06 (retry pattern from YooKassa); not yet in
	// the published doc list above. Reconcile with official ranges.
	"159.194.220.0/24",
}

// parseCIDRs parses the given CIDR strings, skipping (and logging) invalid ones
// so a bad operator-supplied entry can't take the webhook down.
func parseCIDRs(cidrs []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			slog.Warn("Ignoring invalid webhook CIDR", "cidr", cidr, "error", err)
			continue
		}
		nets = append(nets, network)
	}
	return nets
}

// WebhookIPWhitelist returns middleware that only allows requests from YooKassa
// IPs (built-in ranges plus extraCIDRs from config). In development mode (when
// shopID is empty), all IPs are allowed.
func WebhookIPWhitelist(shopID string, extraCIDRs []string) func(http.Handler) http.Handler {
	allowed := parseCIDRs(yooKassaCIDRs)
	allowed = append(allowed, parseCIDRs(extraCIDRs)...)
	if len(extraCIDRs) > 0 {
		slog.Info("Webhook IP allowlist loaded extra CIDRs", "count", len(extraCIDRs))
	}

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

			for _, network := range allowed {
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
