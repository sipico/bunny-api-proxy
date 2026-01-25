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

// TokenAuthMiddleware validates Bearer tokens or Basic Auth for admin API
// It accepts either:
// - Bearer token: validated against stored admin tokens
// - Basic auth: username "admin" with the admin password (for bootstrapping)
func (h *Handler) TokenAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		// Check for Basic auth first (for bootstrapping)
		const basicPrefix = "Basic "
		if strings.HasPrefix(authHeader, basicPrefix) {
			username, password, ok := r.BasicAuth()
			if !ok {
				http.Error(w, "Invalid Basic auth format", http.StatusUnauthorized)
				return
			}

			// Validate against admin credentials
			if username == "admin" && h.sessionStore.ValidatePassword(password) {
				// Add a pseudo token info for basic auth
				tokenInfo := &TokenInfo{
					ID:   0, // No real token ID for basic auth
					Name: "basic-auth-admin",
				}
				ctx := WithTokenInfo(r.Context(), tokenInfo)
				h.logger.Debug("admin API request via basic auth")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			h.logger.Warn("invalid basic auth attempt", "remote_addr", r.RemoteAddr)
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		// Check "Bearer " prefix
		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, bearerPrefix)
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
