// Package main implements a standalone mock bunny.net server for E2E testing.
package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sipico/bunny-api-proxy/internal/testutil/mockbunny"
)

// getPort returns the port from the PORT environment variable or the default.
func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	return port
}

// getPortAddr formats the port into a server address.
func getPortAddr(port string) string {
	return ":" + port
}

// createServer creates a new mockbunny server instance.
func createServer() *mockbunny.Server {
	return mockbunny.New()
}

// createHTTPServer creates an http.Server with the given port and handler.
func createHTTPServer(port string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:    getPortAddr(port),
		Handler: handler,
	}
}

// setupShutdownHandler sets up graceful shutdown handling.
func setupShutdownHandler(httpServer *http.Server) <-chan bool {
	done := make(chan bool)
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Println("Shutting down mockbunny server...")
		//nolint:errcheck
		httpServer.Close()
		close(done)
	}()
	return done
}

// runHealthCheck performs an HTTP health check against the local server.
// Returns 0 on success, 1 on failure. Used by container HEALTHCHECK.
func runHealthCheck() int {
	port := getPort()
	return doHealthCheck("http://localhost:" + port + "/admin/state")
}

// doHealthCheck performs the actual health check HTTP request.
// Extracted for testability.
func doHealthCheck(url string) int {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 1
	}
	//nolint:errcheck // Response body close errors are unrecoverable in health check
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func main() {
	// Handle health check subcommand for distroless container health checks
	if len(os.Args) > 1 && os.Args[1] == "health" {
		os.Exit(runHealthCheck())
	}

	port := getPort()
	server := createServer()

	// Create a standalone HTTP server (not httptest)
	httpServer := createHTTPServer(port, server.Handler())

	// Graceful shutdown
	done := setupShutdownHandler(httpServer)

	log.Printf("mockbunny listening on :%s", port)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}

	<-done
	log.Println("mockbunny stopped")
}
