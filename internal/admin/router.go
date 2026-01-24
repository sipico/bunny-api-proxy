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
		// Handlers added by Issue 4:
		// r.Post("/loglevel", h.HandleSetLogLevel)
		// r.Get("/tokens", h.HandleListTokens)
		// r.Post("/tokens", h.HandleCreateToken)
		// r.Delete("/tokens/{id}", h.HandleDeleteToken)
	})

	// Protected web UI (session auth, added by Issue 5+)
	// r.Group(func(r chi.Router) {
	//     r.Use(h.SessionMiddleware)
	//     // Web UI routes here
	// })

	return r
}
