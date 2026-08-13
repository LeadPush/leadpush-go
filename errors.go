package leadpush

import (
	"errors"
	"fmt"
	"time"
)

// APIError is returned when the Leadpush API responds with a non-2xx status.
type APIError struct {
	StatusCode int
	Payload    any
	RawBody    []byte
}

// Error implements error.
func (e *APIError) Error() string {
	return fmt.Sprintf("leadpush: API request failed with status %d", e.StatusCode)
}

// TimeoutError is returned when the SDK's configured request timeout expires.
type TimeoutError struct {
	Timeout time.Duration
	Err     error
}

// Error implements error.
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("leadpush: API request timed out after %s", e.Timeout)
}

// Unwrap returns the underlying context deadline error.
func (e *TimeoutError) Unwrap() error {
	return e.Err
}

// IsUnauthorized reports whether err is an API error with status 401.
func IsUnauthorized(err error) bool {
	return hasStatus(err, 401)
}

// IsForbidden reports whether err is an API error with status 403.
func IsForbidden(err error) bool {
	return hasStatus(err, 403)
}

// IsNotFound reports whether err is an API error with status 404.
func IsNotFound(err error) bool {
	return hasStatus(err, 404)
}

// IsValidation reports whether err is an API error with status 422.
func IsValidation(err error) bool {
	return hasStatus(err, 422)
}

func hasStatus(err error, status int) bool {
	var apiError *APIError
	return errors.As(err, &apiError) && apiError.StatusCode == status
}
