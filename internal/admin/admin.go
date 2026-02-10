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
type Storage interface {
	// Health check
	Ping(ctx context.Context) error
	Close() error

	// Unified token operations
	CreateToken(ctx context.Context, name string, isAdmin bool, keyHash string) (*storage.Token, error)
	GetTokenByID(ctx context.Context, id int64) (*storage.Token, error)
	GetTokenByHash(ctx context.Context, keyHash string) (*storage.Token, error)
	ListTokens(ctx context.Context) ([]*storage.Token, error)
	DeleteToken(ctx context.Context, id int64) error
	CountAdminTokens(ctx context.Context) (int, error)

	// Unified permission operations
	AddPermissionForToken(ctx context.Context, tokenID int64, perm *storage.Permission) (*storage.Permission, error)
	RemovePermission(ctx context.Context, permID int64) error
	RemovePermissionForToken(ctx context.Context, tokenID, permID int64) error
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
