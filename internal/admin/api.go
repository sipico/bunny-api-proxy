package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sipico/bunny-api-proxy/internal/auth"
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
		WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Invalid JSON")
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
		WriteErrorWithHint(w, http.StatusBadRequest, ErrCodeInvalidRequest,
			"Invalid log level", "Must be one of: debug, info, warn, error")
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
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to list tokens")
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
		WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Invalid JSON")
		return
	}

	if req.Name == "" || req.Token == "" {
		WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Name and token required")
		return
	}

	id, err := h.storage.CreateAdminToken(r.Context(), req.Name, req.Token)
	if err != nil {
		h.logger.Error("failed to create token", "error", err)
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to create token")
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
		WriteErrorWithHint(w, http.StatusBadRequest, ErrCodeInvalidRequest,
			"Invalid token ID", "Token ID must be a number")
		return
	}

	err = h.storage.DeleteAdminToken(r.Context(), id)
	if err != nil {
		if err == storage.ErrNotFound {
			WriteError(w, http.StatusNotFound, ErrCodeNotFound, "Token not found")
			return
		}
		h.logger.Error("failed to delete token", "error", err)
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to delete token")
		return
	}

	h.logger.Info("admin token deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
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

// =============================================================================
// Unified Token API Handlers (Issue 147)
// =============================================================================

// WhoamiResponse represents the current token's identity.
type WhoamiResponse struct {
	TokenID     int64                 `json:"token_id,omitempty"`
	Name        string                `json:"name,omitempty"`
	IsAdmin     bool                  `json:"is_admin"`
	IsMasterKey bool                  `json:"is_master_key"`
	Permissions []*storage.Permission `json:"permissions,omitempty"`
}

// HandleWhoami returns the current token's identity and permissions.
// GET /api/whoami
func (h *Handler) HandleWhoami(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	resp := WhoamiResponse{
		IsMasterKey: auth.IsMasterKeyFromContext(ctx),
		IsAdmin:     auth.IsAdminFromContext(ctx),
	}

	// Get token from context if available
	token := auth.TokenFromContext(ctx)
	if token != nil {
		resp.TokenID = token.ID
		resp.Name = token.Name
		resp.IsAdmin = token.IsAdmin

		// Get permissions for non-admin tokens
		if !token.IsAdmin {
			perms, err := h.storage.GetPermissionsForToken(ctx, token.ID)
			if err != nil {
				h.logger.Error("failed to get permissions", "error", err)
				WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to get permissions")
				return
			}
			resp.Permissions = perms
		}
	}

	w.Header().Set("Content-Type", "application/json")
	encErr := json.NewEncoder(w).Encode(resp)
	if encErr != nil {
		_ = encErr
	}
}

// UnifiedTokenResponse represents a token in API responses (never includes key).
type UnifiedTokenResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	IsAdmin   bool   `json:"is_admin"`
	CreatedAt string `json:"created_at"`
}

// HandleListUnifiedTokens returns all tokens (unified model).
// GET /api/tokens
func (h *Handler) HandleListUnifiedTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.storage.ListTokens(r.Context())
	if err != nil {
		h.logger.Error("failed to list tokens", "error", err)
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to list tokens")
		return
	}

	response := make([]UnifiedTokenResponse, len(tokens))
	for i, t := range tokens {
		response[i] = UnifiedTokenResponse{
			ID:        t.ID,
			Name:      t.Name,
			IsAdmin:   t.IsAdmin,
			CreatedAt: t.CreatedAt.Format(time.RFC3339),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	encErr := json.NewEncoder(w).Encode(response)
	if encErr != nil {
		_ = encErr
	}
}

// CreateUnifiedTokenRequest is the request body for POST /api/tokens (unified model).
type CreateUnifiedTokenRequest struct {
	Name        string   `json:"name"`
	IsAdmin     bool     `json:"is_admin"`
	Zones       []int64  `json:"zones,omitempty"`
	Actions     []string `json:"actions,omitempty"`
	RecordTypes []string `json:"record_types,omitempty"`
}

// CreateUnifiedTokenResponse includes the token (shown only once).
type CreateUnifiedTokenResponse struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Token   string `json:"token"` // Plain token, shown once
	IsAdmin bool   `json:"is_admin"`
}

