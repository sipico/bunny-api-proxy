package config

import (
	"os"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	t.Run("with all environment variables set", func(t *testing.T) {
		// Set required variables
		t.Setenv("ADMIN_PASSWORD", "test-password")
		t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012") // 32 chars
		t.Setenv("LOG_LEVEL", "debug")
		t.Setenv("HTTP_PORT", "9000")
		t.Setenv("DATA_PATH", "/custom/path.db")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}

		if cfg.AdminPassword != "test-password" {
			t.Errorf("AdminPassword = %q, want %q", cfg.AdminPassword, "test-password")
		}
		if string(cfg.EncryptionKey) != "12345678901234567890123456789012" {
			t.Errorf("EncryptionKey = %q, want %q", string(cfg.EncryptionKey), "12345678901234567890123456789012")
		}
		if cfg.LogLevel != "debug" {
			t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
		}
		if cfg.HTTPPort != "9000" {
			t.Errorf("HTTPPort = %q, want %q", cfg.HTTPPort, "9000")
		}
		if cfg.DataPath != "/custom/path.db" {
			t.Errorf("DataPath = %q, want %q", cfg.DataPath, "/custom/path.db")
		}
	})
}

func TestLoad_DefaultValues(t *testing.T) {
	t.Run("with only required variables set", func(t *testing.T) {
		t.Setenv("ADMIN_PASSWORD", "test-password")
		t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
		// Clear optional variables
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("HTTP_PORT")
		os.Unsetenv("DATA_PATH")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}

		if cfg.LogLevel != "info" {
			t.Errorf("LogLevel = %q, want %q (default)", cfg.LogLevel, "info")
		}
		if cfg.HTTPPort != "8080" {
			t.Errorf("HTTPPort = %q, want %q (default)", cfg.HTTPPort, "8080")
		}
		if cfg.DataPath != "/data/proxy.db" {
			t.Errorf("DataPath = %q, want %q (default)", cfg.DataPath, "/data/proxy.db")
		}
	})
}

func TestLoad_MissingAdminPassword(t *testing.T) {
	t.Run("when ADMIN_PASSWORD is not set", func(t *testing.T) {
		os.Unsetenv("ADMIN_PASSWORD")
		t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")

		cfg, err := Load()
		if cfg != nil {
			t.Errorf("Load() cfg = %v, want nil", cfg)
		}
		if err != ErrMissingAdminPassword {
			t.Errorf("Load() error = %v, want %v", err, ErrMissingAdminPassword)
		}
	})

	t.Run("when ADMIN_PASSWORD is empty", func(t *testing.T) {
		t.Setenv("ADMIN_PASSWORD", "")
		t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")

		cfg, err := Load()
		if cfg != nil {
			t.Errorf("Load() cfg = %v, want nil", cfg)
		}
		if err != ErrMissingAdminPassword {
			t.Errorf("Load() error = %v, want %v", err, ErrMissingAdminPassword)
		}
	})
}

func TestLoad_MissingEncryptionKey(t *testing.T) {
	t.Run("when ENCRYPTION_KEY is not set", func(t *testing.T) {
		t.Setenv("ADMIN_PASSWORD", "test-password")
		os.Unsetenv("ENCRYPTION_KEY")

		cfg, err := Load()
		if cfg != nil {
			t.Errorf("Load() cfg = %v, want nil", cfg)
		}
		if err != ErrMissingEncryptionKey {
			t.Errorf("Load() error = %v, want %v", err, ErrMissingEncryptionKey)
		}
	})

	t.Run("when ENCRYPTION_KEY is empty", func(t *testing.T) {
		t.Setenv("ADMIN_PASSWORD", "test-password")
		t.Setenv("ENCRYPTION_KEY", "")

		cfg, err := Load()
		if cfg != nil {
			t.Errorf("Load() cfg = %v, want nil", cfg)
		}
		if err != ErrMissingEncryptionKey {
			t.Errorf("Load() error = %v, want %v", err, ErrMissingEncryptionKey)
		}
	})
}

