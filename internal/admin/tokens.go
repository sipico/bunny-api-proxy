package admin

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// HandleListAdminTokensPage shows the list of admin tokens
// GET /admin/tokens
func (h *Handler) HandleListAdminTokensPage(w http.ResponseWriter, r *http.Request) {
	if h.templates == nil {
		http.Error(w, "Templates not loaded", http.StatusInternalServerError)
		return
	}

	tokens, err := h.storage.ListAdminTokens(r.Context())
	if err != nil {
		h.logger.Error("failed to list tokens", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Title":  "Admin Tokens",
		"Tokens": tokens,
	}

	if err := h.templates.ExecuteTemplate(w, "layout.html", data); err != nil {
		h.logger.Error("template error", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

// HandleNewTokenForm shows the form to create a new admin token
// GET /admin/tokens/new
func (h *Handler) HandleNewTokenForm(w http.ResponseWriter, r *http.Request) {
	if h.templates == nil {
		http.Error(w, "Templates not loaded", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Title": "Create Admin Token",
	}

	if err := h.templates.ExecuteTemplate(w, "layout.html", data); err != nil {
		h.logger.Error("template error", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

// generateRandomToken generates a random token as a hex string
func generateRandomToken() (string, error) {
	// Generate 32 random bytes (256 bits) for a secure token
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	return hex.EncodeToString(token), nil
}

// HandleCreateAdminToken creates a new admin token
// POST /admin/tokens
func (h *Handler) HandleCreateAdminToken(w http.ResponseWriter, r *http.Request) {
	if h.templates == nil {
		http.Error(w, "Templates not loaded", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "Name required", http.StatusBadRequest)
		return
	}

	// Generate random token
	token, err := generateRandomToken()
	if err != nil {
		h.logger.Error("failed to generate token", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Create token in storage
	id, err := h.storage.CreateAdminToken(r.Context(), name, token)
	if err != nil {
		h.logger.Error("failed to create token", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("admin token created", "id", id, "name", name)

	data := map[string]any{
		"Title": "Create Admin Token",
		"ID":    id,
		"Name":  name,
		"Token": token,
	}

	if err := h.templates.ExecuteTemplate(w, "layout.html", data); err != nil {
		h.logger.Error("template error", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

// HandleDeleteAdminToken deletes an admin token
// POST /admin/tokens/{id}/delete
func (h *Handler) HandleDeleteAdminToken(w http.ResponseWriter, r *http.Request) {
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
	http.Redirect(w, r, "/admin/tokens", http.StatusSeeOther)
}