// HandleCreateUnifiedToken creates a new token (admin or scoped).
// POST /api/tokens
// Body: {"name": "...", "is_admin": true/false, "zones": [...], "actions": [...], "record_types": [...]}
//
// Bootstrap logic:
//   - During UNCONFIGURED state: only allow creating admin tokens (is_admin: true)
//   - Master key is allowed during bootstrap
//   - After admin exists: master key is locked out, only admin tokens can manage
func (h *Handler) HandleCreateUnifiedToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateUnifiedTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Invalid JSON in request body")
		return
	}

	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Token name is required")
		return
	}

	// Check bootstrap state
	isMasterKey := auth.IsMasterKeyFromContext(ctx)
	if h.bootstrap != nil {
		state, err := h.bootstrap.GetState(ctx)
		if err != nil {
			h.logger.Error("failed to get bootstrap state", "error", err)
			WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to check bootstrap state")
			return
		}

		// During UNCONFIGURED state
		if state == auth.StateUnconfigured {
			// Only admin tokens can be created during bootstrap
			if !req.IsAdmin {
				WriteErrorWithHint(w, http.StatusUnprocessableEntity, ErrCodeNoAdminTokenExists,
					"No admin token exists. Create an admin token first.",
					"During bootstrap, you must create an admin token (is_admin: true) first.")
				return
			}
		} else {
			// System is CONFIGURED - master key is locked out
			if isMasterKey {
				WriteErrorWithHint(w, http.StatusForbidden, ErrCodeMasterKeyLocked,
					"Master API key cannot access admin endpoints. Use an admin token.",
					"Create requests using an admin token instead of the master API key.")
				return
			}
			// Only admin tokens can manage tokens
			if !auth.IsAdminFromContext(ctx) {
				WriteError(w, http.StatusForbidden, ErrCodeAdminRequired, "Admin token required to manage tokens")
				return
			}
		}
	}

	// Validate permissions for scoped tokens
	if !req.IsAdmin {
		if len(req.Zones) == 0 {
			WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Scoped tokens require at least one zone")
			return
		}
		if len(req.Actions) == 0 {
			WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Scoped tokens require at least one action")
			return
		}
		if len(req.RecordTypes) == 0 {
			WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Scoped tokens require at least one record type")
			return
		}
	}

	// Generate secure token
	plainToken := generateRandomKey(64) // 64 hex chars = 32 bytes = 256 bits

	// Hash the token for storage
	hash := sha256.Sum256([]byte(plainToken))
	keyHash := hex.EncodeToString(hash[:])

	// Create the token
	token, err := h.storage.CreateToken(ctx, req.Name, req.IsAdmin, keyHash)
	if err != nil {
		if err == storage.ErrDuplicate {
			WriteErrorWithHint(w, http.StatusConflict, "duplicate_token",
				"A token with this hash already exists", "Try creating the token again.")
			return
		}
		h.logger.Error("failed to create token", "error", err)
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to create token")
		return
	}

	// Add permissions for scoped tokens
	if !req.IsAdmin && len(req.Zones) > 0 {
		for _, zoneID := range req.Zones {
			perm := &storage.Permission{
				ZoneID:         zoneID,
				AllowedActions: req.Actions,
				RecordTypes:    req.RecordTypes,
			}
			if _, err := h.storage.AddPermissionForToken(ctx, token.ID, perm); err != nil {
				h.logger.Error("failed to add permission", "error", err, "token_id", token.ID, "zone_id", zoneID)
				// Clean up the token we just created
				if delErr := h.storage.DeleteToken(ctx, token.ID); delErr != nil {
					h.logger.Error("failed to clean up token after permission error", "error", delErr)
				}
				WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to add permissions")
				return
			}
		}
	}

	h.logger.Info("token created", "id", token.ID, "name", req.Name, "is_admin", req.IsAdmin)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	encErr := json.NewEncoder(w).Encode(CreateUnifiedTokenResponse{
		ID:      token.ID,
		Name:    req.Name,
		Token:   plainToken, // Return plaintext once
		IsAdmin: req.IsAdmin,
	})
	if encErr != nil {
		_ = encErr
	}
}

