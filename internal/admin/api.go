package admin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// SetLogLevelRequest is the request body for POST /api/loglevel
type SetLogLevelRequest struct {
	Level string `json:"level"`
}

// HandleSetLogLevel changes runtime log level
// POST /api/loglevel
// Body: {"level": "debug|info|warn|error"}
func (h *Handler) HandleSetLogLevel(w http.ResponseWriter, r *http.Request) {
	var req SetLogLevelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var level slog.Level
	switch req.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		http.Error(w, "Invalid level (must be: debug, info, warn, error)", http.StatusBadRequest)
		return
	}

	h.logLevel.Set(level)
	h.logger.Info("log level changed", "new_level", req.Level)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(map[string]string{
		"level": req.Level,
	})
	if err != nil {
		// Encoding errors are not critical for loglevel response
		_ = err
	}
}

// TokenResponse represents an admin token in API responses
type TokenResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// HandleListTokens returns all admin tokens
// GET /api/tokens
func (h *Handler) HandleListTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.storage.ListAdminTokens(r.Context())
	if err != nil {
		h.logger.Error("failed to list tokens", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	response := make([]TokenResponse, len(tokens))
	for i, t := range tokens {
		response[i] = TokenResponse{
			ID:        t.ID,
			Name:      t.Name,
			CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	encErr := json.NewEncoder(w).Encode(response)
	if encErr != nil {
		// Encoding errors are not critical for list response
		_ = encErr
	}
}

// CreateTokenRequest is the request body for POST /api/tokens
type CreateTokenRequest struct {
	Name  string `json:"name"`
	Token string `json:"token"`
}

// CreateTokenResponse includes the token (shown only once)
type CreateTokenResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Token string `json:"token"` // Plain token, shown once
}

// HandleCreateToken creates a new admin token
// POST /api/tokens
// Body: {"name": "...", "token": "..."}
func (h *Handler) HandleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req CreateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Token == "" {
		http.Error(w, "Name and token required", http.StatusBadRequest)
		return
	}

	id, err := h.storage.CreateAdminToken(r.Context(), req.Name, req.Token)
	if err != nil {
		h.logger.Error("failed to create token", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("admin token created", "id", id, "name", req.Name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	encErr := json.NewEncoder(w).Encode(CreateTokenResponse{
		ID:    id,
		Name:  req.Name,
		Token: req.Token, // Return plaintext once
	})
	if encErr != nil {
		// Encoding errors are not critical for create response
		_ = encErr
	}
}

// HandleDeleteToken deletes an admin token
// DELETE /api/tokens/{id}
func (h *Handler) HandleDeleteToken(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid token ID", http.StatusBadRequest)
		return
	}

	err = h.storage.DeleteAdminToken(r.Context(), id)
	if err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "Token not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to delete token", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("admin token deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// SetMasterKeyRequest is the request body for PUT /api/master-key
type SetMasterKeyRequest struct {
	APIKey string `json:"api_key"`
}

// HandleSetMasterKeyAPI sets the master API key via API
// PUT /api/master-key
// Body: {"api_key": "..."}
func (h *Handler) HandleSetMasterKeyAPI(w http.ResponseWriter, r *http.Request) {
	var req SetMasterKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.APIKey == "" {
		http.Error(w, "API key required", http.StatusBadRequest)
		return
	}

	// Check if master key is already set
	existingKey, err := h.storage.GetMasterAPIKey(r.Context())
	if err == nil && existingKey != "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		encErr := json.NewEncoder(w).Encode(map[string]string{
			"error": "master key already set",
		})
		if encErr != nil {
			_ = encErr
		}
		return
	}

	if err := h.storage.SetMasterAPIKey(r.Context(), req.APIKey); err != nil {
		h.logger.Error("failed to set master key", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("master key set via API")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	encErr := json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
	if encErr != nil {
		_ = encErr
	}
}

// CreateKeyRequest is the request body for POST /api/keys
type CreateKeyRequest struct {
	Name        string   `json:"name"`
	Zones       []int64  `json:"zones"`
	Actions     []string `json:"actions"`
	RecordTypes []string `json:"record_types"`
}

// CreateKeyResponse is the response for POST /api/keys
type CreateKeyResponse struct {
	ID  int64  `json:"id"`
	Key string `json:"key"`
}

// HandleCreateKeyAPI creates a new scoped key with permissions via API
// POST /api/keys
// Body: {"name": "...", "zones": [123], "actions": ["list_zones"], "record_types": ["TXT"]}
func (h *Handler) HandleCreateKeyAPI(w http.ResponseWriter, r *http.Request) {
	var req CreateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name required", http.StatusBadRequest)
		return
	}

	if len(req.Zones) == 0 {
		http.Error(w, "At least one zone required", http.StatusBadRequest)
		return
	}

	if len(req.Actions) == 0 {
		http.Error(w, "At least one action required", http.StatusBadRequest)
		return
	}

	if len(req.RecordTypes) == 0 {
		http.Error(w, "At least one record type required", http.StatusBadRequest)
		return
	}

	// Generate a random API key for the scoped key
	apiKey := generateRandomKey(32)

	// Create the scoped key
	keyID, err := h.storage.CreateScopedKey(r.Context(), req.Name, apiKey)
	if err != nil {
		h.logger.Error("failed to create scoped key", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Add permissions for each zone
	for _, zoneID := range req.Zones {
		perm := &storage.Permission{
			ZoneID:         zoneID,
			AllowedActions: req.Actions,
			RecordTypes:    req.RecordTypes,
		}
		if _, err := h.storage.AddPermission(r.Context(), keyID, perm); err != nil {
			h.logger.Error("failed to add permission", "error", err, "key_id", keyID, "zone_id", zoneID)
			// Clean up the key we just created
			if delErr := h.storage.DeleteScopedKey(r.Context(), keyID); delErr != nil {
				h.logger.Error("failed to clean up key after permission error", "error", delErr)
			}
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
	}

	h.logger.Info("scoped key created via API", "id", keyID, "name", req.Name, "zones", req.Zones)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	encErr := json.NewEncoder(w).Encode(CreateKeyResponse{
		ID:  keyID,
		Key: apiKey,
	})
	if encErr != nil {
		_ = encErr
	}
}

// generateRandomKey generates a random hex string of the given length
func generateRandomKey(length int) string {
	b := make([]byte, length/2)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a simple pseudo-random key if crypto/rand fails
		return "fallback-key-" + strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}
