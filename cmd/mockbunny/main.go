// Package main implements a standalone mock bunny.net server for E2E testing.
package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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

func main() {
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
