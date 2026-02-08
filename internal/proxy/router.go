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
	r.Use(middleware.MaxBodySize(1 << 20))     // 1MB limit
	r.Use(authMiddleware)                      // Auth after logging

	// Wire handler methods to routes
	r.Get("/dnszone", handler.HandleListZones)
	r.Post("/dnszone", handler.HandleCreateZone)
	r.With(requireAdmin).Post("/dnszone/checkavailability", handler.HandleCheckAvailability)
	r.With(requireAdmin).Post("/dnszone/{zoneID}/import", handler.HandleImportRecords)
	r.With(requireAdmin).Get("/dnszone/{zoneID}/export", handler.HandleExportRecords)
	r.With(requireAdmin).Post("/dnszone/{zoneID}/dnssec", handler.HandleEnableDNSSEC)
	r.With(requireAdmin).Delete("/dnszone/{zoneID}/dnssec", handler.HandleDisableDNSSEC)
	r.With(requireAdmin).Post("/dnszone/{zoneID}/certificate/issue", handler.HandleIssueCertificate)
	r.With(requireAdmin).Get("/dnszone/{zoneID}/statistics", handler.HandleGetStatistics)
	r.With(requireAdmin).Post("/dnszone/records/scan", handler.HandleTriggerScan)
	r.With(requireAdmin).Post("/dnszone/{zoneID}", handler.HandleUpdateZone)
	r.Get("/dnszone/{zoneID}", handler.HandleGetZone)
	r.Delete("/dnszone/{zoneID}", handler.HandleDeleteZone)
	r.With(requireAdmin).Get("/dnszone/{zoneID}/records/scan", handler.HandleGetScanResult)
	r.Get("/dnszone/{zoneID}/records", handler.HandleListRecords)
	r.Post("/dnszone/{zoneID}/records", handler.HandleAddRecord)
	r.Post("/dnszone/{zoneID}/records/{recordID}", handler.HandleUpdateRecord)
	r.Delete("/dnszone/{zoneID}/records/{recordID}", handler.HandleDeleteRecord)

	return r
}
