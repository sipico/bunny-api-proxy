package mockbunny

import (
	"encoding/json"
	"fmt"
	"io"
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
	var zones []*Zone
	for _, zone := range s.state.zones {
		if search == "" || strings.Contains(zone.Domain, search) {
			zones = append(zones, zone)
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

	var paginatedZones []*Zone
	if start >= total {
		paginatedZones = []*Zone{}
	} else {
		if end > total {
			end = total
		}
		paginatedZones = zones[start:end]
	}

	// Convert zones to short time format for GET response
	shortZones := make([]ZoneShortTime, len(paginatedZones))
	for i, zone := range paginatedZones {
		shortZones[i] = *zone.ZoneShortTime()
	}

	resp := ListZonesResponse{
		Items:        shortZones,
		CurrentPage:  page,
		TotalItems:   total,
		HasMoreItems: end < total,
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleGetZone handles GET /dnszone/{id} requests.
// It returns the zone JSON if found, or 404 if not found.
// Returns 400 for invalid (non-numeric) zone IDs.
// Timestamps are formatted without sub-second precision or Z suffix to match real API.
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
		s.writeError(w, http.StatusNotFound, "dnszone.zone.not_found", "Id", "The requested DNS zone was not found")
		return
	}

	// Convert zone to short time format for GET response
	shortZone := zone.ZoneShortTime()
	writeJSON(w, http.StatusOK, shortZone)
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
		s.writeError(w, http.StatusNotFound, "dnszone.zone.not_found", "Id", "The requested DNS zone was not found")
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
		s.writeError(w, http.StatusNotFound, "dnszone.record.not_found", "RecordId", "The requested DNS zone record was not found")
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

	// Note: the real bunny.net API does NOT validate required fields on update.
	// It accepts partial updates (e.g., empty Name/Value) and returns 204.
	// We match this lenient behavior in the mock.

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	zone, ok := s.state.zones[zoneID]
	if !ok {
		s.writeError(w, http.StatusNotFound, "dnszone.zone.not_found", "Id", "The requested DNS zone was not found")
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
		s.writeError(w, http.StatusNotFound, "dnszone.record.not_found", "RecordId", "The requested DNS zone record was not found")
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
	// The real bunny.net API allows empty Name for MX (4) and CAA (9) records
	if req.Name == "" && req.Type != 4 && req.Type != 9 {
		s.writeError(w, http.StatusBadRequest, "validation_error", "Name", "Name is required")
		return
	}

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	zone, ok := s.state.zones[zoneID]
	if !ok {
		s.writeError(w, http.StatusNotFound, "dnszone.zone.not_found", "Id", "The requested DNS zone was not found")
		return
	}

	// Create record with defaults
	record := Record{
		ID:                    s.state.nextRecordID,
		Type:                  req.Type,
		Name:                  req.Name,
		Value:                 req.Value,
		TTL:                   req.TTL,
		Priority:              req.Priority,
		Weight:                req.Weight,
		Port:                  req.Port,
		Flags:                 req.Flags,
		Tag:                   req.Tag,
		Disabled:              req.Disabled,
		Comment:               req.Comment,
		LinkName:              "",
		IPGeoLocationInfo:     nil,
		GeolocationInfo:       nil,
		MonitorStatus:         0, // 0 = Unknown
		MonitorType:           0, // 0 = None
		EnviromentalVariables: []interface{}{},
		SmartRoutingType:      0, // 0 = None
		AutoSslIssuance:       true,
		AccelerationStatus:    0,
	}
	s.state.nextRecordID++

	zone.Records = append(zone.Records, record)
	zone.DateModified = MockBunnyTime{Time: time.Now().UTC()}

	writeJSON(w, http.StatusCreated, record)
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
		Nameserver1:              "kiki.bunny.net",
		Nameserver2:              "coco.bunny.net",
		NameserversNextCheck:     MockBunnyTime{Time: time.Now().Add(5 * time.Minute)},
		SoaEmail:                 "hostmaster@bunny.net",
		LoggingEnabled:           false,
		LoggingIPAnonymization:   true,
		LogAnonymizationType:     0, // 0 = OneDigit (default)
		DnsSecEnabled:            false,
		CertificateKeyType:       0, // 0 = Ecdsa (default)
	}

	s.state.zones[id] = zone

	// Return 201 Created with zone JSON
	writeJSON(w, http.StatusCreated, zone)
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
		s.writeError(w, http.StatusNotFound, "dnszone.zone.not_found", "Id", "The requested DNS zone was not found")
		return
	}

	// Delete zone
	delete(s.state.zones, id)

	// Return 204 No Content on success
	w.WriteHeader(http.StatusNoContent)
}

// handleUpdateZone handles POST /dnszone/{id} to update zone-level settings.
// Returns 200 OK with updated zone, 404 if zone not found, 400 for invalid zone ID.
func (s *Server) handleUpdateZone(w http.ResponseWriter, r *http.Request) {
	// Parse zone ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid zone ID", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req struct {
		CustomNameserversEnabled      *bool   `json:"CustomNameserversEnabled,omitempty"`
		Nameserver1                   *string `json:"Nameserver1,omitempty"`
		Nameserver2                   *string `json:"Nameserver2,omitempty"`
		SoaEmail                      *string `json:"SoaEmail,omitempty"`
		LoggingEnabled                *bool   `json:"LoggingEnabled,omitempty"`
		LogAnonymizationType          *int    `json:"LogAnonymizationType,omitempty"`
		CertificateKeyType            *int    `json:"CertificateKeyType,omitempty"`
		LoggingIPAnonymizationEnabled *bool   `json:"LoggingIPAnonymizationEnabled,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	// Find zone
	zone, ok := s.state.zones[id]
	if !ok {
		s.writeError(w, http.StatusNotFound, "dnszone.zone.not_found", "Id", "The requested DNS zone was not found")
		return
	}

	// Apply updates to non-nil fields
	if req.CustomNameserversEnabled != nil {
		zone.CustomNameserversEnabled = *req.CustomNameserversEnabled
	}
	if req.Nameserver1 != nil {
		zone.Nameserver1 = *req.Nameserver1
	}
	if req.Nameserver2 != nil {
		zone.Nameserver2 = *req.Nameserver2
	}
	if req.SoaEmail != nil {
		zone.SoaEmail = *req.SoaEmail
	}
	if req.LoggingEnabled != nil {
		zone.LoggingEnabled = *req.LoggingEnabled
	}
	if req.LogAnonymizationType != nil {
		zone.LogAnonymizationType = *req.LogAnonymizationType
	}
	if req.CertificateKeyType != nil {
		zone.CertificateKeyType = *req.CertificateKeyType
	}
	if req.LoggingIPAnonymizationEnabled != nil {
		zone.LoggingIPAnonymization = *req.LoggingIPAnonymizationEnabled
	}

	// Update modification time
	zone.DateModified = MockBunnyTime{Time: time.Now().UTC()}

	// Return updated zone
	writeJSON(w, http.StatusOK, zone)
}

// wellKnownUnavailableDomains contains domains that the real bunny.net API reports
// as unavailable because they are already registered globally. The mock mirrors
// this behavior so tests produce the same results against both mock and real API.
var wellKnownUnavailableDomains = map[string]bool{
	"amazon.com":    true,
	"google.com":    true,
	"siemens.com":   true,
	"shell.com":     true,
	"nestle.com":    true,
	"sap.com":       true,
	"lvmh.com":      true,
	"unilever.com":  true,
	"microsoft.com": true,
	"apple.com":     true,
}

// handleCheckAvailability handles POST /dnszone/checkavailability to check domain availability.
// Matches real bunny.net API behavior: checks both internal zone state AND well-known
// registered domains. The real API queries domain registries, so well-known domains
// like amazon.com always return Available: false regardless of account state.
func (s *Server) handleCheckAvailability(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"Name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		s.writeError(w, http.StatusBadRequest, "validation_error", "Name", "Name is required")
		return
	}

	// Check well-known registered domains first (simulates registry check)
	if wellKnownUnavailableDomains[req.Name] {
		writeJSON(w, http.StatusOK, struct {
			Available bool `json:"Available"`
		}{Available: false})
		return
	}

	s.state.mu.RLock()
	defer s.state.mu.RUnlock()

	// Check if any zone already has this domain
	available := true
	for _, zone := range s.state.zones {
		if zone.Domain == req.Name {
			available = false
			break
		}
	}

	writeJSON(w, http.StatusOK, struct {
		Available bool `json:"Available"`
	}{Available: available})
}

// handleImportRecords handles POST /dnszone/{id}/import to import DNS records.
func (s *Server) handleImportRecords(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid zone ID", http.StatusBadRequest)
		return
	}

	s.state.mu.RLock()
	_, ok := s.state.zones[id]
	s.state.mu.RUnlock()

	if !ok {
		s.writeError(w, http.StatusNotFound, "dnszone.zone.not_found", "Id", "The requested DNS zone was not found")
		return
	}

	// Read and discard body (simulating import processing)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Simulate import: count lines that look like DNS records
	// For mock purposes, count non-empty, non-comment lines as "successful"
	successful := 0
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, ";") {
			successful++
		}
	}

	writeJSON(w, http.StatusOK, struct {
		TotalRecordsParsed int `json:"TotalRecordsParsed"`
		Created            int `json:"Created"`
		Failed             int `json:"Failed"`
		Skipped            int `json:"Skipped"`
	}{
		TotalRecordsParsed: successful,
		Created:            successful,
		Failed:             0,
		Skipped:            0,
	})
}

// handleExportRecords handles GET /dnszone/{id}/export to export DNS records.
func (s *Server) handleExportRecords(w http.ResponseWriter, r *http.Request) {
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
		s.writeError(w, http.StatusNotFound, "dnszone.zone.not_found", "Id", "The requested DNS zone was not found")
		return
	}

	// Build BIND zone file format
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(";; Zone: %s\n", zone.Domain))
	for _, rec := range zone.Records {
		typeName := recordTypeName(rec.Type)
		sb.WriteString(fmt.Sprintf("%s\t%d\tIN\t%s\t%s\n", rec.Name, rec.TTL, typeName, rec.Value))
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck
	w.Write([]byte(sb.String()))
}

// handleEnableDNSSEC handles POST /dnszone/{id}/dnssec to enable DNSSEC.
func (s *Server) handleEnableDNSSEC(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid zone ID", http.StatusBadRequest)
		return
	}

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	zone, ok := s.state.zones[id]
	if !ok {
		s.writeError(w, http.StatusNotFound, "dnszone.zone.not_found", "Id", "The requested DNS zone was not found")
		return
	}

	zone.DnsSecEnabled = true

	dsDigest := "AABBCCDD"
	dnsKeyPublicKey := "mockpublickey123"
	writeJSON(w, http.StatusOK, struct {
		DnsSecEnabled   bool    `json:"DnsSecEnabled"`
		DnsSecAlgorithm int     `json:"DnsSecAlgorithm"`
		DsKeyTag        int     `json:"DsKeyTag"`
		DsAlgorithm     int     `json:"DsAlgorithm"`
		DsDigestType    int     `json:"DsDigestType"`
		DsDigest        *string `json:"DsDigest"`
		DnsKeyFlags     int     `json:"DnsKeyFlags"`
		DnsKeyAlgorithm int     `json:"DnsKeyAlgorithm"`
		DnsKeyPublicKey *string `json:"DnsKeyPublicKey"`
	}{
		DnsSecEnabled:   true,
		DnsSecAlgorithm: 13,
		DsKeyTag:        12345,
		DsAlgorithm:     13,
		DsDigestType:    2,
		DsDigest:        &dsDigest,
		DnsKeyFlags:     257,
		DnsKeyAlgorithm: 13,
		DnsKeyPublicKey: &dnsKeyPublicKey,
	})
}

// handleDisableDNSSEC handles DELETE /dnszone/{id}/dnssec to disable DNSSEC.
func (s *Server) handleDisableDNSSEC(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid zone ID", http.StatusBadRequest)
		return
	}

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	zone, ok := s.state.zones[id]
	if !ok {
		s.writeError(w, http.StatusNotFound, "dnszone.zone.not_found", "Id", "The requested DNS zone was not found")
		return
	}

	zone.DnsSecEnabled = false

	writeJSON(w, http.StatusOK, struct {
		DnsSecEnabled   bool    `json:"DnsSecEnabled"`
		DnsSecAlgorithm int     `json:"DnsSecAlgorithm"`
		DsKeyTag        int     `json:"DsKeyTag"`
		DsAlgorithm     int     `json:"DsAlgorithm"`
		DsDigestType    int     `json:"DsDigestType"`
		DsDigest        *string `json:"DsDigest"`
		DnsKeyFlags     int     `json:"DnsKeyFlags"`
		DnsKeyAlgorithm int     `json:"DnsKeyAlgorithm"`
		DnsKeyPublicKey *string `json:"DnsKeyPublicKey"`
	}{
		DnsSecEnabled:   false,
		DnsSecAlgorithm: 0,
		DsKeyTag:        0,
		DsAlgorithm:     0,
		DsDigestType:    0,
		DsDigest:        nil,
		DnsKeyFlags:     0,
		DnsKeyAlgorithm: 0,
		DnsKeyPublicKey: nil,
	})
}

// handleIssueCertificate handles POST /dnszone/{id}/certificate/issue.
func (s *Server) handleIssueCertificate(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid zone ID", http.StatusBadRequest)
		return
	}

	s.state.mu.RLock()
	_, ok := s.state.zones[id]
	s.state.mu.RUnlock()

	if !ok {
		s.writeError(w, http.StatusNotFound, "dnszone.zone.not_found", "Id", "The requested DNS zone was not found")
		return
	}

	// Read and discard body
	//nolint:errcheck
	io.ReadAll(r.Body)

	w.WriteHeader(http.StatusOK)
}

// handleGetStatistics handles GET /dnszone/{id}/statistics.
func (s *Server) handleGetStatistics(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid zone ID", http.StatusBadRequest)
		return
	}

	s.state.mu.RLock()
	_, ok := s.state.zones[id]
	s.state.mu.RUnlock()

	if !ok {
		s.writeError(w, http.StatusNotFound, "dnszone.zone.not_found", "Id", "The requested DNS zone was not found")
		return
	}

	writeJSON(w, http.StatusOK, struct {
		TotalQueriesServed       int64            `json:"TotalQueriesServed"`
		QueriesServedChart       map[string]int64 `json:"QueriesServedChart"`
		NormalQueriesServedChart map[string]int64 `json:"NormalQueriesServedChart"`
		SmartQueriesServedChart  map[string]int64 `json:"SmartQueriesServedChart"`
		QueriesByTypeChart       map[string]int64 `json:"QueriesByTypeChart"`
	}{
		TotalQueriesServed:       1000,
		QueriesServedChart:       map[string]int64{"2025-01-01": 500, "2025-01-02": 500},
		NormalQueriesServedChart: map[string]int64{"2025-01-01": 400, "2025-01-02": 400},
		SmartQueriesServedChart:  map[string]int64{"2025-01-01": 100, "2025-01-02": 100},
		QueriesByTypeChart:       map[string]int64{"A": 600, "AAAA": 200, "TXT": 200},
	})
}

// handleTriggerScan handles POST /dnszone/records/scan to trigger a DNS scan.
// Takes {"Domain": "..."} in body. Returns Status: 1 (InProgress) immediately.
// Matches real bunny.net API behavior: accepts any domain (not just account zones),
// returns 200 OK with Status 1 and empty Records.
func (s *Server) handleTriggerScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain string `json:"Domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Domain == "" {
		s.writeError(w, http.StatusBadRequest, "validation_error", "Domain", "Domain is required")
		return
	}

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	// Find zone by domain (if it exists in our state, track scan for GET result polling)
	for id, zone := range s.state.zones {
		if zone.Domain == req.Domain {
			s.state.scanTriggered[id] = true
			s.state.scanCallCount[id] = 0
			break
		}
	}

	// Real API accepts any domain and returns 200 with Status 1 (InProgress)
	writeJSON(w, http.StatusOK, struct {
		Status  int           `json:"Status"`
		Records []interface{} `json:"Records"`
	}{Status: 1, Records: []interface{}{}})
}

