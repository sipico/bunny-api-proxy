package logging

import (
	"encoding/json"
	"testing"
)

func TestMaskHeader(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		value    string
		expected string
	}{
		// Password/secret headers (full redaction)
		{"password header", "Password", "secret123", "[REDACTED]"},
		{"authorization with password", "X-Password", "mypass", "[REDACTED]"},
		{"secret header", "X-Secret", "topsecret", "[REDACTED]"},
		{"private key", "Private-Key", "key123", "[REDACTED]"},

		// Token/API key headers (last 4 chars)
		{"authorization bearer", "Authorization", "Bearer token-value-1234", "****1234"},
		{"accesskey header", "AccessKey", "api-key-12345678", "****5678"},
		{"x-api-key header", "X-Api-Key", "mykey123", "****y123"},
		{"short token", "AccessKey", "abc", "****"},

		// Case insensitive
		{"mixed case auth", "AUTHORIZATION", "secret-abcd", "****abcd"},
		{"lowercase auth", "authorization", "mysecret9999", "****9999"},
		{"lowercase accesskey", "accesskey", "token1234567890", "****7890"},
		{"mixed case password", "password", "pass123", "[REDACTED]"},

		// Non-sensitive headers (unchanged)
		{"content-type", "Content-Type", "application/json", "application/json"},
		{"user-agent", "User-Agent", "test-client/1.0", "test-client/1.0"},
		{"custom header", "X-Custom", "value", "value"},
		{"accept", "Accept", "application/json", "application/json"},

		// x-access-key tests
		{"x-access-key header", "X-Access-Key", "mykey123456", "****3456"},
		{"lowercase x-access-key", "x-access-key", "key1234567890", "****7890"},

		// Edge cases
		{"empty value", "Authorization", "", "****"},
		{"four char value", "Authorization", "1234", "****1234"},
		{"five char value", "Authorization", "12345", "****2345"},
		{"single char value", "Authorization", "a", "****"},

		// Case sensitivity in keywords
		{"PASSWORD case variations", "PASSWORD", "secret", "[REDACTED]"},
		{"Secret case variations", "secret", "value", "[REDACTED]"},
		{"PRIVATE-KEY", "PRIVATE-KEY", "key", "[REDACTED]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := MaskHeader(tt.header, tt.value)
			if result != tt.expected {
				t.Errorf("MaskHeader(%q, %q) = %q, want %q",
					tt.header, tt.value, result, tt.expected)
			}
		})
	}
}

