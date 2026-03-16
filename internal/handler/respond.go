package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/fedutinova/smartheart/internal/auth"
	"github.com/google/uuid"
)

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("failed to encode response", "err", err)
	}
}

// writeError writes a structured JSON error response.
func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, APIError{Error: msg})
}

// decodeJSON decodes the request body into v.
// Rejects unknown fields and trailing data to catch malformed requests early.
func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	// Reject trailing garbage after the first JSON value.
	if dec.More() {
		return fmt.Errorf("unexpected data after JSON body")
	}
	return nil
}

// extractUserID extracts and parses the user UUID from JWT claims in the request context.
// Returns uuid.Nil and false if claims are missing or the user ID is invalid.
func extractUserID(r *http.Request) (uuid.UUID, *auth.Claims, bool) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		return uuid.Nil, nil, false
	}
	uid, err := uuid.Parse(claims.UserID)
	if err != nil {
		return uuid.Nil, claims, false
	}
	return uid, claims, true
}

// parseUUID is a convenience wrapper around uuid.Parse.
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
