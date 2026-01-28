// Package admin provides administration endpoints and functionality for the proxy.
package admin

import (
	"context"
	"errors"
	"log/slog"

	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// Common errors
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrBadRequest   = errors.New("bad request")
)

// Handler provides admin endpoints
type Handler struct {
	storage   Storage
	logger    *slog.Logger
	logLevel  *slog.LevelVar
	bootstrap *auth.BootstrapService
}

// Storage interface for admin operations
// Extended by later issues with additional methods
type Storage interface {
	// Config operations (Issue 5)
	GetMasterAPIKey(ctx context.Context) (string, error)
	SetMasterAPIKey(ctx context.Context, key string) error

	// Health check
	Close() error

	// AdminToken operations (Issue 3)
	ValidateAdminToken(ctx context.Context, token string) (*storage.AdminToken, error)

	// AdminToken CRUD (Issue 4)
	CreateAdminToken(ctx context.Context, name, token string) (int64, error)
	ListAdminTokens(ctx context.Context) ([]*storage.AdminToken, error)
	DeleteAdminToken(ctx context.Context, id int64) error

	// Scoped key operations (Issue 91)
	ListScopedKeys(ctx context.Context) ([]*storage.ScopedKey, error)
	GetScopedKey(ctx context.Context, id int64) (*storage.ScopedKey, error)
	CreateScopedKey(ctx context.Context, name, apiKey string) (int64, error)
	DeleteScopedKey(ctx context.Context, id int64) error

	// Permission operations (Issue 91)
	GetPermissions(ctx context.Context, keyID int64) ([]*storage.Permission, error)
	AddPermission(ctx context.Context, scopedKeyID int64, perm *storage.Permission) (int64, error)
	DeletePermission(ctx context.Context, id int64) error

	// Unified token operations (Issue 147)
	CreateToken(ctx context.Context, name string, isAdmin bool, keyHash string) (*storage.Token, error)
	GetTokenByID(ctx context.Context, id int64) (*storage.Token, error)
	ListTokens(ctx context.Context) ([]*storage.Token, error)
	DeleteToken(ctx context.Context, id int64) error
	CountAdminTokens(ctx context.Context) (int, error)
	AddPermissionForToken(ctx context.Context, tokenID int64, perm *storage.Permission) (*storage.Permission, error)
	RemovePermission(ctx context.Context, permID int64) error
	GetPermissionsForToken(ctx context.Context, tokenID int64) ([]*storage.Permission, error)
}

// NewHandler creates an admin handler
func NewHandler(storage Storage, logLevel *slog.LevelVar, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if logLevel == nil {
		logLevel = new(slog.LevelVar)
	}

	return &Handler{
		storage:  storage,
		logLevel: logLevel,
		logger:   logger,
	}
}

// SetBootstrapService sets the bootstrap service for handling token creation during bootstrap.
// This must be called before using the unified token API endpoints.
func (h *Handler) SetBootstrapService(bs *auth.BootstrapService) {
	h.bootstrap = bs
}

// Context helpers (used by later issues)

type contextKey string

const (
	tokenInfoKey contextKey = "tokenInfo"
)

// WithTokenInfo attaches token info to context
func WithTokenInfo(ctx context.Context, info any) context.Context {
	return context.WithValue(ctx, tokenInfoKey, info)
}

// GetTokenInfo retrieves token info from context
func GetTokenInfo(ctx context.Context) (any, bool) {
	info := ctx.Value(tokenInfoKey)
	return info, info != nil
}
