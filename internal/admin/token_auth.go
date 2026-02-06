package admin

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// TokenInfo contains validated admin token information
type TokenInfo struct {
	ID   int64
	Name string
}

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

		// Validate token against legacy admin tokens
		adminToken, err := h.storage.ValidateAdminToken(ctx, token)
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

		// Add token info to context (legacy format)
		tokenInfo := &TokenInfo{
			ID:   adminToken.ID,
			Name: adminToken.Name,
		}
		ctx = WithTokenInfo(ctx, tokenInfo)
		ctx = auth.WithAdmin(ctx, true) // Legacy admin tokens are always admin

		h.logger.Debug("admin API request via legacy token", "token_name", tokenInfo.Name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// validateUnifiedToken validates a token against the unified token system.
// Returns the token if valid, or nil if not found.
func (h *Handler) validateUnifiedToken(ctx context.Context, token string) (*storage.Token, error) {
	// Hash the provided token and look it up
	// The storage layer handles the hashing
	tokens, err := h.storage.ListTokens(ctx)
	if err != nil {
		return nil, err
	}

	// Hash the provided token for comparison
	keyHash := auth.HashToken(token)

	for _, t := range tokens {
		if t.KeyHash == keyHash {
			return t, nil
		}
	}

	return nil, storage.ErrNotFound
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
