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

		// Whoami endpoint - available to any authenticated token
		r.Get("/whoami", h.HandleWhoami)

		// Log level management
		r.Post("/loglevel", h.HandleSetLogLevel)

		// Unified token management (Issue 147)
		r.Get("/tokens", h.HandleListUnifiedTokens)
		r.Post("/tokens", h.HandleCreateUnifiedToken)
		r.Get("/tokens/{id}", h.HandleGetUnifiedToken)
		r.Delete("/tokens/{id}", h.HandleDeleteUnifiedToken)
		r.Post("/tokens/{id}/permissions", h.HandleAddTokenPermission)
		r.Delete("/tokens/{id}/permissions/{pid}", h.HandleDeleteTokenPermission)

		// Legacy endpoints (for backward compatibility)
		r.Put("/master-key", h.HandleSetMasterKeyAPI)
		r.Post("/keys", h.HandleCreateKeyAPI)
	})

	// Protected web UI (session auth)
	r.Group(func(r chi.Router) {
		r.Use(h.SessionMiddleware)
		r.Get("/", h.HandleDashboard)
		r.Get("/master-key", h.HandleMasterKeyForm)
		r.Post("/master-key", h.HandleSetMasterKey)

		// Admin token management (Issue 92)
		r.Get("/tokens", h.HandleListAdminTokensPage)
		r.Get("/tokens/new", h.HandleNewTokenForm)
		r.Post("/tokens", h.HandleCreateAdminToken)
		r.Post("/tokens/{id}/delete", h.HandleDeleteAdminToken)

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
