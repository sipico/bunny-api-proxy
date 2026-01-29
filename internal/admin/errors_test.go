package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		code       string
		message    string
		wantStatus int
		wantCode   string
		wantMsg    string
		wantHint   string
	}{
		{
			name:       "bad request error",
			status:     http.StatusBadRequest,
			code:       ErrCodeInvalidRequest,
			message:    "Invalid JSON",
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeInvalidRequest,
			wantMsg:    "Invalid JSON",
			wantHint:   "",
		},
		{
			name:       "not found error",
			status:     http.StatusNotFound,
			code:       ErrCodeNotFound,
			message:    "Token not found",
			wantStatus: http.StatusNotFound,
			wantCode:   ErrCodeNotFound,
			wantMsg:    "Token not found",
			wantHint:   "",
		},
		{
			name:       "internal error",
			status:     http.StatusInternalServerError,
			code:       ErrCodeInternalError,
			message:    "Failed to process request",
			wantStatus: http.StatusInternalServerError,
			wantCode:   ErrCodeInternalError,
			wantMsg:    "Failed to process request",
			wantHint:   "",
		},
		{
			name:       "unauthorized error",
			status:     http.StatusUnauthorized,
			code:       ErrCodeInvalidCredentials,
			message:    "Invalid API key",
			wantStatus: http.StatusUnauthorized,
			wantCode:   ErrCodeInvalidCredentials,
			wantMsg:    "Invalid API key",
			wantHint:   "",
		},
		{
			name:       "forbidden admin required",
			status:     http.StatusForbidden,
			code:       ErrCodeAdminRequired,
			message:    "Admin token required",
			wantStatus: http.StatusForbidden,
			wantCode:   ErrCodeAdminRequired,
			wantMsg:    "Admin token required",
			wantHint:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteError(rec, tt.status, tt.code, tt.message)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			contentType := rec.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
			}

			var resp APIError
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Error != tt.wantCode {
				t.Errorf("error code = %q, want %q", resp.Error, tt.wantCode)
			}
			if resp.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", resp.Message, tt.wantMsg)
			}
			if resp.Hint != tt.wantHint {
				t.Errorf("hint = %q, want %q", resp.Hint, tt.wantHint)
			}
		})
	}
}

func TestWriteErrorWithHint(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		code       string
		message    string
		hint       string
		wantStatus int
		wantCode   string
		wantMsg    string
		wantHint   string
	}{
		{
			name:       "conflict with hint",
			status:     http.StatusConflict,
			code:       ErrCodeCannotDeleteLastAdmin,
			message:    "Cannot delete the last admin token",
			hint:       "Create another admin token first.",
			wantStatus: http.StatusConflict,
			wantCode:   ErrCodeCannotDeleteLastAdmin,
			wantMsg:    "Cannot delete the last admin token",
			wantHint:   "Create another admin token first.",
		},
		{
			name:       "unprocessable entity with hint",
			status:     http.StatusUnprocessableEntity,
			code:       ErrCodeNoAdminTokenExists,
			message:    "No admin token exists",
			hint:       "Create an admin token (is_admin: true) first.",
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   ErrCodeNoAdminTokenExists,
			wantMsg:    "No admin token exists",
			wantHint:   "Create an admin token (is_admin: true) first.",
		},
		{
			name:       "forbidden master key locked with hint",
			status:     http.StatusForbidden,
			code:       ErrCodeMasterKeyLocked,
			message:    "Master API key cannot access admin endpoints",
			hint:       "Use an admin token instead.",
			wantStatus: http.StatusForbidden,
			wantCode:   ErrCodeMasterKeyLocked,
			wantMsg:    "Master API key cannot access admin endpoints",
			wantHint:   "Use an admin token instead.",
		},
		{
			name:       "empty hint is omitted",
			status:     http.StatusBadRequest,
			code:       ErrCodeInvalidRequest,
			message:    "Invalid request",
			hint:       "",
			wantStatus: http.StatusBadRequest,
			wantCode:   ErrCodeInvalidRequest,
			wantMsg:    "Invalid request",
			wantHint:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteErrorWithHint(rec, tt.status, tt.code, tt.message, tt.hint)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			contentType := rec.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
			}

			var resp APIError
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Error != tt.wantCode {
				t.Errorf("error code = %q, want %q", resp.Error, tt.wantCode)
			}
			if resp.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", resp.Message, tt.wantMsg)
			}
			if resp.Hint != tt.wantHint {
				t.Errorf("hint = %q, want %q", resp.Hint, tt.wantHint)
			}
		})
	}
}

func TestWriteErrorWithHintOmitsEmptyHint(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteErrorWithHint(rec, http.StatusBadRequest, ErrCodeInvalidRequest, "Test message", "")

	// Check that "hint" key is not present in the response when empty
	body := rec.Body.String()
	var rawResp map[string]interface{}
	if err := json.Unmarshal([]byte(body), &rawResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// With omitempty, the hint field should not be present when empty
	if _, exists := rawResp["hint"]; exists {
		t.Error("hint field should be omitted when empty")
	}
}

func TestErrorCodeConstants(t *testing.T) {
	// Verify error code constants are defined correctly
	codes := map[string]string{
		"ErrCodeInvalidRequest":        ErrCodeInvalidRequest,
		"ErrCodeInvalidCredentials":    ErrCodeInvalidCredentials,
		"ErrCodeAdminRequired":         ErrCodeAdminRequired,
		"ErrCodeMasterKeyLocked":       ErrCodeMasterKeyLocked,
		"ErrCodeNotFound":              ErrCodeNotFound,
		"ErrCodeCannotDeleteLastAdmin": ErrCodeCannotDeleteLastAdmin,
		"ErrCodeNoAdminTokenExists":    ErrCodeNoAdminTokenExists,
		"ErrCodeInternalError":         ErrCodeInternalError,
	}

	expectedValues := map[string]string{
		"ErrCodeInvalidRequest":        "invalid_request",
		"ErrCodeInvalidCredentials":    "invalid_credentials",
		"ErrCodeAdminRequired":         "admin_required",
		"ErrCodeMasterKeyLocked":       "master_key_locked",
		"ErrCodeNotFound":              "not_found",
		"ErrCodeCannotDeleteLastAdmin": "cannot_delete_last_admin",
		"ErrCodeNoAdminTokenExists":    "no_admin_token_exists",
		"ErrCodeInternalError":         "internal_error",
	}

	for name, code := range codes {
		expected := expectedValues[name]
		if code != expected {
			t.Errorf("%s = %q, want %q", name, code, expected)
		}
	}
}
