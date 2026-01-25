// Package main provides the entry point for the Bunny API Proxy server.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	// TODO: Add SQLite driver when storage layer is implemented
	// _ "modernc.org/sqlite"
)

const version = "0.1.0"

// run initializes and returns the server configuration
// This is separated from main() to enable testing
func run() (string, chi.Router) {
	httpPort := getHTTPPort()
	r := setupRouter()
	addr := fmt.Sprintf(":%s", httpPort)
	log.Printf("Bunny API Proxy v%s starting on %s", version, addr)
	return addr, r
}

func main() {
	addr, r := run()
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// getHTTPPort returns the HTTP port from environment or default
func getHTTPPort() string {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		return "8080"
	}
	return port
}

// setupRouter configures and returns the HTTP router with all routes and middleware
func setupRouter() chi.Router {
	r := chi.NewRouter()

	// Add Chi middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))

	// Health check endpoints per ARCHITECTURE.md
	r.Get("/health", healthHandler)
	r.Get("/ready", readyHandler)

	// Root endpoint
	r.Get("/", rootHandler)

	return r
}

// healthHandler returns OK if the process is alive
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck // Response write errors are unrecoverable
	fmt.Fprintf(w, `{"status":"ok"}`)
}

// readyHandler returns OK if the service is ready to serve requests (DB connected)
func readyHandler(w http.ResponseWriter, _ *http.Request) {
	// TODO: Check database connection when storage layer is implemented
	// For now, always return OK since there's no DB yet
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck // Response write errors are unrecoverable
	fmt.Fprintf(w, `{"status":"ok"}`)
}

func rootHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck // Response write errors are unrecoverable
	fmt.Fprintf(w, `{"message":"Bunny API Proxy","version":"%s"}`, version)
}
