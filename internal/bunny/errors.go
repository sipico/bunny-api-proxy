// Package bunny provides types and error handling for the bunny.net API client.
package bunny

import (
	"errors"
	"fmt"
)

// APIError represents a structured error from the bunny.net API.
type APIError struct {
	StatusCode int
	ErrorKey   string
	Field      string
	Message    string
}

// Error implements the error interface for APIError.
func (e *APIError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("bunny: %s (field: %s): %s", e.ErrorKey, e.Field, e.Message)
	}
	return fmt.Sprintf("bunny: %s: %s", e.ErrorKey, e.Message)
}

// Sentinel errors for common API error cases.
var (
	ErrUnauthorized = errors.New("bunny: unauthorized (invalid API key)")
	ErrNotFound     = errors.New("bunny: resource not found")
)
