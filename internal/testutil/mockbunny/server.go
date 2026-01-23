package mockbunny

import (
	"net/http"
	"net/http/httptest"

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

	// Placeholder routes (handlers added in subsequent tasks)
	r.Get("/dnszone", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	})
	r.Get("/dnszone/{id}", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	})
	r.Put("/dnszone/{zoneId}/records", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	})
	r.Delete("/dnszone/{zoneId}/records/{id}", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	})

	ts := httptest.NewServer(r)

	return &Server{
		Server: ts,
		state:  state,
		router: r,
	}
}

// URL returns the base URL of the mock server.
func (s *Server) URL() string {
	return s.Server.URL
}
