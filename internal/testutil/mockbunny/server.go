// Package mockbunny provides a mock bunny.net API server for testing.
package mockbunny

import (
	"net/http"
	"net/http/httptest"
)

// Server is a mock bunny.net API server for testing.
type Server struct {
	*httptest.Server
}

// New creates a new mock bunny.net API server.
func New() *Server {
	mux := http.NewServeMux()

	// Add mock endpoints here as needed
	mux.HandleFunc("/dnszone", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Items":[]}`))
	})

	srv := httptest.NewServer(mux)
	return &Server{Server: srv}
}
