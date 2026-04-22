package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/auth"
	"github.com/fedutinova/smartheart/back-api/service"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("Failed to encode response", "err", err)
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
		return errors.New("unexpected data after JSON body")
	}
	return nil
}

// decodeAndValidate decodes JSON body and runs struct tag validation.
// Returns true on success. On failure it writes an error response and returns false.
func decodeAndValidate(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := decodeJSON(r, v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	if err := validate.Struct(v); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			writeError(w, http.StatusBadRequest, formatValidationErrors(ve))
			return false
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

// formatValidationErrors converts validator errors into a human-readable string.
func formatValidationErrors(ve validator.ValidationErrors) string {
	msgs := make([]string, 0, len(ve))
	for _, fe := range ve {
		field := fe.Field()
		switch fe.Tag() {
		case "required":
			msgs = append(msgs, field+" is required")
		case "min":
			msgs = append(msgs, field+" must be at least "+fe.Param())
		case "max":
			msgs = append(msgs, field+" must be at most "+fe.Param())
		case "email":
			msgs = append(msgs, field+" must be a valid email")
		case "url", "uri":
			msgs = append(msgs, field+" must be a valid URL")
		case "gte":
			msgs = append(msgs, field+" must be >= "+fe.Param())
		case "lte":
			msgs = append(msgs, field+" must be <= "+fe.Param())
		default:
			msgs = append(msgs, field+" failed "+fe.Tag()+" validation")
		}
	}
	return strings.Join(msgs, "; ")
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

// handleServiceError maps service-layer errors to HTTP responses.
func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrTooManyAttempts):
		writeError(w, http.StatusTooManyRequests, "too many attempts, try again later")
	case errors.Is(err, apperr.ErrPaymentRequired):
		writeError(w, http.StatusPaymentRequired, err.Error())
	case apperr.IsValidation(err):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, apperr.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid email or password")
	case errors.Is(err, apperr.ErrInvalidToken):
		writeError(w, http.StatusUnauthorized, "invalid token")
	case apperr.IsConflict(err):
		writeError(w, http.StatusConflict, "already exists")
	case apperr.IsNotFound(err):
		writeError(w, http.StatusNotFound, "not found")
	case apperr.IsForbidden(err):
		writeError(w, http.StatusForbidden, "forbidden")
	default:
		slog.Error("Unhandled service error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
