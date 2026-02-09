package mockbunny

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// CreateZoneRequest is the request body for POST /admin/zones
type CreateZoneRequest struct {
	Domain string `json:"domain"`
}

// CreateRecordRequest is the request body for POST /admin/zones/{zoneId}/records
type CreateRecordRequest struct {
	Type  int    `json:"Type"` // 0 = A, 1 = AAAA, 2 = CNAME, 3 = TXT, 4 = MX, 5 = SPF, 6 = Flatten, 7 = PullZone, 8 = SRV, 9 = CAA, 10 = PTR, 11 = Script, 12 = NS
	Name  string `json:"Name"`
	Value string `json:"Value"`
	TTL   int32  `json:"Ttl"`
}

// StateResponse is the response for GET /admin/state
type StateResponse struct {
	Zones        []Zone `json:"zones"`
	NextZoneID   int64  `json:"nextZoneId"`
	NextRecordID int64  `json:"nextRecordId"`
}

// handleAdminCreateZone handles POST /admin/zones
// Creates a new zone with the given domain
func (s *Server) handleAdminCreateZone(w http.ResponseWriter, r *http.Request) {
	var req CreateZoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_JSON", "", "Invalid request body")
		return
	}

	if req.Domain == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_DOMAIN", "domain", "Domain is required")
		return
	}

	zoneID := s.AddZone(req.Domain)
	zone := s.GetZone(zoneID)

	writeJSON(w, http.StatusCreated, zone)
}

// handleAdminCreateRecord handles POST /admin/zones/{zoneId}/records
// Creates a new record in the specified zone
func (s *Server) handleAdminCreateRecord(w http.ResponseWriter, r *http.Request) {
	zoneIDStr := chi.URLParam(r, "zoneId")
	zoneID, err := strconv.ParseInt(zoneIDStr, 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_ZONE_ID", "zoneId", "Invalid zone ID")
		return
	}

	var req CreateRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_JSON", "", "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" || req.Value == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "name,value", "Name and value are required")
		return
	}

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	zone, exists := s.state.zones[zoneID]
	if !exists {
		s.writeError(w, http.StatusNotFound, "ZONE_NOT_FOUND", "zoneId", "Zone not found")
		return
	}

	// Create record with sensible defaults
	record := Record{
		ID:               s.state.nextRecordID,
		Type:             req.Type,
		Name:             req.Name,
		Value:            req.Value,
		TTL:              req.TTL,
		MonitorStatus:    0, // 0 = Unknown
		MonitorType:      0, // 0 = None
		SmartRoutingType: 0, // 0 = None
	}
	s.state.nextRecordID++
	zone.Records = append(zone.Records, record)

	writeJSON(w, http.StatusCreated, record)
}

// handleAdminReset handles DELETE /admin/reset
// Clears all zones and records, resetting ID counters, scan state, and failure injection state
func (s *Server) handleAdminReset(w http.ResponseWriter, r *http.Request) {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	s.state.zones = make(map[int64]*Zone)
	s.state.nextZoneID = 1
	s.state.nextRecordID = 1
	s.state.scanTriggered = make(map[int64]bool)
	s.state.scanCallCount = make(map[int64]int)
	// Clear failure injection state with proper initialization
	s.state.failureInjection = FailureInjection{
		rateLimitAfter: -1,
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleAdminState handles GET /admin/state
// Returns the full server state for debugging
func (s *Server) handleAdminState(w http.ResponseWriter, r *http.Request) {
	s.state.mu.RLock()
	zones := make([]Zone, 0, len(s.state.zones))
	for _, z := range s.state.zones {
		zones = append(zones, *z)
	}
	nextZoneID := s.state.nextZoneID
	nextRecordID := s.state.nextRecordID
	s.state.mu.RUnlock()

	resp := StateResponse{
		Zones:        zones,
		NextZoneID:   nextZoneID,
		NextRecordID: nextRecordID,
	}

	writeJSON(w, http.StatusOK, resp)
}
