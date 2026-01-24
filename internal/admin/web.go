package admin

import (
	"net/http"
)

// HandleDashboard shows the admin dashboard
// GET /admin
func (h *Handler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	if h.templates == nil {
		http.Error(w, "Templates not loaded", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Title": "Admin Dashboard",
	}

	if err := h.templates.ExecuteTemplate(w, "layout.html", data); err != nil {
		h.logger.Error("template error", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

// HandleMasterKeyForm shows the master key form
// GET /admin/master-key
func (h *Handler) HandleMasterKeyForm(w http.ResponseWriter, r *http.Request) {
	if h.templates == nil {
		http.Error(w, "Templates not loaded", http.StatusInternalServerError)
		return
	}

	// Get current key (masked)
	key, err := h.storage.GetMasterAPIKey(r.Context())
	var maskedKey string
	if err == nil && key != "" {
		if len(key) < 4 {
			maskedKey = "****"
		} else {
			maskedKey = "****" + key[len(key)-4:]
		}
	}

	data := map[string]any{
		"Title":      "Master API Key",
		"CurrentKey": maskedKey,
	}

	if err := h.templates.ExecuteTemplate(w, "layout.html", data); err != nil {
		h.logger.Error("template error", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

// HandleSetMasterKey updates the master key
// POST /admin/master-key
func (h *Handler) HandleSetMasterKey(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	key := r.FormValue("key")
	if key == "" {
		http.Error(w, "Key required", http.StatusBadRequest)
		return
	}

	if err := h.storage.SetMasterAPIKey(r.Context(), key); err != nil {
		h.logger.Error("failed to set master key", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("master key updated")
	http.Redirect(w, r, "/admin/master-key", http.StatusSeeOther)
}
