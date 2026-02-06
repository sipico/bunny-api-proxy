package admin

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	internalMiddleware "github.com/sipico/bunny-api-proxy/internal/middleware"
)

// NewRouter creates the admin router with API routes only
func (h *Handler) NewRouter() chi.Router {
	r := chi.NewRouter()

	// Allowlist for admin API - protects token fields
	adminAllowlist := []string{
		"id", "name", "created_at", "zone_id",
		"allowed_actions", "record_types", "level", "is_admin",
	}

	// Middleware (order matters)
	r.Use(internalMiddleware.RequestID)                             // Request ID first
	r.Use(internalMiddleware.HTTPLogging(h.logger, adminAllowlist)) // Logging with allowlist
	r.Use(middleware.Recoverer)                                     // Panic recovery
	r.Use(internalMiddleware.MaxBodySize(1 << 20))                  // 1MB limit

	// Public endpoints (no auth)
	r.Get("/health", h.HandleHealth)
	r.Get("/ready", h.HandleReady)

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
	})

	return r
}
