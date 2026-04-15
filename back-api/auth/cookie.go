package auth

import (
	"net/http"
	"time"

	"github.com/fedutinova/smartheart/back-api/config"
)

const refreshCookieName = "refresh_token"

// SetRefreshTokenCookie writes an httpOnly cookie carrying the refresh token.
func SetRefreshTokenCookie(w http.ResponseWriter, token string, maxAge time.Duration, cfg config.CookieConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/v1/auth",
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
		Domain:   cfg.Domain,
	})
}

// ClearRefreshTokenCookie removes the refresh-token cookie.
func ClearRefreshTokenCookie(w http.ResponseWriter, cfg config.CookieConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/v1/auth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
		Domain:   cfg.Domain,
	})
}

// RefreshTokenFromCookie extracts the refresh token from the request cookie.
// Returns empty string if the cookie is absent.
func RefreshTokenFromCookie(r *http.Request) string {
	c, err := r.Cookie(refreshCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}
