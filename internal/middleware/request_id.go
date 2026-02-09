// Package middleware provides HTTP middleware components for the proxy.
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const requestIDKey contextKey = "request-id"

// RequestID is a middleware that generates a unique request ID for each request.
// It adds the ID to the request context and includes it in the response headers.
//
// If the incoming request already has an X-Request-ID header, it will be used only
// if it passes validation (length ≤ 128 chars, alphanumeric + dash/underscore/period).
// Otherwise, a new UUID v4 will be generated.
//
// The request ID is:
// - Stored in the request context (retrievable via GetRequestID)
// - Added to response headers as X-Request-ID
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for existing X-Request-ID header
		id := r.Header.Get("X-Request-ID")

		// If not present, empty, or invalid, generate a new UUID
		if id == "" || !isValidRequestID(id) {
			id = uuid.New().String()
		}

		// Store in request context
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		r = r.WithContext(ctx)

		// Add to response headers
		w.Header().Set("X-Request-ID", id)

		// Call next handler with updated context
		next.ServeHTTP(w, r)
	})
}

// isValidRequestID validates that a request ID is safe to use.
// A valid request ID must:
// - Be at most 128 characters long
// - Contain only alphanumeric characters, dash, underscore, or period
func isValidRequestID(id string) bool {
	if len(id) > 128 {
		return false
	}
	for _, c := range id {
		isAlphanumeric := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		isAllowedSpecial := c == '-' || c == '_' || c == '.'
		if !isAlphanumeric && !isAllowedSpecial {
			return false
		}
	}
	return true
}

// GetRequestID retrieves the request ID from the context.
// Returns empty string if no request ID is found.
func GetRequestID(ctx context.Context) string {
	id, ok := ctx.Value(requestIDKey).(string)
	if !ok {
		return ""
	}
	return id
}

// GetRequestIDContextKey returns the context key used for storing request IDs.
// This is primarily useful for testing purposes.
func GetRequestIDContextKey() contextKey {
	return requestIDKey
}
