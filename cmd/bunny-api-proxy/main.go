// Package main provides the entry point for the Bunny API Proxy server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sipico/bunny-api-proxy/internal/admin"
	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/bunny"
	"github.com/sipico/bunny-api-proxy/internal/config"
	"github.com/sipico/bunny-api-proxy/internal/proxy"
	"github.com/sipico/bunny-api-proxy/internal/storage"
)

const version = "0.1.0"
const serverShutdownTimeout = 30 * time.Second

func main() {
	if err := run(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// serverComponents holds all initialized server components for testing
type serverComponents struct {
	logger      *slog.Logger
	logLevel    *slog.LevelVar
	store       storage.Storage
	validator   *auth.Validator
	bunnyClient *bunny.StorageClient
	proxyRouter http.Handler
	adminRouter http.Handler
	mainRouter  *chi.Mux
}

// initializeComponents sets up all server components with proper error handling
func initializeComponents(cfg *config.Config) (*serverComponents, error) {
	// 2. Configure structured logging with dynamic level
	logLevel := new(slog.LevelVar)
	if err := logLevel.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", cfg.LogLevel, err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	logger.Info("Server starting",
		"version", version,
		"logLevel", cfg.LogLevel,
		"httpPort", cfg.HTTPPort,
	)

	// 3. Initialize storage
	store, err := storage.New(cfg.DataPath, cfg.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("storage initialization failed: %w", err)
	}

	// 4. Create auth validator
	validator := auth.NewValidator(store)

	// 5. Create bunny client (storage-backed)
	var bunnyOpts []interface{}
	if cfg.BunnyAPIURL != "" {
		bunnyOpts = append(bunnyOpts, bunny.WithStorageClientBaseURL(cfg.BunnyAPIURL))
	}
	bunnyClient := bunny.NewStorageClient(store, bunnyOpts...)

	// 6. Create proxy handler and router
	proxyHandler := proxy.NewHandler(bunnyClient, logger)
	proxyRouter := proxy.NewRouter(proxyHandler, auth.Middleware(validator))

	// 7. Create admin handler and router
	sessionStore := admin.NewSessionStore(24 * time.Hour)
	adminHandler := admin.NewHandler(store, sessionStore, logLevel, logger)
	adminRouter := adminHandler.NewRouter()

	// 8. Assemble main router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", healthHandler)
	r.Get("/ready", readyHandler(store))
	r.Mount("/api", proxyRouter)
	r.Mount("/admin", adminRouter)

	return &serverComponents{
		logger:      logger,
		logLevel:    logLevel,
		store:       store,
		validator:   validator,
		bunnyClient: bunnyClient,
		proxyRouter: proxyRouter,
		adminRouter: adminRouter,
		mainRouter:  r,
	}, nil
}

// createServer creates and returns an HTTP server with the given configuration
func createServer(cfg *config.Config, handler http.Handler) *http.Server {
	addr := fmt.Sprintf(":%s", cfg.HTTPPort)
	return &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// startServerAndWaitForShutdown starts the server and waits for shutdown signal or error
func startServerAndWaitForShutdown(logger *slog.Logger, server *http.Server) error {
	logger.Info("Server listening", "address", server.Addr)

	// Channel to signal server shutdown
	serverErrors := make(chan error, 1)

	// Start server in a goroutine
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	// Wait for shutdown signal or server error
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err)
		}
	case sig := <-sigChan:
		logger.Info("Received signal, shutting down", "signal", sig.String())

		// Graceful shutdown with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}

		logger.Info("Server shut down gracefully")
	}

	return nil
}

// run initializes all components and starts the server with graceful shutdown.
func run() error {
	// 1. Load and validate configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config load failed: %w", err)
	}

	// Initialize all components
	components, err := initializeComponents(cfg)
	if err != nil {
		return err
	}

	// Ensure storage is closed when we exit
	defer func() {
		if closeErr := components.store.Close(); closeErr != nil {
			components.logger.Error("storage close failed", "error", closeErr)
		}
	}()

	// Create and start server with graceful shutdown
	server := createServer(cfg, components.mainRouter)
	return startServerAndWaitForShutdown(components.logger, server)
}

// healthHandler returns OK if the process is alive
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck // Response write errors are unrecoverable
	fmt.Fprint(w, `{"status":"ok"}`)
}

// readyHandler returns OK if the service is ready to serve requests (DB connected)
func readyHandler(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check database connectivity by attempting a simple operation
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		// Try to get master API key - just tests connectivity, doesn't require a key to exist
		_, err := store.GetMasterAPIKey(ctx)
		if err != nil && err != storage.ErrNotFound {
			// Database error
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			//nolint:errcheck // Response write errors are unrecoverable
			fmt.Fprint(w, `{"status":"not_ready","error":"database unavailable"}`)
			return
		}

		// Database is accessible
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck // Response write errors are unrecoverable
		fmt.Fprint(w, `{"status":"ok"}`)
	}
}
