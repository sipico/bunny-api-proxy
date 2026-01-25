// Package config provides configuration loading and validation from environment variables.
package config

import (
	"errors"
	"os"
)

// Config holds all application configuration
type Config struct {
	AdminPassword  string
	EncryptionKey  []byte // Must be 32 bytes
	LogLevel       string // debug, info, warn, error
	HTTPPort       string
	DataPath       string
}

// ErrMissingAdminPassword indicates the ADMIN_PASSWORD environment variable is required.
var ErrMissingAdminPassword = errors.New("ADMIN_PASSWORD is required")

// ErrMissingEncryptionKey indicates the ENCRYPTION_KEY environment variable is required.
var ErrMissingEncryptionKey = errors.New("ENCRYPTION_KEY is required")

// ErrInvalidEncryptionKey indicates the ENCRYPTION_KEY must be exactly 32 characters.
var ErrInvalidEncryptionKey = errors.New("ENCRYPTION_KEY must be exactly 32 characters")

// Load parses configuration from environment variables.
// Returns error if required variables are missing or invalid.
func Load() (*Config, error) {
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	encryptionKeyStr := os.Getenv("ENCRYPTION_KEY")
	logLevel := os.Getenv("LOG_LEVEL")
	httpPort := os.Getenv("HTTP_PORT")
	dataPath := os.Getenv("DATA_PATH")

	// Validate required fields
	if adminPassword == "" {
		return nil, ErrMissingAdminPassword
	}

	if encryptionKeyStr == "" {
		return nil, ErrMissingEncryptionKey
	}

	// Set defaults for optional fields
	if logLevel == "" {
		logLevel = "info"
	}

	if httpPort == "" {
		httpPort = "8080"
	}

	if dataPath == "" {
		dataPath = "/data/proxy.db"
	}

	cfg := &Config{
		AdminPassword:  adminPassword,
		EncryptionKey:  []byte(encryptionKeyStr),
		LogLevel:       logLevel,
		HTTPPort:       httpPort,
		DataPath:       dataPath,
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks all configuration constraints
func (c *Config) Validate() error {
	if len(c.EncryptionKey) != 32 {
		return ErrInvalidEncryptionKey
	}
	return nil
}
