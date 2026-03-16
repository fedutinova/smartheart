package service

import "errors"

// ErrTooManyAttempts signals that the caller has been rate-limited.
var ErrTooManyAttempts = errors.New("too many attempts")
