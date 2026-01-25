package admin

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates the admin router with all routes
func (h *Handler) NewRouter() chi.Router {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Public endpoints (no auth)
	r.Get("/health", h.HandleHealth)
	r.Get("/ready", h.HandleReady)
	r.Post("/login", h.HandleLogin)
	r.Post("/logout", h.HandleLogout)

	// Admin API (token auth)
	r.Route("/api", func(r chi.Router) {
		r.Use(h.TokenAuthMiddleware)
		r.Post("/loglevel", h.HandleSetLogLevel)
		r.Get("/tokens", h.HandleListTokens)
		r.Post("/tokens", h.HandleCreateToken)
		r.Delete("/tokens/{id}", h.HandleDeleteToken)
	})

	// Protected web UI (session auth)
	r.Group(func(r chi.Router) {
		r.Use(h.SessionMiddleware)
		r.Get("/", h.HandleDashboard)
		r.Get("/master-key", h.HandleMasterKeyForm)
		r.Post("/master-key", h.HandleSetMasterKey)

		// Key and permission management (Issue 91)
		r.Get("/keys", h.HandleListKeys)
		r.Get("/keys/new", h.HandleNewKeyForm)
		r.Post("/keys", h.HandleCreateKey)
		r.Get("/keys/{id}", h.HandleKeyDetail)
		r.Post("/keys/{id}/delete", h.HandleDeleteKey)
		r.Get("/keys/{id}/permissions/new", h.HandleAddPermissionForm)
		r.Post("/keys/{id}/permissions", h.HandleAddPermission)
		r.Post("/keys/{id}/permissions/{pid}/delete", h.HandleDeletePermission)
	})

	return r
}
