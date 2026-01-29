package config

import (
	"os"
	"testing"
)

func TestLoad_DefaultValues(t *testing.T) {
	t.Run("with no environment variables set", func(t *testing.T) {
		// Clear all config-related environment variables
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("LISTEN_ADDR")
		os.Unsetenv("DATABASE_PATH")
		os.Unsetenv("BUNNY_API_URL")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}

		if cfg.LogLevel != "info" {
			t.Errorf("LogLevel = %q, want %q (default)", cfg.LogLevel, "info")
		}
		if cfg.ListenAddr != ":8080" {
			t.Errorf("ListenAddr = %q, want %q (default)", cfg.ListenAddr, ":8080")
		}
		if cfg.DatabasePath != "/data/proxy.db" {
			t.Errorf("DatabasePath = %q, want %q (default)", cfg.DatabasePath, "/data/proxy.db")
		}
		if cfg.BunnyAPIURL != "" {
			t.Errorf("BunnyAPIURL = %q, want empty string (default)", cfg.BunnyAPIURL)
		}
	})
}

func TestLoad_CustomValues(t *testing.T) {
	t.Run("with all environment variables set", func(t *testing.T) {
		t.Setenv("LOG_LEVEL", "debug")
		t.Setenv("LISTEN_ADDR", ":9000")
		t.Setenv("DATABASE_PATH", "/custom/path.db")
		t.Setenv("BUNNY_API_URL", "http://mockbunny:8081")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}

		if cfg.LogLevel != "debug" {
			t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
		}
		if cfg.ListenAddr != ":9000" {
			t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9000")
		}
		if cfg.DatabasePath != "/custom/path.db" {
			t.Errorf("DatabasePath = %q, want %q", cfg.DatabasePath, "/custom/path.db")
		}
		if cfg.BunnyAPIURL != "http://mockbunny:8081" {
			t.Errorf("BunnyAPIURL = %q, want %q", cfg.BunnyAPIURL, "http://mockbunny:8081")
		}
	})
}

func TestLoad_LogLevel(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{"not set uses default", "", "info"},
		{"debug", "debug", "debug"},
		{"info", "info", "info"},
		{"warn", "warn", "warn"},
		{"error", "error", "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue == "" {
				os.Unsetenv("LOG_LEVEL")
			} else {
				t.Setenv("LOG_LEVEL", tt.envValue)
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.LogLevel != tt.want {
				t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, tt.want)
			}
		})
	}
}

func TestLoad_ListenAddr(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{"not set uses default", "", ":8080"},
		{"custom port", ":3000", ":3000"},
		{"with host", "0.0.0.0:8080", "0.0.0.0:8080"},
		{"localhost only", "127.0.0.1:8080", "127.0.0.1:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue == "" {
				os.Unsetenv("LISTEN_ADDR")
			} else {
				t.Setenv("LISTEN_ADDR", tt.envValue)
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.ListenAddr != tt.want {
				t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, tt.want)
			}
		})
	}
}

func TestLoad_DatabasePath(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{"not set uses default", "", "/data/proxy.db"},
		{"custom path", "/var/lib/bunny/proxy.db", "/var/lib/bunny/proxy.db"},
		{"memory database", ":memory:", ":memory:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue == "" {
				os.Unsetenv("DATABASE_PATH")
			} else {
				t.Setenv("DATABASE_PATH", tt.envValue)
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.DatabasePath != tt.want {
				t.Errorf("DatabasePath = %q, want %q", cfg.DatabasePath, tt.want)
			}
		})
	}
}

func TestLoad_BunnyAPIURL(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{"not set returns empty", "", ""},
		{"custom URL", "http://localhost:8081", "http://localhost:8081"},
		{"production URL", "https://api.bunny.net", "https://api.bunny.net"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue == "" {
				os.Unsetenv("BUNNY_API_URL")
			} else {
				t.Setenv("BUNNY_API_URL", tt.envValue)
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.BunnyAPIURL != tt.want {
				t.Errorf("BunnyAPIURL = %q, want %q", cfg.BunnyAPIURL, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Run("always returns nil for valid config", func(t *testing.T) {
		cfg := &Config{
			LogLevel:     "info",
			ListenAddr:   ":8080",
			DatabasePath: "/data/proxy.db",
		}

		err := cfg.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})
}
