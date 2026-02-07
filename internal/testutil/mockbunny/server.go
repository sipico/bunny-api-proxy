package mockbunny

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
)

// Server represents a mock bunny.net server for testing.
// It wraps httptest.Server and maintains internal state for zones and records.
type Server struct {
	*httptest.Server
	state  *State
	router chi.Router
	logger *slog.Logger
	apiKey string // Expected API key for authentication
}

// New creates a new mock bunny.net server for testing.
// It initializes the server with placeholder routes that return 501 Not Implemented.
// The state is initialized with empty zones and auto-incrementing IDs.
// If DEBUG environment variable is set to "true", HTTP request/response logging is enabled.
// If BUNNY_API_KEY is set, API key authentication is required for DNS API endpoints.
func New() *Server {
	state := NewState()

	r := chi.NewRouter()

	// Read expected API key from environment (optional)
	apiKey := os.Getenv("BUNNY_API_KEY")

	// Create logger if DEBUG=true
	var logger *slog.Logger
	if os.Getenv("DEBUG") == "true" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		if apiKey != "" {
			logger.Info("mockbunny initialized with API key authentication")
		}
	}

	ts := httptest.NewServer(r)

	server := &Server{
		Server: ts,
		state:  state,
		router: r,
		logger: logger,
		apiKey: apiKey,
	}

	// Apply logging middleware if logger present
	if logger != nil {
		r.Use(LoggingMiddleware(logger))
	}

	// Apply Vary: Accept-Encoding header middleware to GET responses
	r.Use(VaryAcceptEncodingMiddleware)

	// Wire up DNS API handlers with authentication (if API key is configured)
	r.Group(func(r chi.Router) {
		if apiKey != "" {
			r.Use(server.authMiddleware)
		}
		r.Get("/dnszone", server.handleListZones)
		r.Post("/dnszone", server.handleCreateZone)
		r.Post("/dnszone/checkavailability", server.handleCheckAvailability)
		r.Get("/dnszone/{id}", server.handleGetZone)
		r.Delete("/dnszone/{id}", server.handleDeleteZone)
		r.Post("/dnszone/{id}", server.handleUpdateZone)
		r.Put("/dnszone/{zoneId}/records", server.handleAddRecord)
		r.Post("/dnszone/{zoneId}/records/{id}", server.handleUpdateRecord)
		r.Delete("/dnszone/{zoneId}/records/{id}", server.handleDeleteRecord)
	})

	// Admin endpoints for test seeding (no authentication required)
	r.Route("/admin", func(r chi.Router) {
		r.Post("/zones", server.handleAdminCreateZone)
		r.Post("/zones/{zoneId}/records", server.handleAdminCreateRecord)
		r.Delete("/reset", server.handleAdminReset)
		r.Get("/state", server.handleAdminState)
	})

	return server
}

// authMiddleware validates the AccessKey header against the configured API key.
// Returns 401 Unauthorized if the key is missing or doesn't match.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accessKey := r.Header.Get("AccessKey")

		if accessKey == "" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			//nolint:errcheck // Error responses are best effort
			w.Write([]byte(`{"ErrorKey":"unauthorized","Message":"The request authorization header is not valid.\r"}`))
			return
		}

		if accessKey != s.apiKey {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			//nolint:errcheck // Error responses are best effort
			w.Write([]byte(`{"ErrorKey":"unauthorized","Message":"The request authorization header is not valid.\r"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
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

	now := MockBunnyTime{Time: time.Now().UTC()}
	zone := &Zone{
		ID:                       id,
		Domain:                   domain,
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
		// Set defaults for new fields added to match real API behavior
		if records[i].EnviromentalVariables == nil {
			records[i].EnviromentalVariables = []interface{}{}
		}
		// AutoSslIssuance always defaults to true in real API
		records[i].AutoSslIssuance = true
	}
	zone.Records = records
	zone.DateModified = MockBunnyTime{Time: time.Now().UTC()}

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

// Handler returns the HTTP handler for use with a standalone server.
// This allows running mockbunny outside of httptest.
func (s *Server) Handler() chi.Router {
	return s.router
}