// UnifiedTokenDetailResponse includes token details and permissions.
type UnifiedTokenDetailResponse struct {
	ID          int64                 `json:"id"`
	Name        string                `json:"name"`
	IsAdmin     bool                  `json:"is_admin"`
	CreatedAt   string                `json:"created_at"`
	Permissions []*storage.Permission `json:"permissions,omitempty"`
}

// HandleGetUnifiedToken returns token details.
// GET /api/tokens/{id}
func (h *Handler) HandleGetUnifiedToken(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		WriteErrorWithHint(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Invalid token ID", "Token ID must be a number.")
		return
	}

	ctx := r.Context()

	token, err := h.storage.GetTokenByID(ctx, id)
	if err != nil {
		if err == storage.ErrNotFound {
			WriteError(w, http.StatusNotFound, ErrCodeNotFound, "Token not found")
			return
		}
		h.logger.Error("failed to get token", "error", err, "id", id)
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to get token")
		return
	}

	resp := UnifiedTokenDetailResponse{
		ID:        token.ID,
		Name:      token.Name,
		IsAdmin:   token.IsAdmin,
		CreatedAt: token.CreatedAt.Format(time.RFC3339),
	}

	// Get permissions for scoped tokens
	if !token.IsAdmin {
		perms, err := h.storage.GetPermissionsForToken(ctx, token.ID)
		if err != nil {
			h.logger.Error("failed to get permissions", "error", err, "token_id", id)
			WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to get permissions")
			return
		}
		resp.Permissions = perms
	}

	w.Header().Set("Content-Type", "application/json")
	encErr := json.NewEncoder(w).Encode(resp)
	if encErr != nil {
		_ = encErr
	}
}

