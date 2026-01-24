package admin

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

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
