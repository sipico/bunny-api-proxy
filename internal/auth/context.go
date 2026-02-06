package auth

import (
	"context"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// contextKey is a private type for context keys to prevent collisions.
type ctxKey int

const (
	// Context keys for authentication data.
	tokenKey       ctxKey = iota // stores *storage.Token
	permissionsKey               // stores []*storage.Permission
	masterKeyKey                 // stores bool (is master key auth)
	adminKey                     // stores bool (is admin)
)

// TokenFromContext retrieves the authenticated token from context.
// Returns nil if no token is set (e.g., master key authentication).
func TokenFromContext(ctx context.Context) *storage.Token {
	if v := ctx.Value(tokenKey); v != nil {
		if token, ok := v.(*storage.Token); ok {
			return token
		}
	}
	return nil
}

// PermissionsFromContext retrieves the token's permissions from context.
// Returns nil if no permissions are set.
func PermissionsFromContext(ctx context.Context) []*storage.Permission {
	if v := ctx.Value(permissionsKey); v != nil {
		if perms, ok := v.([]*storage.Permission); ok {
			return perms
		}
	}
	return nil
}

// IsMasterKeyFromContext returns true if the request was authenticated with the master key.
func IsMasterKeyFromContext(ctx context.Context) bool {
	if v := ctx.Value(masterKeyKey); v != nil {
		if isMaster, ok := v.(bool); ok {
			return isMaster
		}
	}
	return false
}

// IsAdminFromContext returns true if the request has admin privileges.
// This is true for master key authentication (during bootstrap) or admin tokens.
func IsAdminFromContext(ctx context.Context) bool {
	if v := ctx.Value(adminKey); v != nil {
		if isAdmin, ok := v.(bool); ok {
			return isAdmin
		}
	}
	return false
}

// WithToken adds a token to the context.
func WithToken(ctx context.Context, token *storage.Token) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}

// WithPermissions adds permissions to the context.
func WithPermissions(ctx context.Context, perms []*storage.Permission) context.Context {
	return context.WithValue(ctx, permissionsKey, perms)
}

// WithMasterKey marks the context as authenticated with master key.
func WithMasterKey(ctx context.Context, isMaster bool) context.Context {
	return context.WithValue(ctx, masterKeyKey, isMaster)
}

// WithAdmin marks the context as having admin privileges.
func WithAdmin(ctx context.Context, isAdmin bool) context.Context {
	return context.WithValue(ctx, adminKey, isAdmin)
}
