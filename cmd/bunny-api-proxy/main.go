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
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sipico/bunny-api-proxy/internal/admin"
	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/bunny"
	"github.com/sipico/bunny-api-proxy/internal/config"
	"github.com/sipico/bunny-api-proxy/internal/metrics"
	"github.com/sipico/bunny-api-proxy/internal/proxy"
	"github.com/sipico/bunny-api-proxy/internal/storage"
)

const version = "2026.01.2"
const serverShutdownTimeout = 30 * time.Second

func main() {
	// Handle health check subcommand for distroless container health checks
	if len(os.Args) > 1 && os.Args[1] == "health" { // coverage-ignore: health subcommand only used in container HEALTHCHECK
		os.Exit(runHealthCheck()) // coverage-ignore: health subcommand only used in container HEALTHCHECK
	}

	if err := run(); err != nil { // coverage-ignore: run() errors only occur in production failures
		log.Fatalf("Server failed: %v", err) // coverage-ignore: run() errors only occur in production failures
	}
}

// runHealthCheck performs an HTTP health check against the local server.
// Returns 0 on success, 1 on failure. Used by container HEALTHCHECK.
func runHealthCheck() int {
	return doHealthCheck("http://localhost:8080/health")
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

// serverComponents holds all initialized server components for testing
type serverComponents struct {
	logger           *slog.Logger
	logLevel         *slog.LevelVar
	store            storage.Storage
	validator        *auth.Validator
	bunnyClient      *bunny.Client
	bootstrapService *auth.BootstrapService
	proxyRouter      http.Handler
	adminRouter      http.Handler
	mainRouter       *chi.Mux
	metricsRouter    http.Handler
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
		"listenAddr", cfg.ListenAddr,
	)

	// Initialize Prometheus metrics
	// Note: In tests, this may be called multiple times. We ignore duplicate registration errors
	// since the metrics will already be registered in the global registry.
	if err := metrics.Init(prometheus.DefaultRegisterer); err != nil {
		// Check if this is a duplicate registration error (expected in tests)
		if !strings.Contains(err.Error(), "duplicate metrics collector registration") { // coverage-ignore: metrics init failures only occur on malformed metric definitions
			return nil, fmt.Errorf("metrics initialization failed: %w", err) // coverage-ignore: metrics init failures only occur on malformed metric definitions
		}
		// Log that metrics were already initialized
		logger.Debug("Metrics already initialized")
	}

	// 3. Initialize storage
	store, err := storage.New(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("storage initialization failed: %w", err)
	}

	// 4. Create auth validator
	validator := auth.NewValidator(store)

	// 5. Create bunny client with real API key and logging transport
	var bunnyOpts []bunny.Option
	if cfg.BunnyAPIURL != "" {
		bunnyOpts = append(bunnyOpts, bunny.WithBaseURL(cfg.BunnyAPIURL))
	}

	// Wire up LoggingTransport to log bunny.net API calls
	loggingTransport := &bunny.LoggingTransport{
		Transport: http.DefaultTransport,
		Logger:    logger,
		Prefix:    "BUNNY",
	}
	httpClient := &http.Client{Transport: loggingTransport}
	bunnyOpts = append(bunnyOpts, bunny.WithHTTPClient(httpClient))

	bunnyClient := bunny.NewClient(cfg.BunnyAPIKey, bunnyOpts...)

	// 6. Create bootstrap service for managing master key and bootstrap state
	bootstrapService := auth.NewBootstrapService(store, cfg.BunnyAPIKey)

	// 7. Create proxy handler and router
	proxyHandler := proxy.NewHandler(bunnyClient, logger)
	proxyAuthenticator := auth.NewAuthenticator(store, bootstrapService)
	// Chain authentication and permission checking middleware
	proxyAuthChain := func(next http.Handler) http.Handler {
		return proxyAuthenticator.Authenticate(proxyAuthenticator.CheckPermissions(next))
	}
	proxyRouter := proxy.NewRouter(proxyHandler, proxyAuthChain, logger)

	// 8. Create admin handler and router
	adminHandler := admin.NewHandler(store, logLevel, logger)
	adminHandler.SetBootstrapService(bootstrapService)
	adminRouter := adminHandler.NewRouter()

	// 9. Assemble main router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(metrics.Middleware)

	r.Get("/health", healthHandler)
	r.Get("/ready", readyHandler(store))
	r.Mount("/admin", adminRouter)
	r.Mount("/", proxyRouter)

	// 10. Assemble metrics router on a separate internal listener
	metricsRouter := chi.NewRouter()
	metricsRouter.Handle("/metrics", metrics.Handler())

	return &serverComponents{
		logger:           logger,
		logLevel:         logLevel,
		store:            store,
		validator:        validator,
		bunnyClient:      bunnyClient,
		bootstrapService: bootstrapService,
		proxyRouter:      proxyRouter,
		adminRouter:      adminRouter,
		mainRouter:       r,
		metricsRouter:    metricsRouter,
	}, nil
}

