// Package proxy implements the API proxy for bunny.net requests.
package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/sipico/bunny-api-proxy/internal/bunny"
)

// BunnyClient defines the bunny.net API operations needed by the proxy.
// This interface enables testing with mock implementations.
type BunnyClient interface {
	// ListZones retrieves all DNS zones with optional filtering.
	ListZones(ctx context.Context, opts *bunny.ListZonesOptions) (*bunny.ListZonesResponse, error)

	// GetZone retrieves a single zone by ID, including all records.
	GetZone(ctx context.Context, id int64) (*bunny.Zone, error)

	// AddRecord creates a new DNS record in the specified zone.
	AddRecord(ctx context.Context, zoneID int64, req *bunny.AddRecordRequest) (*bunny.Record, error)

	// DeleteRecord removes a DNS record from the specified zone.
	DeleteRecord(ctx context.Context, zoneID, recordID int64) error
}

// Handler handles proxy requests to bunny.net API.
type Handler struct {
	client BunnyClient
	logger *slog.Logger
}

// NewHandler creates a new proxy handler.
// If logger is nil, slog.Default() will be used.
func NewHandler(client BunnyClient, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		client: client,
		logger: logger,
	}
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Log encoding errors but don't fail the response
		slog.Default().Error("failed to encode JSON response", "error", err)
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(map[string]string{"error": message})
	if err != nil {
		// Encoding errors on error responses are not critical
		_ = err
	}
}

// handleBunnyError maps bunny.net client errors to appropriate HTTP responses.
func handleBunnyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, bunny.ErrNotFound):
		writeError(w, http.StatusNotFound, "resource not found")
	case errors.Is(err, bunny.ErrUnauthorized):
		// Master key issue - proxy's bunny.net credentials are invalid
		writeError(w, http.StatusBadGateway, "upstream authentication failed")
	default:
		// Generic errors (network, parsing, etc.)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}
