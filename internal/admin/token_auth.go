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

		// SECURITY: Every request performs the same work (master key check,
		// bootstrap state lookup and token lookup), and a locked master key
		// gets exactly the same response as an unknown key. Otherwise the
		// master key could be identified by response time or by a distinct
		// error (CWE-208, issue #391).
		isMasterKey := false
		canUseMasterKey := false
		if h.bootstrap != nil {
			isMasterKey = h.bootstrap.IsMasterKey(token)
			var err error
			canUseMasterKey, err = h.bootstrap.CanUseMasterKey(ctx)
			if err != nil {
				h.logger.Error("failed to check bootstrap state", "error", err)
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}
		}

		// Check against unified tokens (Issue 147)
		unifiedToken, err := h.validateUnifiedToken(ctx, token)

		// Master key is allowed only during bootstrap (UNCONFIGURED state)
		if isMasterKey && canUseMasterKey {
			ctx = auth.WithMasterKey(ctx, true)
			ctx = auth.WithAdmin(ctx, true)
			h.logger.Debug("admin API request via master key")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		if isMasterKey {
			// Locked master key: the hint goes to the server log only. The
			// client gets the same 401 as any invalid token below.
			h.logger.Warn("master API key used after bootstrap; it is locked, use an admin token instead",
				"remote_addr", r.RemoteAddr)
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

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
