package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebhookIPWhitelist_AllowsYooKassaIP(t *testing.T) {
	handler := WebhookIPWhitelist("shop-123", nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/webhook", http.NoBody)
	req.RemoteAddr = "185.71.76.1:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestWebhookIPWhitelist_BlocksUnknownIP(t *testing.T) {
	handler := WebhookIPWhitelist("shop-123", nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/webhook", http.NoBody)
	req.RemoteAddr = "1.2.3.4:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestWebhookIPWhitelist_SkipsInDevMode(t *testing.T) {
	handler := WebhookIPWhitelist("", nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/webhook", http.NoBody)
	req.RemoteAddr = "1.2.3.4:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestWebhookIPWhitelist_AllowsXForwardedFor(t *testing.T) {
	handler := WebhookIPWhitelist("shop-123", nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/webhook", http.NoBody)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "77.75.153.10, 10.0.0.1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestWebhookIPWhitelist_AllowsExtraCIDR(t *testing.T) {
	// An IP outside the built-in YooKassa ranges is allowed once supplied via
	// the configurable extra-CIDR list.
	handler := WebhookIPWhitelist("shop-123", []string{"203.0.113.0/24"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/webhook", http.NoBody)
	req.RemoteAddr = "203.0.113.65:443"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestWebhookIPWhitelist_InvalidExtraCIDRIgnored(t *testing.T) {
	// A malformed operator-supplied CIDR must not crash or block the webhook;
	// the built-in ranges keep working.
	handler := WebhookIPWhitelist("shop-123", []string{"not-a-cidr"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/webhook", http.NoBody)
	req.RemoteAddr = "185.71.76.1:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}