// HandleDeleteUnifiedToken deletes a token with last-admin protection.
// DELETE /api/tokens/{id}
func (h *Handler) HandleDeleteUnifiedToken(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		WriteErrorWithHint(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Invalid token ID", "Token ID must be a number.")
		return
	}

	ctx := r.Context()

	// Get the token to check if it's an admin
	token, err := h.storage.GetTokenByID(ctx, id)
	if err != nil {
		if err == storage.ErrNotFound {
			WriteError(w, http.StatusNotFound, ErrCodeNotFound, "Token not found")
			return
		}
		h.logger.Error("failed to get token", "error", err, "id", id)
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to get token")
		return
	}

	// Last-admin protection: check if this is the last admin token
	if token.IsAdmin {
		count, err := h.storage.CountAdminTokens(ctx)
		if err != nil {
			h.logger.Error("failed to count admin tokens", "error", err)
			WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to check admin count")
			return
		}
		if count <= 1 {
			WriteErrorWithHint(w, http.StatusConflict, ErrCodeCannotDeleteLastAdmin,
				"Cannot delete the last admin token. Create another admin first.",
				"Create a new admin token before deleting this one.")
			return
		}
	}

	err = h.storage.DeleteToken(ctx, id)
	if err != nil {
		if err == storage.ErrNotFound {
			WriteError(w, http.StatusNotFound, ErrCodeNotFound, "Token not found")
			return
		}
		h.logger.Error("failed to delete token", "error", err, "id", id)
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to delete token")
		return
	}

	h.logger.Info("token deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// AddPermissionRequest is the request body for POST /api/tokens/{id}/permissions.
type AddPermissionRequest struct {
	ZoneID         int64    `json:"zone_id"`
	AllowedActions []string `json:"allowed_actions"`
	RecordTypes    []string `json:"record_types"`
}

// PermissionResponse represents a permission in API responses.
type PermissionResponse struct {
	ID             int64    `json:"id"`
	ZoneID         int64    `json:"zone_id"`
	AllowedActions []string `json:"allowed_actions"`
	RecordTypes    []string `json:"record_types"`
}

// HandleAddTokenPermission adds a permission to a token.
// POST /api/tokens/{id}/permissions
// Body: {"zone_id": 123, "allowed_actions": [...], "record_types": [...]}
func (h *Handler) HandleAddTokenPermission(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	tokenID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		WriteErrorWithHint(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Invalid token ID", "Token ID must be a number.")
		return
	}

	ctx := r.Context()

	// Verify token exists
	token, err := h.storage.GetTokenByID(ctx, tokenID)
	if err != nil {
		if err == storage.ErrNotFound {
			WriteError(w, http.StatusNotFound, ErrCodeNotFound, "Token not found")
			return
		}
		h.logger.Error("failed to get token", "error", err, "id", tokenID)
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to get token")
		return
	}

	// Admin tokens don't use permissions
	if token.IsAdmin {
		WriteErrorWithHint(w, http.StatusBadRequest, ErrCodeInvalidRequest,
			"Admin tokens do not use zone permissions",
			"Admin tokens have full access. Permissions are only for scoped tokens.")
		return
	}

	var req AddPermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Invalid JSON in request body")
		return
	}

	// Validate required fields
	if req.ZoneID <= 0 {
		WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Zone ID must be greater than 0")
		return
	}
	if len(req.AllowedActions) == 0 {
		WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "At least one action is required")
		return
	}
	if len(req.RecordTypes) == 0 {
		WriteError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "At least one record type is required")
		return
	}

	perm := &storage.Permission{
		ZoneID:         req.ZoneID,
		AllowedActions: req.AllowedActions,
		RecordTypes:    req.RecordTypes,
	}

	createdPerm, err := h.storage.AddPermissionForToken(ctx, tokenID, perm)
	if err != nil {
		h.logger.Error("failed to add permission", "error", err, "token_id", tokenID)
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to add permission")
		return
	}

	h.logger.Info("permission added", "token_id", tokenID, "permission_id", createdPerm.ID, "zone_id", req.ZoneID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	encErr := json.NewEncoder(w).Encode(PermissionResponse{
		ID:             createdPerm.ID,
		ZoneID:         createdPerm.ZoneID,
		AllowedActions: createdPerm.AllowedActions,
		RecordTypes:    createdPerm.RecordTypes,
	})
	if encErr != nil {
		_ = encErr
	}
}

// HandleDeleteTokenPermission removes a permission from a token.
// DELETE /api/tokens/{id}/permissions/{pid}
func (h *Handler) HandleDeleteTokenPermission(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	tokenID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		WriteErrorWithHint(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Invalid token ID", "Token ID must be a number.")
		return
	}

	pidStr := chi.URLParam(r, "pid")
	permID, err := strconv.ParseInt(pidStr, 10, 64)
	if err != nil {
		WriteErrorWithHint(w, http.StatusBadRequest, ErrCodeInvalidRequest,
			"Invalid permission ID", "Permission ID must be a number.")
		return
	}

	ctx := r.Context()

	// Verify token exists
	_, err = h.storage.GetTokenByID(ctx, tokenID)
	if err != nil {
		if err == storage.ErrNotFound {
			WriteError(w, http.StatusNotFound, ErrCodeNotFound, "Token not found")
			return
		}
		h.logger.Error("failed to get token", "error", err, "id", tokenID)
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to get token")
		return
	}

	// Delete the permission
	err = h.storage.RemovePermission(ctx, permID)
	if err != nil {
		if err == storage.ErrNotFound {
			WriteError(w, http.StatusNotFound, ErrCodeNotFound, "Permission not found")
			return
		}
		h.logger.Error("failed to delete permission", "error", err, "permission_id", permID)
		WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to delete permission")
		return
	}

	h.logger.Info("permission deleted", "token_id", tokenID, "permission_id", permID)
	w.WriteHeader(http.StatusNoContent)
}
