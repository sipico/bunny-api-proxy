package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

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

		// Optional expiry (issue #400): an expired token is rejected
		if token.IsExpired(time.Now()) {
			writeJSONError(w, http.StatusUnauthorized, "token expired")
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

		if req.Action == ActionUpdateZone || req.Action == ActionCreateZone || req.Action == ActionDeleteZone || req.Action == ActionCheckAvailability || req.Action == ActionImportRecords || req.Action == ActionExportRecords || req.Action == ActionEnableDNSSEC || req.Action == ActionDisableDNSSEC || req.Action == ActionIssueCertificate || req.Action == ActionGetStatistics || req.Action == ActionTriggerDNSScan || req.Action == ActionGetDNSScanResult {
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

		// Check permissions
		if err := CheckPermission(keyInfo, req); err != nil {
			writeJSONError(w, http.StatusForbidden, "permission denied")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GetKeyInfo returns the scope of a scoped token: its permissions, which
// handlers use to filter zones and records.
//
// It returns nil for admin tokens and when no token is in the context; nil
// means "not restricted". An admin token has no permissions of its own, so
// a KeyInfo for it would have an empty scope and filter everything away
// (admins saw no zones in GET /dnszone; caught by the authorization matrix).
func GetKeyInfo(ctx context.Context) *KeyInfo {
	token := TokenFromContext(ctx)
	if token == nil || token.IsAdmin || IsAdminFromContext(ctx) {
		return nil
	}

	perms := PermissionsFromContext(ctx)
	if perms == nil {
		perms = []*storage.Permission{} // a scoped token without permissions sees nothing
	}

	return &KeyInfo{
		KeyID:       token.ID,
		KeyName:     token.Name,
		Permissions: perms,
	}
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
