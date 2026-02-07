package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// Authenticator handles authentication for API requests.
// It supports both master key authentication (during bootstrap) and token authentication.
type Authenticator struct {
	tokens    storage.TokenStore
	bootstrap *BootstrapService
}

// NewAuthenticator creates a new authentication middleware.
func NewAuthenticator(tokens storage.TokenStore, bootstrap *BootstrapService) *Authenticator {
	return &Authenticator{
		tokens:    tokens,
		bootstrap: bootstrap,
	}
}

// Authenticate is middleware that validates the API key and sets authentication context.
// It checks in order:
// 1. Master key (only valid during UNCONFIGURED state)
// 2. Token from the tokens table (SHA256 hash lookup)
//
// On success, it sets:
// - Token in context (nil for master key)
// - Permissions in context (nil for master key or admin tokens)
// - IsMasterKey flag
// - IsAdmin flag
func (m *Authenticator) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract API key from AccessKey header
		apiKey := extractAccessKey(r)
		if apiKey == "" {
			writeJSONError(w, http.StatusUnauthorized, "missing API key")
			return
		}

		ctx := r.Context()

		// First, check if this is the master key (only during UNCONFIGURED state)
		isMasterKeyValid, err := m.bootstrap.ValidateMasterKey(ctx, apiKey)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal error")
			return
		}

		if isMasterKeyValid {
			// Master key authenticated - set context and continue
			ctx = WithMasterKey(ctx, true)
			ctx = WithAdmin(ctx, true)
			// No token or permissions for master key
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Try token authentication using SHA256 hash
		hash := sha256.Sum256([]byte(apiKey))
		keyHash := hex.EncodeToString(hash[:])

		token, err := m.tokens.GetTokenByHash(ctx, keyHash)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				writeJSONError(w, http.StatusUnauthorized, "invalid API key")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Token found - set context
		ctx = WithToken(ctx, token)
		ctx = WithMasterKey(ctx, false)
		ctx = WithAdmin(ctx, token.IsAdmin)

		// Load permissions for scoped tokens
		if !token.IsAdmin {
			perms, err := m.loadPermissions(ctx, token.ID)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "internal error")
				return
			}
			ctx = WithPermissions(ctx, perms)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loadPermissions loads permissions for a token.
// Uses the PermissionStore interface if available on the tokens store.
func (m *Authenticator) loadPermissions(ctx context.Context, tokenID int64) ([]*storage.Permission, error) {
	// Check if the token store also implements GetPermissionsForToken
	type permissionLoader interface {
		GetPermissionsForToken(ctx context.Context, tokenID int64) ([]*storage.Permission, error)
	}

	if loader, ok := m.tokens.(permissionLoader); ok {
		return loader.GetPermissionsForToken(ctx, tokenID)
	}

	// No permission loading available - return empty slice
	return []*storage.Permission{}, nil
}

// RequireAdmin is middleware that requires admin privileges.
// It must be used after Authenticate middleware.
// Returns 403 Forbidden if the request is not from an admin.
func (m *Authenticator) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsAdminFromContext(r.Context()) {
			writeJSONErrorWithCode(w, http.StatusForbidden, "admin_required", "This endpoint requires an admin token.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CheckPermissions is middleware that validates token permissions for proxy requests.
// It must be used after Authenticate middleware.
// Admin tokens and master key bypass permission checks.
// Scoped tokens are validated against their permissions.
func (m *Authenticator) CheckPermissions(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Admin and master key bypass permission checks
		if IsAdminFromContext(ctx) {
			next.ServeHTTP(w, r)
			return
		}

		// Get permissions from context (set by Authenticate middleware)
		perms := PermissionsFromContext(ctx)
		if perms == nil {
			perms = []*storage.Permission{} // Empty permissions for scoped tokens
		}

		// Parse the request to determine required permissions
		req, err := ParseRequest(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Check if this is an admin-only action
		if req.Action == ActionUpdateZone || req.Action == ActionCreateZone {
			writeJSONErrorWithCode(w, http.StatusForbidden, "admin_required", "This endpoint requires an admin token.")
			return
		}

		// Build KeyInfo for permission checking
		token := TokenFromContext(ctx)
		var keyInfo *KeyInfo
		if token != nil {
			keyInfo = &KeyInfo{
				KeyID:       token.ID,
				KeyName:     token.Name,
				Permissions: perms,
			}
		} else {
			// Shouldn't happen if Authenticate ran first, but handle gracefully
			keyInfo = &KeyInfo{
				Permissions: perms,
			}
		}

		// Use existing Validator logic for permission checking
		validator := &Validator{} // Empty validator just for CheckPermission method
		if err := validator.CheckPermission(keyInfo, req); err != nil {
			writeJSONError(w, http.StatusForbidden, "permission denied")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// --- Legacy support for existing Validator-based middleware ---
// The following functions maintain backward compatibility with existing code.

const (
	// keyInfoKey is the context key for storing KeyInfo (legacy).
	keyInfoKey ctxKey = 100
)

// KeyInfoContextKey is the public context key for storing KeyInfo (legacy).
const KeyInfoContextKey = keyInfoKey

// Middleware returns Chi-compatible middleware for API key validation (legacy).
// This middleware uses the old Validator-based authentication.
// For new code, use Authenticator.Authenticate instead.
func Middleware(v *Validator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract API key from AccessKey header
			apiKey := extractAccessKey(r)
			if apiKey == "" {
				writeJSONError(w, http.StatusUnauthorized, "missing API key")
				return
			}

			// Validate the key
			keyInfo, err := v.ValidateKey(r.Context(), apiKey)
			if err != nil {
				if errors.Is(err, ErrInvalidKey) {
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
			ctx := context.WithValue(r.Context(), keyInfoKey, keyInfo)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetKeyInfo retrieves KeyInfo from request context (legacy).
func GetKeyInfo(ctx context.Context) *KeyInfo {
	if v := ctx.Value(keyInfoKey); v != nil {
		if info, ok := v.(*KeyInfo); ok {
			return info
		}
	}
	return nil
}

// --- Helper functions ---

// extractAccessKey gets API key from "AccessKey" header.
func extractAccessKey(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("AccessKey"))
}

// writeJSONError writes a JSON error response with just an error message.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(map[string]string{"error": message})
	if err != nil {
		// Encoding errors are not critical for error responses
		_ = err
	}
}

// writeJSONErrorWithCode writes a JSON error response with code and message.
func writeJSONErrorWithCode(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]string{
		"error":   code,
		"message": message,
	}
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		// Encoding errors are not critical for error responses
		_ = err
	}
}
