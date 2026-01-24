// Package proxy implements the API proxy for bunny.net requests.
package proxy

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// NewRouter creates a Chi router with all proxy endpoints.
// The authMiddleware parameter should be auth.Middleware(validator).
func NewRouter(handler *Handler, authMiddleware func(http.Handler) http.Handler) http.Handler {
	r := chi.NewRouter()

	// Apply auth middleware to all routes
	r.Use(authMiddleware)

	// Wire handler methods to routes
	r.Get("/dnszone", handler.HandleListZones)
	r.Get("/dnszone/{zoneID}", handler.HandleGetZone)
	r.Get("/dnszone/{zoneID}/records", handler.HandleListRecords)
	r.Post("/dnszone/{zoneID}/records", handler.HandleAddRecord)
	r.Delete("/dnszone/{zoneID}/records/{recordID}", handler.HandleDeleteRecord)

	return r
}
