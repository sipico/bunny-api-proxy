// Package admin provides administration endpoints and functionality for the proxy.
package admin

import (
	"context"
	"errors"
	"log/slog"
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
	storage Storage
	logger  *slog.Logger
	// Additional fields added by later issues:
	// - sessionStore (Issue 2)
	// - logLevel (Issue 4)
}

// Storage interface for admin operations
// Extended by later issues with additional methods
type Storage interface {
	// Health check
	Close() error
}

// NewHandler creates an admin handler
func NewHandler(storage Storage, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		storage: storage,
		logger:  logger,
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