func TestLoad_InvalidEncryptionKeyLength(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		length int
	}{
		{"too short", "12345678901234567890123456789", 31},
		{"too long", "123456789012345678901234567890133", 33},
		{"way too short", "short", 5},
		{"way too long", "this is a much longer string that exceeds the 32 character requirement", 70},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ADMIN_PASSWORD", "test-password")
			t.Setenv("ENCRYPTION_KEY", tt.key)

			cfg, err := Load()
			if cfg != nil {
				t.Errorf("Load() cfg = %v, want nil", cfg)
			}
			if err != ErrInvalidEncryptionKey {
				t.Errorf("Load() error = %v, want %v", err, ErrInvalidEncryptionKey)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Run("valid encryption key length", func(t *testing.T) {
		cfg := &Config{
			AdminPassword: "test-password",
			EncryptionKey: []byte("12345678901234567890123456789012"),
		}

		err := cfg.Validate()
		if err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})

	t.Run("invalid encryption key length", func(t *testing.T) {
		cfg := &Config{
			AdminPassword: "test-password",
			EncryptionKey: []byte("short"),
		}

		err := cfg.Validate()
		if err != ErrInvalidEncryptionKey {
			t.Errorf("Validate() error = %v, want %v", err, ErrInvalidEncryptionKey)
		}
	})
}

func TestLoad_LogLevelValidation(t *testing.T) {
	t.Run("sets log level when not set", func(t *testing.T) {
		t.Setenv("ADMIN_PASSWORD", "test-password")
		t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
		os.Unsetenv("LOG_LEVEL")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.LogLevel != "info" {
			t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
		}
	})

	t.Run("respects set log level", func(t *testing.T) {
		t.Setenv("ADMIN_PASSWORD", "test-password")
		t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
		t.Setenv("LOG_LEVEL", "error")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.LogLevel != "error" {
			t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "error")
		}
	})
}

func TestLoad_HTTPPortValidation(t *testing.T) {
	t.Run("sets default http port when not set", func(t *testing.T) {
		t.Setenv("ADMIN_PASSWORD", "test-password")
		t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
		os.Unsetenv("HTTP_PORT")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.HTTPPort != "8080" {
			t.Errorf("HTTPPort = %q, want %q", cfg.HTTPPort, "8080")
		}
	})

	t.Run("respects set http port", func(t *testing.T) {
		t.Setenv("ADMIN_PASSWORD", "test-password")
		t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
		t.Setenv("HTTP_PORT", "3000")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.HTTPPort != "3000" {
			t.Errorf("HTTPPort = %q, want %q", cfg.HTTPPort, "3000")
		}
	})
}

func TestLoad_DataPathValidation(t *testing.T) {
	t.Run("sets default data path when not set", func(t *testing.T) {
		t.Setenv("ADMIN_PASSWORD", "test-password")
		t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
		os.Unsetenv("DATA_PATH")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.DataPath != "/data/proxy.db" {
			t.Errorf("DataPath = %q, want %q", cfg.DataPath, "/data/proxy.db")
		}
	})

	t.Run("respects set data path", func(t *testing.T) {
		t.Setenv("ADMIN_PASSWORD", "test-password")
		t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
		t.Setenv("DATA_PATH", "/var/lib/proxy.db")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.DataPath != "/var/lib/proxy.db" {
			t.Errorf("DataPath = %q, want %q", cfg.DataPath, "/var/lib/proxy.db")
		}
	})
}

func TestLoad_BunnyAPIURL_Set(t *testing.T) {
	t.Run("when BUNNY_API_URL is set", func(t *testing.T) {
		t.Setenv("ADMIN_PASSWORD", "test-password")
		t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
		t.Setenv("BUNNY_API_URL", "http://mockbunny:8081")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.BunnyAPIURL != "http://mockbunny:8081" {
			t.Errorf("BunnyAPIURL = %q, want %q", cfg.BunnyAPIURL, "http://mockbunny:8081")
		}
	})
}

func TestLoad_BunnyAPIURL_NotSet(t *testing.T) {
	t.Run("when BUNNY_API_URL is not set", func(t *testing.T) {
		t.Setenv("ADMIN_PASSWORD", "test-password")
		t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
		os.Unsetenv("BUNNY_API_URL")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.BunnyAPIURL != "" {
			t.Errorf("BunnyAPIURL = %q, want empty string", cfg.BunnyAPIURL)
		}
	})
}
