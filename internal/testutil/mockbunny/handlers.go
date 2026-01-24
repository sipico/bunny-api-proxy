package mockbunny

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// handleListZones handles GET /dnszone requests.
// It returns a paginated list of zones with optional search filtering.
func (s *Server) handleListZones(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	page := 1
	perPage := 1000
	search := r.URL.Query().Get("search")

	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	if pp := r.URL.Query().Get("perPage"); pp != "" {
		if parsed, err := strconv.Atoi(pp); err == nil && parsed >= 5 && parsed <= 1000 {
			perPage = parsed
		}
	}

	s.state.mu.RLock()
	defer s.state.mu.RUnlock()

	// Collect and filter zones
	var zones []Zone
	for _, zone := range s.state.zones {
		if search == "" || strings.Contains(zone.Domain, search) {
			zones = append(zones, *zone)
		}
	}

	// Sort by ID for consistent ordering
	sort.Slice(zones, func(i, j int) bool {
		return zones[i].ID < zones[j].ID
	})

	// Paginate
	total := len(zones)
	start := (page - 1) * perPage
	end := start + perPage

	if start >= total {
		zones = []Zone{}
	} else {
		if end > total {
			end = total
		}
		zones = zones[start:end]
	}

	resp := ListZonesResponse{
		CurrentPage:  page,
		TotalItems:   total,
		HasMoreItems: end < total,
		Items:        zones,
	}

	w.Header().Set("Content-Type", "application/json")
	//nolint:errcheck
	json.NewEncoder(w).Encode(resp)
}

// handleGetZone handles GET /dnszone/{id} requests.
// It returns the zone JSON if found, or 404 if not found.
// Returns 400 for invalid (non-numeric) zone IDs.
func (s *Server) handleGetZone(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid zone ID", http.StatusBadRequest)
		return
	}

	s.state.mu.RLock()
	zone, ok := s.state.zones[id]
	s.state.mu.RUnlock()

	if !ok {
		http.Error(w, "zone not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	//nolint:errcheck
	json.NewEncoder(w).Encode(zone)
}

// handleDeleteRecord handles DELETE /dnszone/{zoneId}/records/{id}
// Returns 204 No Content on success, 404 if zone or record not found, 400 for invalid IDs.
func (s *Server) handleDeleteRecord(w http.ResponseWriter, r *http.Request) {
	// Parse zone ID from URL
	zoneIDStr := chi.URLParam(r, "zoneId")
	zoneID, err := strconv.ParseInt(zoneIDStr, 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid zone ID: %v", err), http.StatusBadRequest)
		return
	}

	// Parse record ID from URL
	recordIDStr := chi.URLParam(r, "id")
	recordID, err := strconv.ParseInt(recordIDStr, 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid record ID: %v", err), http.StatusBadRequest)
		return
	}

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	// Check if zone exists
	zone, ok := s.state.zones[zoneID]
	if !ok {
		http.Error(w, "zone not found", http.StatusNotFound)
		return
	}

	// Find and remove record
	found := false
	for i, record := range zone.Records {
		if record.ID == recordID {
			zone.Records = append(zone.Records[:i], zone.Records[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "record not found", http.StatusNotFound)
		return
	}

	// Update zone's DateModified
	zone.DateModified = time.Now().UTC()

	// Return 204 No Content on success
	w.WriteHeader(http.StatusNoContent)
}

// AddRecordRequest represents the request body for creating a new DNS record.
type AddRecordRequest struct {
	Type     string `json:"Type"`
	Name     string `json:"Name"`
	Value    string `json:"Value"`
	TTL      int32  `json:"Ttl"`
	Priority int32  `json:"Priority"`
	Weight   int32  `json:"Weight"`
	Port     int32  `json:"Port"`
	Flags    int    `json:"Flags"`
	Tag      string `json:"Tag"`
	Disabled bool   `json:"Disabled"`
	Comment  string `json:"Comment"`
}

// handleAddRecord handles PUT /dnszone/{zoneId}/records to add a new DNS record.
func (s *Server) handleAddRecord(w http.ResponseWriter, r *http.Request) {
	zoneIDStr := chi.URLParam(r, "zoneId")
	zoneID, err := strconv.ParseInt(zoneIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid zone ID", http.StatusBadRequest)
		return
	}

	var req AddRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Type == "" {
		s.writeError(w, http.StatusBadRequest, "validation.error", "Type", "Type is required")
		return
	}
	if req.Name == "" {
		s.writeError(w, http.StatusBadRequest, "validation.error", "Name", "Name is required")
		return
	}
	if req.Value == "" {
		s.writeError(w, http.StatusBadRequest, "validation.error", "Value", "Value is required")
		return
	}

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	zone, ok := s.state.zones[zoneID]
	if !ok {
		http.Error(w, "zone not found", http.StatusNotFound)
		return
	}

	// Create record with defaults
	record := Record{
		ID:               s.state.nextRecordID,
		Type:             req.Type,
		Name:             req.Name,
		Value:            req.Value,
		TTL:              req.TTL,
		Priority:         req.Priority,
		Weight:           req.Weight,
		Port:             req.Port,
		Flags:            req.Flags,
		Tag:              req.Tag,
		Disabled:         req.Disabled,
		Comment:          req.Comment,
		MonitorStatus:    "Unknown",
		MonitorType:      "None",
		SmartRoutingType: "None",
	}
	s.state.nextRecordID++

	zone.Records = append(zone.Records, record)
	zone.DateModified = time.Now().UTC()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	//nolint:errcheck
	json.NewEncoder(w).Encode(record)
}

// writeError writes an error response in the bunny.net API format.
func (s *Server) writeError(w http.ResponseWriter, status int, errorKey, field, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	//nolint:errcheck
	json.NewEncoder(w).Encode(ErrorResponse{
		ErrorKey: errorKey,
		Field:    field,
		Message:  message,
	})
}
