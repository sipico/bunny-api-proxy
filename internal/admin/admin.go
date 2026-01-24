// Package admin provides administration endpoints and functionality for the proxy.
package admin

import (
	"context"
	"errors"
	"log/slog"

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
	storage      Storage
	sessionStore *SessionStore
	logger       *slog.Logger
	logLevel     *slog.LevelVar
}

// Storage interface for admin operations
// Extended by later issues with additional methods
type Storage interface {
	// Health check
	Close() error

	// AdminToken operations (Issue 3)
	ValidateAdminToken(ctx context.Context, token string) (*storage.AdminToken, error)

	// AdminToken CRUD (Issue 4)
	CreateAdminToken(ctx context.Context, name, token string) (int64, error)
	ListAdminTokens(ctx context.Context) ([]*storage.AdminToken, error)
	DeleteAdminToken(ctx context.Context, id int64) error
}

// NewHandler creates an admin handler
func NewHandler(storage Storage, sessionStore *SessionStore, logLevel *slog.LevelVar, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if logLevel == nil {
		logLevel = new(slog.LevelVar)
	}
	return &Handler{
		storage:      storage,
		sessionStore: sessionStore,
		logLevel:     logLevel,
		logger:       logger,
	}
}

// Context helpers (used by later issues)

type contextKey string

const (
	sessionIDKey contextKey = "sessionID"
	tokenInfoKey contextKey = "tokenInfo"
)

// WithSessionID attaches session ID to context
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

// GetSessionID retrieves session ID from context
func GetSessionID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(sessionIDKey).(string)
	return id, ok
}

// WithTokenInfo attaches token info to context
func WithTokenInfo(ctx context.Context, info any) context.Context {
	return context.WithValue(ctx, tokenInfoKey, info)
}

// GetTokenInfo retrieves token info from context
func GetTokenInfo(ctx context.Context) (any, bool) {
	info := ctx.Value(tokenInfoKey)
	return info, info != nil
}
