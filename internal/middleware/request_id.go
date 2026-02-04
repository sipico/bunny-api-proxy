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
// If the incoming request already has an X-Request-ID header, it will be used.
// Otherwise, a new UUID v4 will be generated.
//
// The request ID is:
// - Stored in the request context (retrievable via GetRequestID)
// - Added to response headers as X-Request-ID
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for existing X-Request-ID header
		id := r.Header.Get("X-Request-ID")

		// If not present or empty, generate a new UUID
		if id == "" {
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

// GetRequestID retrieves the request ID from the context.
// Returns empty string if no request ID is found.
func GetRequestID(ctx context.Context) string {
	id, ok := ctx.Value(requestIDKey).(string)
	if !ok {
		return ""
	}
	return id
}
