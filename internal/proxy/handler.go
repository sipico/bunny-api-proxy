// Package proxy implements the API proxy for bunny.net requests.
package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/bunny"
)

// BunnyClient defines the bunny.net API operations needed by the proxy.
// This interface enables testing with mock implementations.
type BunnyClient interface {
	// ListZones retrieves all DNS zones with optional filtering.
	ListZones(ctx context.Context, opts *bunny.ListZonesOptions) (*bunny.ListZonesResponse, error)

	// CreateZone creates a new DNS zone.
	CreateZone(ctx context.Context, domain string) (*bunny.Zone, error)

	// GetZone retrieves a single zone by ID, including all records.
	GetZone(ctx context.Context, id int64) (*bunny.Zone, error)

	// DeleteZone deletes a DNS zone by ID.
	DeleteZone(ctx context.Context, id int64) error

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
	//nolint:errcheck
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// handleBunnyError maps bunny.net client errors to appropriate HTTP responses.
// It logs errors to help with debugging upstream issues.
func handleBunnyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, bunny.ErrNotFound):
		writeError(w, http.StatusNotFound, "resource not found")
	case errors.Is(err, bunny.ErrUnauthorized):
		// Master key issue - proxy's bunny.net credentials are invalid
		slog.Default().Error("upstream authentication failed", "error", err)
		writeError(w, http.StatusBadGateway, "upstream authentication failed")
	default:
		// Generic errors (network, parsing, etc.) - log for debugging
		slog.Default().Error("bunny.net API error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

// filterRecordsByPermission filters zone records based on the scoped key's permitted record types.
// Returns the original records if no type restriction applies.
func filterRecordsByPermission(records []bunny.Record, keyInfo *auth.KeyInfo, zoneID int64) []bunny.Record {
	if keyInfo == nil {
		return records
	}
	permittedTypes := auth.GetPermittedRecordTypes(keyInfo, zoneID)
	if permittedTypes == nil {
		return records
	}
	typeSet := make(map[string]bool, len(permittedTypes))
	for _, t := range permittedTypes {
		typeSet[t] = true
	}
	filtered := make([]bunny.Record, 0, len(records))
	for _, record := range records {
		if typeSet[auth.MapRecordTypeToString(record.Type)] {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

// HandleListZones lists all DNS zones with optional filtering.
func (h *Handler) HandleListZones(w http.ResponseWriter, r *http.Request) {
	opts := &bunny.ListZonesOptions{}

	// Parse optional query parameters
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid page parameter")
			return
		}
		opts.Page = page
	}

	if perPageStr := r.URL.Query().Get("perPage"); perPageStr != "" {
		perPage, err := strconv.Atoi(perPageStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid perPage parameter")
			return
		}
		opts.PerPage = perPage
	}

	if search := r.URL.Query().Get("search"); search != "" {
		opts.Search = search
	}

	// Call client to list zones
	result, err := h.client.ListZones(r.Context(), opts)
	if err != nil {
		handleBunnyError(w, err)
		return
	}

	// Filter zones by permission if scoped key
	keyInfo := auth.GetKeyInfo(r.Context())
	if keyInfo != nil && !auth.HasAllZonesPermission(keyInfo) {
		permittedIDs := auth.GetPermittedZoneIDs(keyInfo)
		idSet := make(map[int64]bool)
		for _, id := range permittedIDs {
			idSet[id] = true
		}

		filtered := make([]bunny.Zone, 0)
		for _, zone := range result.Items {
			if idSet[zone.ID] {
				filtered = append(filtered, zone)
			}
		}
		result.Items = filtered
		result.TotalItems = len(filtered)
		result.HasMoreItems = false
	}

	// Log the request
	h.logger.Info("list zones", "page", opts.Page, "perPage", opts.PerPage, "search", opts.Search)

	// Return successful response
	writeJSON(w, http.StatusOK, result)
}

// HandleCreateZone creates a new DNS zone.
// POST /dnszone
// Body: {"Domain": "example.com"}
func (h *Handler) HandleCreateZone(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req struct {
		Domain string `json:"Domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Domain == "" {
		writeError(w, http.StatusBadRequest, "missing domain")
		return
	}

	// Create zone via bunny client
	zone, err := h.client.CreateZone(r.Context(), req.Domain)
	if err != nil {
		handleBunnyError(w, err)
		return
	}

	// Log the request
	h.logger.Info("create zone", "domain", req.Domain, "zoneID", zone.ID)

	// Return successful response
	writeJSON(w, http.StatusCreated, zone)
}

// HandleGetZone retrieves a single DNS zone by ID.
func (h *Handler) HandleGetZone(w http.ResponseWriter, r *http.Request) {
	zoneIDStr := chi.URLParam(r, "zoneID")
	if zoneIDStr == "" {
		writeError(w, http.StatusBadRequest, "missing zone ID")
		return
	}

	zoneID, err := strconv.ParseInt(zoneIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid zone ID")
		return
	}

	// Call client to get zone
	zone, err := h.client.GetZone(r.Context(), zoneID)
	if err != nil {
		handleBunnyError(w, err)
		return
	}

	// Filter records by record type if scoped key
	keyInfo := auth.GetKeyInfo(r.Context())
	zone.Records = filterRecordsByPermission(zone.Records, keyInfo, zoneID)

	// Log the request
	h.logger.Info("get zone", "zone_id", zoneID)

	// Return successful response
	writeJSON(w, http.StatusOK, zone)
}

// HandleDeleteZone deletes a DNS zone by ID.
// DELETE /dnszone/{zoneID}
func (h *Handler) HandleDeleteZone(w http.ResponseWriter, r *http.Request) {
	zoneIDStr := chi.URLParam(r, "zoneID")
	if zoneIDStr == "" {
		writeError(w, http.StatusBadRequest, "missing zone ID")
		return
	}

	zoneID, err := strconv.ParseInt(zoneIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid zone ID")
		return
	}

	// Delete zone via bunny client
	if err := h.client.DeleteZone(r.Context(), zoneID); err != nil {
		handleBunnyError(w, err)
		return
	}

	// Log the request
	h.logger.Info("delete zone", "zone_id", zoneID)

	// Return successful response (204 No Content)
	w.WriteHeader(http.StatusNoContent)
}

// HandleListRecords lists all DNS records for a zone.
func (h *Handler) HandleListRecords(w http.ResponseWriter, r *http.Request) {
	zoneIDStr := chi.URLParam(r, "zoneID")
	if zoneIDStr == "" {
		writeError(w, http.StatusBadRequest, "missing zone ID")
		return
	}

	zoneID, err := strconv.ParseInt(zoneIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid zone ID")
		return
	}

	// Call client to get zone (which includes records)
	zone, err := h.client.GetZone(r.Context(), zoneID)
	if err != nil {
		handleBunnyError(w, err)
		return
	}

	// Filter records by record type if scoped key
	keyInfo := auth.GetKeyInfo(r.Context())
	zone.Records = filterRecordsByPermission(zone.Records, keyInfo, zoneID)

	// Log the request
	h.logger.Info("list records", "zone_id", zoneID)

	// Return only the records array
	writeJSON(w, http.StatusOK, zone.Records)
}

// HandleAddRecord creates a new DNS record in the specified zone.
func (h *Handler) HandleAddRecord(w http.ResponseWriter, r *http.Request) {
	zoneIDStr := chi.URLParam(r, "zoneID")
	if zoneIDStr == "" {
		writeError(w, http.StatusBadRequest, "missing zone ID")
		return
	}

	zoneID, err := strconv.ParseInt(zoneIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid zone ID")
		return
	}

	// Decode request body
	var req bunny.AddRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Call client to add record
	record, err := h.client.AddRecord(r.Context(), zoneID, &req)
	if err != nil {
		handleBunnyError(w, err)
		return
	}

	// Log the request
	h.logger.Info("add record", "zone_id", zoneID, "type", req.Type, "name", req.Name)

	// Return 201 Created with the record
	writeJSON(w, http.StatusCreated, record)
}

// HandleDeleteRecord removes a DNS record from the specified zone.
func (h *Handler) HandleDeleteRecord(w http.ResponseWriter, r *http.Request) {
	zoneIDStr := chi.URLParam(r, "zoneID")
	if zoneIDStr == "" {
		writeError(w, http.StatusBadRequest, "missing zone ID")
		return
	}

	zoneID, err := strconv.ParseInt(zoneIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid zone ID")
		return
	}

	recordIDStr := chi.URLParam(r, "recordID")
	if recordIDStr == "" {
		writeError(w, http.StatusBadRequest, "missing record ID")
		return
	}

	recordID, err := strconv.ParseInt(recordIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid record ID")
		return
	}

	// Call client to delete record
	err = h.client.DeleteRecord(r.Context(), zoneID, recordID)
	if err != nil {
		handleBunnyError(w, err)
		return
	}

	// Log the request
	h.logger.Info("delete record", "zone_id", zoneID, "record_id", recordID)

	// Return 204 No Content
	w.WriteHeader(http.StatusNoContent)
}
