package admin

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// TokenInfo contains validated admin token information
type TokenInfo struct {
	ID   int64
	Name string
}

// TokenAuthMiddleware validates Bearer tokens for admin API
func (h *Handler) TokenAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract Bearer token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		// Check "Bearer " prefix
		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, prefix)
		if token == "" {
			http.Error(w, "Missing token", http.StatusUnauthorized)
			return
		}

		// Validate token against database
		adminToken, err := h.storage.ValidateAdminToken(r.Context(), token)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				h.logger.Warn("invalid admin token attempt", "remote_addr", r.RemoteAddr)
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}
			h.logger.Error("token validation error", "error", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		// Add token info to context
		tokenInfo := &TokenInfo{
			ID:   adminToken.ID,
			Name: adminToken.Name,
		}
		ctx := WithTokenInfo(r.Context(), tokenInfo)

		h.logger.Debug("admin API request", "token_name", tokenInfo.Name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetTokenInfoFromContext retrieves token info from context
// Returns nil if not found
func GetTokenInfoFromContext(ctx context.Context) *TokenInfo {
	info, ok := GetTokenInfo(ctx)
	if !ok {
		return nil
	}

	tokenInfo, ok := info.(*TokenInfo)
	if !ok {
		return nil
	}

	return tokenInfo
}
