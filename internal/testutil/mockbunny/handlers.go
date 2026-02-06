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
	zone.DateModified = MockBunnyTime{Time: time.Now().UTC()}

	// Return 204 No Content on success
	w.WriteHeader(http.StatusNoContent)
}

// AddRecordRequest represents the request body for creating a new DNS record.
type AddRecordRequest struct {
	Type     int    `json:"Type"` // 0 = A, 1 = AAAA, 2 = CNAME, 3 = TXT, 4 = MX, 5 = SPF, 6 = Flatten, 7 = PullZone, 8 = SRV, 9 = CAA, 10 = PTR, 11 = Script, 12 = NS
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

// handleUpdateRecord handles POST /dnszone/{zoneId}/records/{id} to update an existing DNS record.
func (s *Server) handleUpdateRecord(w http.ResponseWriter, r *http.Request) {
	// Parse zone ID from URL
	zoneIDStr := chi.URLParam(r, "zoneId")
	zoneID, err := strconv.ParseInt(zoneIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid zone ID", http.StatusBadRequest)
		return
	}

	// Parse record ID from URL
	recordIDStr := chi.URLParam(r, "id")
	recordID, err := strconv.ParseInt(recordIDStr, 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid record ID: %v", err), http.StatusBadRequest)
		return
	}

	var req AddRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Value == "" {
		s.writeError(w, http.StatusBadRequest, "validation_error", "Value", "Value is required")
		return
	}
	if req.Name == "" {
		s.writeError(w, http.StatusBadRequest, "validation_error", "Name", "Name is required")
		return
	}

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	zone, ok := s.state.zones[zoneID]
	if !ok {
		http.Error(w, "zone not found", http.StatusNotFound)
		return
	}

	// Find record by ID
	found := false
	for i, record := range zone.Records {
		if record.ID == recordID {
			// Update record fields (Type is immutable on real bunny.net API — do not update it)
			zone.Records[i].Name = req.Name
			zone.Records[i].Value = req.Value
			zone.Records[i].TTL = req.TTL
			zone.Records[i].Priority = req.Priority
			zone.Records[i].Weight = req.Weight
			zone.Records[i].Port = req.Port
			zone.Records[i].Flags = req.Flags
			zone.Records[i].Tag = req.Tag
			zone.Records[i].Disabled = req.Disabled
			zone.Records[i].Comment = req.Comment

			// Update zone's DateModified
			zone.DateModified = MockBunnyTime{Time: time.Now().UTC()}

			// Return 204 No Content on success (matching real bunny.net API behavior)
			w.WriteHeader(http.StatusNoContent)
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "record not found", http.StatusNotFound)
		return
	}
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
	// Type is validated implicitly (must be provided as int 0-12)
	if req.Value == "" {
		s.writeError(w, http.StatusBadRequest, "validation_error", "Value", "Value is required")
		return
	}
	if req.Name == "" {
		s.writeError(w, http.StatusBadRequest, "validation_error", "Name", "Name is required")
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
		MonitorStatus:    0, // 0 = Unknown
		MonitorType:      0, // 0 = None
		SmartRoutingType: 0, // 0 = None
	}
	s.state.nextRecordID++

	zone.Records = append(zone.Records, record)
	zone.DateModified = MockBunnyTime{Time: time.Now().UTC()}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	//nolint:errcheck
	json.NewEncoder(w).Encode(record)
}

// handleCreateZone handles POST /dnszone to create a new DNS zone.
// Returns 201 Created on success, 400 for invalid domain, 409 if zone already exists.
func (s *Server) handleCreateZone(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req struct {
		Domain string `json:"Domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate domain
	if req.Domain == "" {
		s.writeError(w, http.StatusBadRequest, "validation_error", "Domain", "Domain is required")
		return
	}

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	// Check if domain already exists
	for _, zone := range s.state.zones {
		if zone.Domain == req.Domain {
			s.writeError(w, http.StatusConflict, "conflict", "", "Zone already exists")
			return
		}
	}

	// Create new zone
	id := s.state.nextZoneID
	s.state.nextZoneID++

	now := MockBunnyTime{Time: time.Now().UTC()}
	zone := &Zone{
		ID:                       id,
		Domain:                   req.Domain,
		Records:                  []Record{},
		DateCreated:              now,
		DateModified:             now,
		NameserversDetected:      true,
		CustomNameserversEnabled: false,
		Nameserver1:              "ns1.bunny.net",
		Nameserver2:              "ns2.bunny.net",
		SoaEmail:                 "hostmaster@bunny.net",
		LoggingEnabled:           false,
		LogAnonymizationType:     0, // 0 = OneDigit (default)
		DnsSecEnabled:            false,
		CertificateKeyType:       0, // 0 = Ecdsa (default)
	}

	s.state.zones[id] = zone

	// Return 201 Created with zone JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	//nolint:errcheck
	json.NewEncoder(w).Encode(zone)
}

// handleDeleteZone handles DELETE /dnszone/{id} to delete a DNS zone.
// Returns 204 No Content on success, 404 if zone not found, 400 for invalid zone ID.
func (s *Server) handleDeleteZone(w http.ResponseWriter, r *http.Request) {
	// Parse zone ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid zone ID", http.StatusBadRequest)
		return
	}

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	// Check if zone exists
	_, ok := s.state.zones[id]
	if !ok {
		http.Error(w, "zone not found", http.StatusNotFound)
		return
	}

	// Delete zone
	delete(s.state.zones, id)

	// Return 204 No Content on success
	w.WriteHeader(http.StatusNoContent)
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