// createServer creates and returns an HTTP server with the given configuration
func createServer(cfg *config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// createMetricsServer creates and returns an HTTP server for metrics on the internal listener
func createMetricsServer(cfg *config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         cfg.MetricsListenAddr,
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

// startServersAndWaitForShutdown starts the main and metrics servers, handles graceful shutdown for both
func startServersAndWaitForShutdown(logger *slog.Logger, mainServer *http.Server, metricsServer *http.Server, metricsErrors chan error) error {
	logger.Info("Server listening", "address", mainServer.Addr)

	// Channel to signal server shutdown
	mainErrors := make(chan error, 1)

	// Start main server in a goroutine
	go func() {
		mainErrors <- mainServer.ListenAndServe()
	}()

	// Wait for shutdown signal or server error
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-mainErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err) // coverage-ignore: server startup errors rarely occur in tests
		}
	case err := <-metricsErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("metrics server error: %w", err) // coverage-ignore: metrics server startup errors rarely occur in tests
		}
	case sig := <-sigChan:
		logger.Info("Received signal, shutting down", "signal", sig.String())

		// Graceful shutdown with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		defer cancel()

		// Shut down both servers
		mainErr := mainServer.Shutdown(shutdownCtx)
		metricsErr := metricsServer.Shutdown(shutdownCtx)

		if mainErr != nil {
			return fmt.Errorf("main server shutdown failed: %w", mainErr) // coverage-ignore: shutdown errors during signal handling rarely occur in tests
		}
		if metricsErr != nil {
			return fmt.Errorf("metrics server shutdown failed: %w", metricsErr) // coverage-ignore: metrics shutdown errors during signal handling rarely occur in tests
		}

		logger.Info("Server shut down gracefully")
	}

	return nil
}

// run initializes all components and starts the server with graceful shutdown.
func run() error {
	// 1. Load and validate configuration
	cfg, err := config.Load() // coverage-ignore: config.Load only fails if os.Getenv fails (internal error)
	if err != nil {           // coverage-ignore: config.Load only fails if os.Getenv fails (internal error)
		return fmt.Errorf("config load failed: %w", err) // coverage-ignore: config.Load only fails if os.Getenv fails (internal error)
	}

	if err := cfg.Validate(); err != nil { // coverage-ignore: config.Validate only fails if BUNNY_API_KEY missing (caught by CI)
		return fmt.Errorf("config validation failed: %w", err) // coverage-ignore: config.Validate only fails if BUNNY_API_KEY missing (caught by CI)
	}

	// Initialize all components
	components, err := initializeComponents(cfg)
	if err != nil {
		return err
	}

	// Ensure storage is closed when we exit
	defer func() {
		if closeErr := components.store.Close(); closeErr != nil { // coverage-ignore: storage.Close only fails on I/O errors
			components.logger.Error("storage close failed", "error", closeErr) // coverage-ignore: storage.Close only fails on I/O errors
		}
	}()

	// Create servers
	mainServer := createServer(cfg, components.mainRouter)
	metricsServer := createMetricsServer(cfg, components.metricsRouter)

	// Start metrics server in a goroutine
	metricsErrors := make(chan error, 1)
	go func() {
		components.logger.Info("Metrics listener starting", "address", metricsServer.Addr)
		metricsErrors <- metricsServer.ListenAndServe()
	}()

	// Start main server and handle graceful shutdown for both
	return startServersAndWaitForShutdown(components.logger, mainServer, metricsServer, metricsErrors)
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

		// Test database connectivity by listing tokens
		_, err := store.ListTokens(ctx)
		if err != nil {
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
