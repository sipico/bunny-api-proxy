// Package proxy implements the API proxy for bunny.net requests.
package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
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

	// ListRecords retrieves one page of a zone's records (GET /dnszone/{id}/records).
	ListRecords(ctx context.Context, zoneID int64, opts *bunny.ListRecordsOptions) (*bunny.ListRecordsResponse, error)

	// DeleteZone deletes a DNS zone by ID.
	DeleteZone(ctx context.Context, id int64) error
	// UpdateZone updates zone-level settings.
	UpdateZone(ctx context.Context, id int64, req *bunny.UpdateZoneRequest) (*bunny.Zone, error)
	// CheckZoneAvailability checks if a domain name is available to be added as a zone.
	CheckZoneAvailability(ctx context.Context, name string) (*bunny.CheckAvailabilityResponse, error)

	// ImportRecords imports DNS records from BIND zone file format.
	ImportRecords(ctx context.Context, zoneID int64, body io.Reader, contentType string) (*bunny.ImportRecordsResponse, error)

	// ExportRecords exports DNS records in BIND zone file format.
	ExportRecords(ctx context.Context, zoneID int64) (string, error)
	// EnableDNSSEC enables DNSSEC for a DNS zone.
	EnableDNSSEC(ctx context.Context, zoneID int64) (*bunny.DNSSECResponse, error)
	// DisableDNSSEC disables DNSSEC for a DNS zone.
	DisableDNSSEC(ctx context.Context, zoneID int64) (*bunny.DNSSECResponse, error)
	// IssueCertificate triggers issuance of a wildcard SSL certificate.
	IssueCertificate(ctx context.Context, zoneID int64, domain string) error
	// GetZoneStatistics retrieves DNS query statistics for a zone.
	GetZoneStatistics(ctx context.Context, zoneID int64, dateFrom, dateTo string) (*bunny.ZoneStatisticsResponse, error)
	// TriggerDNSScan triggers a background DNS record scan for a domain.
	TriggerDNSScan(ctx context.Context, domain string) (*bunny.DNSScanResult, error)

	// GetDNSScanResult retrieves the latest DNS record scan result.
	GetDNSScanResult(ctx context.Context, zoneID int64) (*bunny.DNSScanResult, error)
	// AddRecord creates a new DNS record in the specified zone.
	AddRecord(ctx context.Context, zoneID int64, req *bunny.AddRecordRequest) (*bunny.Record, error)

	// UpdateRecord updates an existing DNS record in the specified zone.
	UpdateRecord(ctx context.Context, zoneID, recordID int64, req *bunny.AddRecordRequest) (*bunny.Record, error)

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
	case isUpstreamTimeout(err):
		// The request may have reached bunny.net and been applied: measured
		// against the real API, PUTs whose client timed out had created
		// their record. Writes are not retried (see bunny.RetryTransport).
		slog.Default().Error("bunny.net did not answer in time", "error", err)
		writeError(w, http.StatusGatewayTimeout,
			"bunny.net did not answer in time. "+mayHaveBeenApplied)
	case isUpstreamServerError(err):
		// bunny.net's own 5xx text goes to the log only.
		slog.Default().Error("bunny.net server error", "error", err)
		writeError(w, http.StatusBadGateway,
			fmt.Sprintf("bunny.net returned an error (HTTP %d). %s", upstreamStatus(err), mayHaveBeenApplied))
	case isUpstreamUnreachable(err):
		slog.Default().Error("bunny.net unreachable", "error", err)
		writeError(w, http.StatusBadGateway,
			"could not reach bunny.net or the connection broke. "+mayHaveBeenApplied)
	default:
		// Check if it's a structured APIError with a specific status code
		var apiErr *bunny.APIError
		if errors.As(err, &apiErr) {
			// Forward the APIError status code (e.g., 400 for validation errors)
			writeError(w, apiErr.StatusCode, apiErr.Message)
			return
		}
		// Generic errors (parsing, etc.) - log for debugging
		slog.Default().Error("bunny.net API error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

// mayHaveBeenApplied is added to every upstream failure: bunny.net may have
// applied a change although it did not confirm it.
const mayHaveBeenApplied = "If this request changes data, it may have been applied: check before retrying."

// isUpstreamTimeout reports a timeout while waiting for bunny.net,
// including the proxy's own client timeout.
func isUpstreamTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// isUpstreamServerError reports a 5xx answer from bunny.net, with or without
// a structured body.
func isUpstreamServerError(err error) bool {
	return upstreamStatus(err) >= 500
}

// upstreamStatus returns bunny.net's status code for a 5xx error, or 0.
func upstreamStatus(err error) int {
	var serverErr *bunny.ServerError
	if errors.As(err, &serverErr) {
		return serverErr.StatusCode
	}
	var apiErr *bunny.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode >= 500 {
		return apiErr.StatusCode
	}
	return 0
}

// isUpstreamUnreachable reports a transport failure talking to bunny.net
// (connection refused, DNS, connection reset).
func isUpstreamUnreachable(err error) bool {
	var urlErr *url.Error
	return errors.As(err, &urlErr)
}

// authorizeExistingRecord checks, for a scoped token whose permission for
// the zone is limited to record types, that the record it wants to change or
// delete has one of those types. Without this a TXT-only token could delete
// or rewrite A or MX records (the request itself names no type, or only the
// new one). It looks the record up in the zone; a record that is not in the
// zone gets 404. Admins and permissions without a type limit need no lookup.
// On refusal it writes the response and returns false.
func (h *Handler) authorizeExistingRecord(w http.ResponseWriter, r *http.Request, zoneID, recordID int64) bool {
	permitted := auth.GetPermittedRecordTypes(auth.GetKeyInfo(r.Context()), zoneID)
	if permitted == nil {
		return true
	}
	zone, err := h.client.GetZone(r.Context(), zoneID)
	if err != nil {
		handleBunnyError(w, err)
		return false
	}
	for _, rec := range zone.Records {
		if rec.ID != recordID {
			continue
		}
		recType := auth.MapRecordTypeToString(rec.Type)
		for _, t := range permitted {
			if t == recType {
				return true
			}
		}
		h.logger.Warn("record type not permitted for this token",
			"zone_id", zoneID, "record_id", recordID, "record_type", recType)
		writeError(w, http.StatusForbidden, "permission denied")
		return false
	}
	writeError(w, http.StatusNotFound, "resource not found")
	return false
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

	// Zones in the list carry their records: show a scoped token only the
	// record types it may see, as GET /dnszone/{id} and the record list do.
	if keyInfo != nil {
		for i := range result.Items {
			result.Items[i].Records = filterRecordsByPermission(result.Items[i].Records, keyInfo, result.Items[i].ID)
		}
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

// HandleUpdateZone updates zone-level settings.
// POST /dnszone/{zoneID}
// Admin only — zone settings are sensitive operations.
func (h *Handler) HandleUpdateZone(w http.ResponseWriter, r *http.Request) {
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

	var req bunny.UpdateZoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	zone, err := h.client.UpdateZone(r.Context(), zoneID, &req)
	if err != nil {
		handleBunnyError(w, err)
		return
	}

	h.logger.Info("update zone", "zone_id", zoneID)
	writeJSON(w, http.StatusOK, zone)
}

// HandleCheckAvailability checks if a domain name is available to be added as a DNS zone.
// POST /dnszone/checkavailability
// Admin only — zone-less operation for zone creation workflows.
func (h *Handler) HandleCheckAvailability(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"Name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "missing domain name")
		return
	}

	result, err := h.client.CheckZoneAvailability(r.Context(), req.Name)
	if err != nil {
		handleBunnyError(w, err)
		return
	}

	h.logger.Info("check zone availability", "name", req.Name, "available", result.Available)

	writeJSON(w, http.StatusOK, result)
}

// HandleImportRecords imports DNS records from BIND zone file format.
// POST /dnszone/{zoneID}/import
// Admin only — bulk import operation.
func (h *Handler) HandleImportRecords(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.client.ImportRecords(r.Context(), zoneID, r.Body, r.Header.Get("Content-Type"))
	if err != nil {
		handleBunnyError(w, err)
		return
	}

	h.logger.Info("import records", "zone_id", zoneID, "created", result.Created, "failed", result.Failed, "skipped", result.Skipped)

	writeJSON(w, http.StatusOK, result)
}

// HandleExportRecords exports DNS records in BIND zone file format.
// GET /dnszone/{zoneID}/export
// Admin only — exports all records as raw text.
func (h *Handler) HandleExportRecords(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.client.ExportRecords(r.Context(), zoneID)
	if err != nil {
		handleBunnyError(w, err)
		return
	}

	h.logger.Info("export records", "zone_id", zoneID)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck
	w.Write([]byte(result))
}

// HandleEnableDNSSEC enables DNSSEC for a DNS zone.
// POST /dnszone/{zoneID}/dnssec
// Admin only — sensitive zone-level configuration.
func (h *Handler) HandleEnableDNSSEC(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.client.EnableDNSSEC(r.Context(), zoneID)
	if err != nil {
		handleBunnyError(w, err)
		return
	}

	h.logger.Info("enable DNSSEC", "zone_id", zoneID)

	writeJSON(w, http.StatusOK, result)
}

// HandleDisableDNSSEC disables DNSSEC for a DNS zone.
// DELETE /dnszone/{zoneID}/dnssec
// Admin only — sensitive zone-level configuration.
func (h *Handler) HandleDisableDNSSEC(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.client.DisableDNSSEC(r.Context(), zoneID)
	if err != nil {
		handleBunnyError(w, err)
		return
	}

	h.logger.Info("disable DNSSEC", "zone_id", zoneID)

	writeJSON(w, http.StatusOK, result)
}

// HandleIssueCertificate triggers issuance of a wildcard SSL certificate.
// POST /dnszone/{zoneID}/certificate/issue
// Admin only — certificate issuance is security-sensitive.
func (h *Handler) HandleIssueCertificate(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		Domain string `json:"Domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.client.IssueCertificate(r.Context(), zoneID, req.Domain); err != nil {
		handleBunnyError(w, err)
		return
	}

	h.logger.Info("issue certificate", "zone_id", zoneID, "domain", req.Domain)

	w.WriteHeader(http.StatusOK)
}

// HandleGetStatistics retrieves DNS query statistics for a zone.
// GET /dnszone/{zoneID}/statistics
// Admin only — statistics are outside the record-level permission model.
func (h *Handler) HandleGetStatistics(w http.ResponseWriter, r *http.Request) {
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

	dateFrom := r.URL.Query().Get("dateFrom")
	dateTo := r.URL.Query().Get("dateTo")

	result, err := h.client.GetZoneStatistics(r.Context(), zoneID, dateFrom, dateTo)
	if err != nil {
		handleBunnyError(w, err)
		return
	}

	h.logger.Info("get statistics", "zone_id", zoneID)

	writeJSON(w, http.StatusOK, result)
}

// HandleTriggerScan triggers a background DNS record scan for a domain.
// POST /dnszone/records/scan
// Admin only — operational/maintenance action.
func (h *Handler) HandleTriggerScan(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.client.TriggerDNSScan(r.Context(), req.Domain)
	if err != nil {
		handleBunnyError(w, err)
		return
	}

	h.logger.Info("trigger DNS scan", "domain", req.Domain)

	writeJSON(w, http.StatusOK, result)
}

// HandleGetScanResult retrieves the latest DNS record scan result.
// GET /dnszone/{zoneID}/records/scan
// Admin only — operational/maintenance action.
func (h *Handler) HandleGetScanResult(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.client.GetDNSScanResult(r.Context(), zoneID)
	if err != nil {
		handleBunnyError(w, err)
		return
	}

	h.logger.Info("get DNS scan result", "zone_id", zoneID)

	writeJSON(w, http.StatusOK, result)
}

// Paging limits of bunny.net's GET /dnszone/{zoneId}/records.
const (
	recordsDefaultPerPage = 1000
	recordsMinPerPage     = 5
	recordsMaxPerPage     = 1000
	// recordsMaxPages bounds how many bunny.net pages one request reads
	// (1000 records each), as a guard against a runaway HasMoreItems.
	recordsMaxPages = 100
)

// HandleListRecords implements bunny.net's GET /dnszone/{zoneId}/records:
// a paginated list (CurrentPage, TotalItems, HasMoreItems, Items) with the
// optional query parameters page, perPage, type and search.
//
// Scoped tokens only see the record types they may access. Filtering one
// bunny.net page would give short pages and a TotalItems that counts hidden
// records, so the proxy reads every matching record from bunny.net, filters
// them, and pages the result itself: TotalItems and HasMoreItems describe
// only what the token may see.
func (h *Handler) HandleListRecords(w http.ResponseWriter, r *http.Request) {
	zoneID, err := strconv.ParseInt(chi.URLParam(r, "zoneID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid zone ID")
		return
	}

	q := r.URL.Query()
	page, perPage := 1, recordsDefaultPerPage
	if v := q.Get("page"); v != "" {
		if page, err = strconv.Atoi(v); err != nil || page < 1 {
			writeError(w, http.StatusBadRequest, "invalid page parameter: must be a number of at least 1")
			return
		}
	}
	if v := q.Get("perPage"); v != "" {
		if perPage, err = strconv.Atoi(v); err != nil || perPage < recordsMinPerPage || perPage > recordsMaxPerPage {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid perPage parameter: must be between %d and %d", recordsMinPerPage, recordsMaxPerPage))
			return
		}
	}
	upstream := bunny.ListRecordsOptions{PerPage: recordsMaxPerPage, Search: q.Get("search")}
	if v := q.Get("type"); v != "" {
		t, err := strconv.Atoi(v)
		if err != nil || auth.MapRecordTypeToString(t) == "" {
			writeError(w, http.StatusBadRequest, "invalid type parameter: must be a bunny.net record type (0-15)")
			return
		}
		upstream.Type = &t
	}

	// Read every matching record from bunny.net.
	var all []bunny.Record
	for p := 1; p <= recordsMaxPages; p++ {
		upstream.Page = p
		resp, err := h.client.ListRecords(r.Context(), zoneID, &upstream)
		if err != nil {
			handleBunnyError(w, err)
			return
		}
		all = append(all, resp.Items...)
		if !resp.HasMoreItems || len(resp.Items) == 0 {
			break
		}
	}

	visible := filterRecordsByPermission(all, auth.GetKeyInfo(r.Context()), zoneID)
	if visible == nil {
		visible = []bunny.Record{} // Items is always a list, never null
	}

	from := (page - 1) * perPage
	if from > len(visible) {
		from = len(visible)
	}
	to := from + perPage
	if to > len(visible) {
		to = len(visible)
	}
	h.logger.Info("list records", "zone_id", zoneID, "visible", len(visible), "page", page)
	writeJSON(w, http.StatusOK, bunny.ListRecordsResponse{
		CurrentPage:  page,
		TotalItems:   len(visible),
		HasMoreItems: to < len(visible),
		Items:        visible[from:to],
	})
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

// HandleUpdateRecord updates an existing DNS record in the specified zone.
func (h *Handler) HandleUpdateRecord(w http.ResponseWriter, r *http.Request) {
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

	// Decode request body
	var req bunny.AddRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// The middleware checked the record type in the body; the record being
	// changed must be of a permitted type too.
	if !h.authorizeExistingRecord(w, r, zoneID, recordID) {
		return
	}

	// Call client to update record — validation is delegated to the backend
	// (bunny.net API has nuanced validation rules per record type)
	record, err := h.client.UpdateRecord(r.Context(), zoneID, recordID, &req)
	if err != nil {
		handleBunnyError(w, err)
		return
	}

	// Log the request
	h.logger.Info("update record", "zone_id", zoneID, "record_id", recordID, "type", req.Type, "name", req.Name)

	// If record is nil (204 No Content from backend), return 204
	if record == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Return 200 OK with the record
	writeJSON(w, http.StatusOK, record)
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

	// A delete request names no record type: check the existing record's.
	if !h.authorizeExistingRecord(w, r, zoneID, recordID) {
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
