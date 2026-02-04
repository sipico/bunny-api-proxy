// Package proxy implements the API proxy for bunny.net requests.
package proxy

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sipico/bunny-api-proxy/internal/middleware"
)

// NewRouter creates a Chi router with all proxy endpoints.
// The authMiddleware parameter should be auth.Middleware(validator).
// The logger parameter is used for debug logging of HTTP requests/responses.
func NewRouter(handler *Handler, authMiddleware func(http.Handler) http.Handler, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	// Apply middlewares in order
	r.Use(middleware.RequestID)                // Add request ID first
	r.Use(middleware.HTTPLogging(logger, nil)) // Log with no allowlist (DNS API has no secrets)
	r.Use(authMiddleware)                      // Auth after logging

	// Wire handler methods to routes
	r.Get("/dnszone", handler.HandleListZones)
	r.Post("/dnszone", handler.HandleCreateZone)
	r.Get("/dnszone/{zoneID}", handler.HandleGetZone)
	r.Delete("/dnszone/{zoneID}", handler.HandleDeleteZone)
	r.Get("/dnszone/{zoneID}/records", handler.HandleListRecords)
	r.Post("/dnszone/{zoneID}/records", handler.HandleAddRecord)
	r.Delete("/dnszone/{zoneID}/records/{recordID}", handler.HandleDeleteRecord)

	return r
}