// scanRecord represents a record in a scan result response.
type scanRecord struct {
	Name      string      `json:"Name"`
	Type      int         `json:"Type"`
	TTL       int32       `json:"Ttl"`
	Value     string      `json:"Value"`
	Priority  interface{} `json:"Priority"`
	Weight    interface{} `json:"Weight"`
	Port      interface{} `json:"Port"`
	IsProxied bool        `json:"IsProxied"`
}

// handleGetScanResult handles GET /dnszone/{id}/records/scan to get scan results.
// Simulates the async scan lifecycle:
// - No scan triggered: Status 0 (NotStarted)
// - First poll after trigger: Status 1 (InProgress)
// - Second+ poll after trigger: Status 2 (Completed) with zone records
func (s *Server) handleGetScanResult(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid zone ID", http.StatusBadRequest)
		return
	}

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	zone, ok := s.state.zones[id]
	if !ok {
		s.writeError(w, http.StatusNotFound, "dnszone.zone.not_found", "Id", "The requested DNS zone was not found")
		return
	}

	// Determine scan status based on trigger state
	if !s.state.scanTriggered[id] {
		// No scan triggered — Status 0 (NotStarted)
		writeJSON(w, http.StatusOK, struct {
			Status  int           `json:"Status"`
			Records []interface{} `json:"Records"`
		}{Status: 0, Records: []interface{}{}})
		return
	}

	s.state.scanCallCount[id]++

	if s.state.scanCallCount[id] <= 1 {
		// First poll — Status 1 (InProgress)
		writeJSON(w, http.StatusOK, struct {
			Status  int           `json:"Status"`
			Records []interface{} `json:"Records"`
		}{Status: 1, Records: []interface{}{}})
		return
	}

	// Second+ poll — Status 2 (Completed) with zone records
	records := make([]scanRecord, len(zone.Records))
	for i, rec := range zone.Records {
		records[i] = scanRecord{
			Name:      rec.Name,
			Type:      rec.Type,
			TTL:       rec.TTL,
			Value:     rec.Value,
			Priority:  nil,
			Weight:    nil,
			Port:      nil,
			IsProxied: false,
		}
	}

	writeJSON(w, http.StatusOK, struct {
		Status  int          `json:"Status"`
		Records []scanRecord `json:"Records"`
	}{Status: 2, Records: records})
}

// recordTypeName converts a record type integer to its DNS name.
func recordTypeName(t int) string {
	switch t {
	case 0:
		return "A"
	case 1:
		return "AAAA"
	case 2:
		return "CNAME"
	case 3:
		return "TXT"
	case 4:
		return "MX"
	case 5:
		return "SPF"
	case 6:
		return "REDIRECT"
	case 7:
		return "PULLZONE"
	case 8:
		return "SRV"
	case 9:
		return "CAA"
	case 10:
		return "PTR"
	case 11:
		return "SCRIPT"
	case 12:
		return "NS"
	default:
		return "UNKNOWN"
	}
}

// writeJSON writes a JSON response with correct Content-Type and no trailing newline.
func writeJSON(w http.ResponseWriter, status int, v any) {
	//nolint:errcheck
	data, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	//nolint:errcheck
	w.Write(data)
}

// writeError writes an error response in the bunny.net API format.
func (s *Server) writeError(w http.ResponseWriter, status int, errorKey, field, message string) {
	writeJSON(w, status, ErrorResponse{
		ErrorKey: errorKey,
		Field:    field,
		Message:  message + "\r",
	})
}
