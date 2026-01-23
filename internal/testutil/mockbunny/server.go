package mockbunny

import (
	"net/http/httptest"
	"time"

	"github.com/go-chi/chi/v5"
)

// Server represents a mock bunny.net server for testing.
// It wraps httptest.Server and maintains internal state for zones and records.
type Server struct {
	*httptest.Server
	state  *State
	router chi.Router
}

// New creates a new mock bunny.net server for testing.
// It initializes the server with placeholder routes that return 501 Not Implemented.
// The state is initialized with empty zones and auto-incrementing IDs.
func New() *Server {
	state := NewState()

	r := chi.NewRouter()

	ts := httptest.NewServer(r)

	server := &Server{
		Server: ts,
		state:  state,
		router: r,
	}

	// Wire up handlers
	r.Get("/dnszone", server.handleListZones)
	r.Get("/dnszone/{id}", server.handleGetZone)
	r.Put("/dnszone/{zoneId}/records", server.handleAddRecord)
	r.Delete("/dnszone/{zoneId}/records/{id}", server.handleDeleteRecord)

	return server
}

// URL returns the base URL of the mock server.
func (s *Server) URL() string {
	return s.Server.URL
}

// AddZone adds a zone with sensible defaults and returns its ID.
// This method is thread-safe and commonly used for test setup.
func (s *Server) AddZone(domain string) int64 {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	id := s.state.nextZoneID
	s.state.nextZoneID++

	now := time.Now().UTC()
	zone := &Zone{
		ID:                       id,
		Domain:                   domain,
		Records:                  []Record{},
		DateCreated:              now,
		DateModified:             now,
		NameserversDetected:      true,
		CustomNameserversEnabled: false,
		Nameserver1:              "ns1.bunny.net",
		Nameserver2:              "ns2.bunny.net",
		SoaEmail:                 "admin@" + domain,
		LoggingEnabled:           false,
		DnsSecEnabled:            false,
		CertificateKeyType:       "Ecdsa",
	}

	s.state.zones[id] = zone
	return id
}

// AddZoneWithRecords adds a zone with the given pre-populated records.
// Returns the zone ID. This method is thread-safe.
func (s *Server) AddZoneWithRecords(domain string, records []Record) int64 {
	id := s.AddZone(domain)

	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	zone := s.state.zones[id]
	for i := range records {
		records[i].ID = s.state.nextRecordID
		s.state.nextRecordID++
		// Set defaults for unset fields
		if records[i].MonitorStatus == "" {
			records[i].MonitorStatus = "Unknown"
		}
		if records[i].MonitorType == "" {
			records[i].MonitorType = "None"
		}
		if records[i].SmartRoutingType == "" {
			records[i].SmartRoutingType = "None"
		}
	}
	zone.Records = records
	zone.DateModified = time.Now().UTC()

	return id
}

// GetZone returns a copy of a zone by ID, or nil if not found.
// The returned copy is safe to modify without affecting internal state.
// This method is thread-safe.
func (s *Server) GetZone(id int64) *Zone {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()

	zone, ok := s.state.zones[id]
	if !ok {
		return nil
	}

	// Return a copy to prevent test code from modifying state
	copy := *zone
	copy.Records = make([]Record, len(zone.Records))
	for i, r := range zone.Records {
		copy.Records[i] = r
	}
	return &copy
}

// GetState returns a snapshot of all zones for debugging.
// The returned map contains copies of zones, safe to inspect without affecting state.
// This method is thread-safe.
func (s *Server) GetState() map[int64]Zone {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()

	result := make(map[int64]Zone)
	for id, zone := range s.state.zones {
		copy := *zone
		copy.Records = make([]Record, len(zone.Records))
		for i, r := range zone.Records {
			copy.Records[i] = r
		}
		result[id] = copy
	}
	return result
}
