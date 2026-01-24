package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// contextKey for storing KeyInfo in context
type contextKey string

const keyInfoContextKey contextKey = "keyInfo"

// Middleware returns Chi-compatible middleware for API key validation
func Middleware(v *Validator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract API key from Authorization header
			apiKey := extractBearerToken(r)
			if apiKey == "" {
				writeJSONError(w, http.StatusUnauthorized, "missing API key")
				return
			}

			// Validate the key
			keyInfo, err := v.ValidateKey(r.Context(), apiKey)
			if err != nil {
				if err == ErrInvalidKey {
					writeJSONError(w, http.StatusUnauthorized, "invalid API key")
					return
				}
				writeJSONError(w, http.StatusInternalServerError, "internal error")
				return
			}

			// Parse the request to determine required permissions
			req, err := ParseRequest(r)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, err.Error())
				return
			}

			// Check permissions
			if err := v.CheckPermission(keyInfo, req); err != nil {
				writeJSONError(w, http.StatusForbidden, "permission denied")
				return
			}

			// Attach KeyInfo to context and continue
			ctx := context.WithValue(r.Context(), keyInfoContextKey, keyInfo)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetKeyInfo retrieves KeyInfo from request context
func GetKeyInfo(ctx context.Context) *KeyInfo {
	if v := ctx.Value(keyInfoContextKey); v != nil {
		if info, ok := v.(*KeyInfo); ok {
			return info
		}
	}
	return nil
}

// extractBearerToken gets token from "Authorization: Bearer <token>" header
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

// writeJSONError writes a JSON error response
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(map[string]string{"error": message})
	if err != nil {
		// Encoding errors are not critical for error responses
		_ = err
	}
}