func TestMaskJSONBody(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		allowlist []string
		wantJSON  string
	}{
		{
			name:      "nil allowlist returns unchanged",
			body:      `{"user":"alice","password":"secret"}`,
			allowlist: nil,
			wantJSON:  `{"user":"alice","password":"secret"}`,
		},
		{
			name:      "empty allowlist redacts all",
			body:      `{"user":"alice","token":"secret"}`,
			allowlist: []string{},
			wantJSON:  `{"user":"[REDACTED]","token":"[REDACTED]"}`,
		},
		{
			name:      "allowlist preserves specified fields",
			body:      `{"id":123,"name":"alice","token":"secret"}`,
			allowlist: []string{"id", "name"},
			wantJSON:  `{"id":123,"name":"alice","token":"[REDACTED]"}`,
		},
		{
			name:      "nested objects",
			body:      `{"user":"alice","auth":{"token":"secret","exp":1234}}`,
			allowlist: []string{"user", "exp"},
			wantJSON:  `{"user":"alice","auth":{"token":"[REDACTED]","exp":1234}}`,
		},
		{
			name:      "invalid json returns unchanged",
			body:      `not valid json`,
			allowlist: []string{"field"},
			wantJSON:  `not valid json`,
		},
		{
			name:      "empty body",
			body:      ``,
			allowlist: []string{"field"},
			wantJSON:  ``,
		},
		{
			name:      "array of objects",
			body:      `[{"id":1,"secret":"a"},{"id":2,"secret":"b"}]`,
			allowlist: []string{"id"},
			wantJSON:  `[{"id":1,"secret":"[REDACTED]"},{"id":2,"secret":"[REDACTED]"}]`,
		},
		{
			name:      "nested arrays",
			body:      `{"items":[{"id":1,"token":"secret"}],"name":"test"}`,
			allowlist: []string{"items", "id", "name"},
			wantJSON:  `{"items":[{"id":1,"token":"[REDACTED]"}],"name":"test"}`,
		},
		{
			name:      "primitive values in allowlist",
			body:      `{"id":123,"count":456,"active":true}`,
			allowlist: []string{"id", "active"},
			wantJSON:  `{"id":123,"active":true,"count":"[REDACTED]"}`,
		},
		{
			name:      "null values",
			body:      `{"id":1,"data":null}`,
			allowlist: []string{"id", "data"},
			wantJSON:  `{"id":1,"data":null}`,
		},
		{
			name:      "deeply nested objects",
			body:      `{"level1":{"level2":{"level3":{"secret":"value","id":1}}}}`,
			allowlist: []string{"level1", "level2", "level3", "id"},
			wantJSON:  `{"level1":{"level2":{"level3":{"secret":"[REDACTED]","id":1}}}}`,
		},
		{
			name:      "mixed types in array",
			body:      `[1,"string",{"key":"value"},true]`,
			allowlist: []string{"key"},
			wantJSON:  `[1,"string",{"key":"value"},true]`,
		},
		{
			name:      "unicode characters",
			body:      `{"name":"José","token":"secret123"}`,
			allowlist: []string{"name"},
			wantJSON:  `{"name":"José","token":"[REDACTED]"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := MaskJSONBody([]byte(tt.body), tt.allowlist)

			// Compare as JSON to ignore whitespace differences
			if !jsonEqual(result, []byte(tt.wantJSON)) {
				t.Errorf("MaskJSONBody(...) = %s, want %s", string(result), tt.wantJSON)
			}
		})
	}
}

func TestFormatBinaryData(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{"empty data", []byte{}, "[BINARY: 0 bytes]"},
		{"small data", []byte{1, 2, 3}, "[BINARY: 3 bytes]"},
		{"1KB data", make([]byte, 1024), "[BINARY: 1024 bytes]"},
		{"nil data", nil, "[BINARY: 0 bytes]"},
		{"single byte", []byte{0xFF}, "[BINARY: 1 bytes]"},
		{"100 bytes", make([]byte, 100), "[BINARY: 100 bytes]"},
		{"1MB data", make([]byte, 1024*1024), "[BINARY: 1048576 bytes]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := FormatBinaryData(tt.data)
			if result != tt.expected {
				t.Errorf("FormatBinaryData(...) = %q, want %q", result, tt.expected)
			}
		})
	}
}

// jsonEqual compares two JSON byte slices for semantic equality
func jsonEqual(a, b []byte) bool {
	var aVal, bVal interface{}

	// Handle empty slices
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) == 0 || len(b) == 0 {
		return len(a) == len(b)
	}

	if err := json.Unmarshal(a, &aVal); err != nil {
		// If we can't unmarshal a, compare as strings
		return string(a) == string(b)
	}
	if err := json.Unmarshal(b, &bVal); err != nil {
		return false
	}

	// Compare the unmarshaled values
	aJSON, _ := json.Marshal(aVal)
	bJSON, _ := json.Marshal(bVal)
	return string(aJSON) == string(bJSON)
}

// TestMaskHeaderPasswordVariants tests various password-like header names
func TestMaskHeaderPasswordVariants(t *testing.T) {
	passwordVariants := []string{
		"Password",
		"password",
		"PASSWORD",
		"X-Password",
		"X-PASSWORD",
		"Custom-Password",
		"Api-Password",
	}

	for _, headerName := range passwordVariants {
		t.Run("password_variant_"+headerName, func(t *testing.T) {
			result := MaskHeader(headerName, "mypassword")
			if result != "[REDACTED]" {
				t.Errorf("MaskHeader(%q, \"mypassword\") = %q, want \"[REDACTED]\"", headerName, result)
			}
		})
	}
}

// TestMaskHeaderSecretVariants tests various secret-like header names
func TestMaskHeaderSecretVariants(t *testing.T) {
	secretVariants := []string{
		"Secret",
		"secret",
		"SECRET",
		"X-Secret",
		"X-SECRET",
		"Api-Secret",
		"Client-Secret",
	}

	for _, headerName := range secretVariants {
		t.Run("secret_variant_"+headerName, func(t *testing.T) {
			result := MaskHeader(headerName, "mysecret")
			if result != "[REDACTED]" {
				t.Errorf("MaskHeader(%q, \"mysecret\") = %q, want \"[REDACTED]\"", headerName, result)
			}
		})
	}
}

// TestMaskHeaderPrivateKeyVariants tests various private-key-like header names
func TestMaskHeaderPrivateKeyVariants(t *testing.T) {
	privateKeyVariants := []string{
		"Private-Key",
		"private-key",
		"PRIVATE-KEY",
		"X-Private-Key",
		"Api-Private-Key",
	}

	for _, headerName := range privateKeyVariants {
		t.Run("private_key_variant_"+headerName, func(t *testing.T) {
			result := MaskHeader(headerName, "myprivatekey")
			if result != "[REDACTED]" {
				t.Errorf("MaskHeader(%q, \"myprivatekey\") = %q, want \"[REDACTED]\"", headerName, result)
			}
		})
	}
}

// TestMaskHeaderTokenVariants tests various token-like header names
func TestMaskHeaderTokenVariants(t *testing.T) {
	tests := []struct {
		headerName string
		value      string
	}{
		{"Authorization", "Bearer abcdef123456"},
		{"authorization", "Bearer token123456"},
		{"AUTHORIZATION", "Bearer token987654"},
		{"AccessKey", "key123456789abc"},
		{"accesskey", "key987654321def"},
		{"ACCESSKEY", "key555666777888"},
		{"X-Api-Key", "key123456789xyz"},
		{"x-api-key", "key987654321uvw"},
		{"X-API-KEY", "key555666777rst"},
		{"X-Access-Key", "key123456789pqr"},
		{"x-access-key", "key987654321mnl"},
		{"X-ACCESS-KEY", "key555666777ijk"},
	}

	for _, tt := range tests {
		t.Run("token_variant_"+tt.headerName, func(t *testing.T) {
			result := MaskHeader(tt.headerName, tt.value)
			if len(tt.value) < 4 {
				if result != "****" {
					t.Errorf("MaskHeader(%q, %q) = %q, want \"****\"", tt.headerName, tt.value, result)
				}
			} else {
				expected := "****" + tt.value[len(tt.value)-4:]
				if result != expected {
					t.Errorf("MaskHeader(%q, %q) = %q, want %q", tt.headerName, tt.value, result, expected)
				}
			}
		})
	}
}

// TestMaskJSONBodyComplexNesting tests complex nested structures
func TestMaskJSONBodyComplexNesting(t *testing.T) {
	body := []byte(`{"user":{"id":1,"name":"Alice","credentials":{"token":"secret123","password":"pass456"}},"metadata":{"created":"2024-01-01","updated":"2024-01-02"}}`)

	allowlist := []string{"user", "id", "name", "metadata", "created", "updated"}

	result := MaskJSONBody(body, allowlist)

	// Unmarshal to verify structure
	var resultMap map[string]interface{}
	if err := json.Unmarshal(result, &resultMap); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Check that allowed fields are present and non-redacted
	user := resultMap["user"].(map[string]interface{})
	if user["name"] != "Alice" {
		t.Errorf("Expected name to be 'Alice', got %v", user["name"])
	}
	if user["id"] != float64(1) {
		t.Errorf("Expected id to be 1, got %v", user["id"])
	}

	// Check that credentials object is recursively processed (not completely redacted)
	creds := user["credentials"].(map[string]interface{})
	if creds["token"] != "[REDACTED]" {
		t.Errorf("Expected token to be redacted, got %v", creds["token"])
	}
	if creds["password"] != "[REDACTED]" {
		t.Errorf("Expected password to be redacted, got %v", creds["password"])
	}

	// Check metadata
	metadata := resultMap["metadata"].(map[string]interface{})
	if metadata["created"] != "2024-01-01" {
		t.Errorf("Expected created to be preserved, got %v", metadata["created"])
	}
}

// TestMaskJSONBodyEmptyAllowlist verifies all fields are redacted with empty allowlist
func TestMaskJSONBodyEmptyAllowlist(t *testing.T) {
	body := []byte(`{"id":1,"name":"test","secret":"value"}`)
	result := MaskJSONBody(body, []string{})

	var resultMap map[string]interface{}
	if err := json.Unmarshal(result, &resultMap); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	for key, val := range resultMap {
		if val != "[REDACTED]" {
			t.Errorf("Expected field %q to be redacted, got %v", key, val)
		}
	}
}
