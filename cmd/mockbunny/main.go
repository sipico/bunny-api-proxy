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

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	server := mockbunny.New()

	// Create a standalone HTTP server (not httptest)
	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: server.Handler(),
	}

	// Graceful shutdown
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

	log.Printf("mockbunny listening on :%s", port)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}

	<-done
	log.Println("mockbunny stopped")
}
