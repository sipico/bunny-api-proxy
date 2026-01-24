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

	// Routes added by later issues:
	// - POST /login (Issue 2)
	// - POST /logout (Issue 2)
	// - /api/* (Issue 4, with token auth middleware)
	// - /* (Issue 5+, with session auth middleware)

	return r
}
