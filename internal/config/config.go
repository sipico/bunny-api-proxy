// Package config provides configuration loading and validation from environment variables.
package config

import (
	"fmt"
	"os"
)

// Config holds all application configuration for API-only mode.
type Config struct {
	LogLevel     string // debug, info, warn, error
	ListenAddr   string // Server listen address (e.g., ":8080")
	DatabasePath string // SQLite database path
	BunnyAPIURL  string // Optional: Base URL for bunny.net API (empty = use default)
	BunnyAPIKey  string // Required: bunny.net API key for master authentication
}

// Load parses configuration from environment variables.
// All configuration options have sensible defaults for ease of deployment.
func Load() (*Config, error) {
	logLevel := os.Getenv("LOG_LEVEL")
	listenAddr := os.Getenv("LISTEN_ADDR")
	databasePath := os.Getenv("DATABASE_PATH")
	bunnyAPIURL := os.Getenv("BUNNY_API_URL")
	bunnyAPIKey := os.Getenv("BUNNY_API_KEY")

	// Set defaults for optional fields
	if logLevel == "" {
		logLevel = "info"
	}

	if listenAddr == "" {
		listenAddr = ":8080"
	}

	if databasePath == "" {
		databasePath = "/data/proxy.db"
	}

	cfg := &Config{
		LogLevel:     logLevel,
		ListenAddr:   listenAddr,
		DatabasePath: databasePath,
		BunnyAPIURL:  bunnyAPIURL,
		BunnyAPIKey:  bunnyAPIKey,
	}

	return cfg, nil
}

// Validate checks all configuration constraints.
func (c *Config) Validate() error {
	if c.BunnyAPIKey == "" {
		return fmt.Errorf("BUNNY_API_KEY environment variable is required")
	}
	return nil
}
