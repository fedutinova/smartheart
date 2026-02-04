package common

import (
	"errors"
	"fmt"
)

// Domain errors - use errors.Is() to check
var (
	// Generic errors
	ErrInternal     = errors.New("internal error")
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("already exists")
	ErrBadRequest   = errors.New("bad request")

	// Authentication errors
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")

	// Resource-specific errors
	ErrUserNotFound    = fmt.Errorf("user %w", ErrNotFound)
	ErrRequestNotFound = fmt.Errorf("request %w", ErrNotFound)
	ErrFileNotFound    = fmt.Errorf("file %w", ErrNotFound)
	ErrJobNotFound     = fmt.Errorf("job %w", ErrNotFound)

	// Validation errors
	ErrValidation = errors.New("validation error")
)

// ValidationError represents a validation error with field details
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Is implements errors.Is for ValidationError
func (e ValidationError) Is(target error) bool {
	return target == ErrValidation
}

// WrapNotFound wraps an error as a not found error with context
func WrapNotFound(resource string, err error) error {
	return fmt.Errorf("%s: %w", resource, errors.Join(ErrNotFound, err))
}

// WrapInternal wraps an error as an internal error with context
func WrapInternal(operation string, err error) error {
	return fmt.Errorf("%s: %w", operation, errors.Join(ErrInternal, err))
}

// IsNotFound checks if error is a not found error
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsConflict checks if error is a conflict error
func IsConflict(err error) bool {
	return errors.Is(err, ErrConflict)
}

// IsUnauthorized checks if error is an unauthorized error
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}

// IsForbidden checks if error is a forbidden error
func IsForbidden(err error) bool {
	return errors.Is(err, ErrForbidden)
}

// IsValidation checks if error is a validation error
func IsValidation(err error) bool {
	return errors.Is(err, ErrValidation)
}
