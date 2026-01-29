package admin

import (
	"encoding/json"
	"net/http"
)

// Standard error codes for API responses.
const (
	// ErrCodeInvalidRequest indicates a malformed request body.
	ErrCodeInvalidRequest = "invalid_request"

	// ErrCodeInvalidCredentials indicates invalid or missing API key.
	ErrCodeInvalidCredentials = "invalid_credentials"

	// ErrCodeAdminRequired indicates an admin token is required.
	ErrCodeAdminRequired = "admin_required"

	// ErrCodeMasterKeyLocked indicates master key is not allowed after bootstrap.
	ErrCodeMasterKeyLocked = "master_key_locked"

	// ErrCodeNotFound indicates a resource was not found.
	ErrCodeNotFound = "not_found"

	// ErrCodeCannotDeleteLastAdmin indicates last admin protection.
	ErrCodeCannotDeleteLastAdmin = "cannot_delete_last_admin"

	// ErrCodeNoAdminTokenExists indicates first token must be admin.
	ErrCodeNoAdminTokenExists = "no_admin_token_exists"

	// ErrCodeInternalError indicates a server error.
	ErrCodeInternalError = "internal_error"
)

// APIError is the standard error response format for JSON APIs.
type APIError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// WriteError writes a JSON error response with the given status code, error code, and message.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteErrorWithHint(w, status, code, message, "")
}

// WriteErrorWithHint writes a JSON error response with an optional hint for resolving the error.
func WriteErrorWithHint(w http.ResponseWriter, status int, code, message, hint string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := APIError{
		Error:   code,
		Message: message,
		Hint:    hint,
	}
	// Encoding errors are not critical since headers are already sent
	encErr := json.NewEncoder(w).Encode(resp)
	if encErr != nil {
		// Response already started, nothing we can do
		_ = encErr
	}
}
