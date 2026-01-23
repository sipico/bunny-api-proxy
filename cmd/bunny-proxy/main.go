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

func main() {
	// Get configuration from environment variables per ARCHITECTURE.md
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	// Initialize Chi router
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

	addr := fmt.Sprintf(":%s", httpPort)
	log.Printf("Bunny API Proxy v%s starting on %s", version, addr)

	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// healthHandler returns OK if the process is alive
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok"}`)
}

// readyHandler returns OK if the service is ready to serve requests (DB connected)
func readyHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Check database connection when storage layer is implemented
	// For now, always return OK since there's no DB yet
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok"}`)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"message":"Bunny API Proxy","version":"%s"}`, version)
}
