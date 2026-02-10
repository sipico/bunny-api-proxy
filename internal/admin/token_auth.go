package admin

import (
	"context"
	"net/http"
	"strings"

	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// TokenAuthMiddleware validates AccessKey tokens for admin API
// It accepts:
// - AccessKey header: validated against stored admin tokens or master API key
func (h *Handler) TokenAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accessKey := r.Header.Get("AccessKey")
		if accessKey == "" {
			http.Error(w, "missing API key", http.StatusUnauthorized)
			return
		}

		token := strings.TrimSpace(accessKey)
		if token == "" {
			http.Error(w, "missing API key", http.StatusUnauthorized)
			return
		}

		ctx := r.Context()

		// First, check if this is the master API key using the bootstrap service
		if h.bootstrap != nil && h.bootstrap.IsMasterKey(token) {
			// Master key is allowed only during bootstrap (UNCONFIGURED state)
			canUse, err := h.bootstrap.CanUseMasterKey(ctx)
			if err != nil {
				h.logger.Error("failed to check bootstrap state", "error", err)
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}
			if !canUse {
				WriteError(w, http.StatusForbidden, ErrCodeMasterKeyLocked,
					"Master API key is locked. Use an admin token instead.")
				return
			}
			// Master key authenticated - set context flags
			ctx = auth.WithMasterKey(ctx, true)
			ctx = auth.WithAdmin(ctx, true)
			h.logger.Debug("admin API request via master key")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Check against unified tokens (Issue 147)
		unifiedToken, err := h.validateUnifiedToken(ctx, token)
		if err == nil && unifiedToken != nil {
			// Add token and admin status to context
			ctx = auth.WithToken(ctx, unifiedToken)
			ctx = auth.WithAdmin(ctx, unifiedToken.IsAdmin)
			h.logger.Debug("admin API request via unified token", "token_name", unifiedToken.Name, "is_admin", unifiedToken.IsAdmin)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// No valid token found
		h.logger.Warn("invalid admin token attempt", "remote_addr", r.RemoteAddr)
		http.Error(w, "Invalid token", http.StatusUnauthorized)
	})
}

// validateUnifiedToken validates a token against the unified token system.
// Returns the token if valid, or nil if not found.
func (h *Handler) validateUnifiedToken(ctx context.Context, token string) (*storage.Token, error) {
	keyHash := auth.HashToken(token)
	return h.storage.GetTokenByHash(ctx, keyHash)
}
