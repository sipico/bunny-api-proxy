// Package admin provides administration endpoints and functionality for the proxy.
package admin

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"path/filepath"

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
	templates    *template.Template
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
}

// NewHandler creates an admin handler
func NewHandler(storage Storage, sessionStore *SessionStore, logLevel *slog.LevelVar, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if logLevel == nil {
		logLevel = new(slog.LevelVar)
	}

	// Load templates from filesystem
	// Try multiple locations to support both binary and test execution
	var tmpl *template.Template
	var err error

	// Try relative to current directory first (binary execution)
	tmpl, err = template.ParseGlob("web/templates/*.html")
	if err != nil {
		// Try relative to project root (when running from within project)
		tmpl, err = template.ParseGlob("../../web/templates/*.html")
		if err != nil {
			// Try absolute path based on common deployment
			homeDir, err2 := os.UserHomeDir()
			if err2 == nil && homeDir != "" {
				absPath := filepath.Join(homeDir, "bunny-api-proxy", "web", "templates", "*.html")
				tmpl, err = template.ParseGlob(absPath)
			}
		}
	}

	if err != nil {
		logger.Warn("failed to load templates", "error", fmt.Sprintf("%v (tried relative and absolute paths)", err))
	}

	return &Handler{
		storage:      storage,
		sessionStore: sessionStore,
		logLevel:     logLevel,
		logger:       logger,
		templates:    tmpl,
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
