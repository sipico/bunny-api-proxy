package admin

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// renderTemplate renders a template with layout
func (h *Handler) renderTemplate(w http.ResponseWriter, contentTemplate string, data map[string]any) error {
	if h.templates == nil {
		return fmt.Errorf("templates not loaded")
	}

	// We need to render layout.html but call the content template inside it
	// Since we can't pass a template reference, we'll execute the content template
	// and capture its output, then render layout with that content

	// Get the content template
	contentTmpl := h.templates.Lookup(contentTemplate)
	if contentTmpl == nil {
		return fmt.Errorf("template %s not found", contentTemplate)
	}

	// Execute content template to get its output
	contentBuf := &bytes.Buffer{}
	if err := contentTmpl.Execute(contentBuf, data); err != nil {
		return err
	}

	// Now render layout with the content
	layoutData := map[string]any{
		"Title":        data["Title"],
		"RenderedHTML": template.HTML(contentBuf.String()),
	}

	return h.templates.ExecuteTemplate(w, "layout.html", layoutData)
}

// HandleListKeys shows all scoped keys
// GET /admin/keys
func (h *Handler) HandleListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.storage.ListScopedKeys(r.Context())
	if err != nil {
		h.logger.Error("failed to list keys", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if h.templates == nil {
		http.Error(w, "Templates not loaded", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Title": "API Keys",
		"Keys":  keys,
	}

	if err := h.renderTemplate(w, "keys_content", data); err != nil {
		h.logger.Error("template error", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

// HandleKeyDetail shows a single key with permissions
// GET /admin/keys/{id}
func (h *Handler) HandleKeyDetail(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid key ID", http.StatusBadRequest)
		return
	}

	key, err := h.storage.GetScopedKey(r.Context(), id)
	if err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "Key not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get key", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	permissions, err := h.storage.GetPermissions(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to list permissions", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Title":       "Key Details",
		"Key":         key,
		"Permissions": permissions,
	}

	if err := h.renderTemplate(w, "key_detail_content", data); err != nil {
		h.logger.Error("template error", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

// HandleNewKeyForm shows create key form
// GET /admin/keys/new
func (h *Handler) HandleNewKeyForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title": "Create API Key",
	}

	if err := h.renderTemplate(w, "key_form_content", data); err != nil {
		h.logger.Error("template error", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

// HandleCreateKey creates a new scoped key
// POST /admin/keys
func (h *Handler) HandleCreateKey(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	apiKey := r.FormValue("api_key")

	if name == "" || apiKey == "" {
		http.Error(w, "Name and API key required", http.StatusBadRequest)
		return
	}

	id, err := h.storage.CreateScopedKey(r.Context(), name, apiKey)
	if err != nil {
		h.logger.Error("failed to create key", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("scoped key created", "id", id, "name", name)
	http.Redirect(w, r, "/admin/keys", http.StatusSeeOther)
}

// HandleDeleteKey deletes a key
// POST /admin/keys/{id}/delete
func (h *Handler) HandleDeleteKey(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid key ID", http.StatusBadRequest)
		return
	}

	if err := h.storage.DeleteScopedKey(r.Context(), id); err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "Key not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to delete key", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("scoped key deleted", "id", id)
	http.Redirect(w, r, "/admin/keys", http.StatusSeeOther)
}

// HandleAddPermissionForm shows the form to add a permission
// GET /admin/keys/{id}/permissions/new
func (h *Handler) HandleAddPermissionForm(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid key ID", http.StatusBadRequest)
		return
	}

	// Verify key exists
	key, err := h.storage.GetScopedKey(r.Context(), id)
	if err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "Key not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get key", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Title": "Add Permission",
		"Key":   key,
	}

	if err := h.renderTemplate(w, "permission_form_content", data); err != nil {
		h.logger.Error("template error", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

// HandleAddPermission adds a permission to a key
// POST /admin/keys/{id}/permissions
func (h *Handler) HandleAddPermission(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	keyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid key ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	zoneIDStr := r.FormValue("zone_id")
	actionsStr := r.FormValue("allowed_actions")
	recordTypesStr := r.FormValue("record_types")

	if zoneIDStr == "" || actionsStr == "" || recordTypesStr == "" {
		http.Error(w, "Zone ID, actions, and record types required", http.StatusBadRequest)
		return
	}

	zoneID, err := strconv.ParseInt(zoneIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid zone ID", http.StatusBadRequest)
		return
	}

	// Parse comma-separated values
	actions := parseCSV(actionsStr)
	recordTypes := parseCSV(recordTypesStr)

	if len(actions) == 0 || len(recordTypes) == 0 {
		http.Error(w, "Actions and record types cannot be empty", http.StatusBadRequest)
		return
	}

	perm := &storage.Permission{
		ZoneID:         zoneID,
		AllowedActions: actions,
		RecordTypes:    recordTypes,
	}

	permID, err := h.storage.AddPermission(r.Context(), keyID, perm)
	if err != nil {
		h.logger.Error("failed to add permission", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("permission added", "id", permID, "key_id", keyID)
	http.Redirect(w, r, "/admin/keys/"+idStr, http.StatusSeeOther)
}

// HandleDeletePermission removes a permission
// POST /admin/keys/{id}/permissions/{pid}/delete
func (h *Handler) HandleDeletePermission(w http.ResponseWriter, r *http.Request) {
	keyIDStr := chi.URLParam(r, "id")
	permIDStr := chi.URLParam(r, "pid")

	permID, err := strconv.ParseInt(permIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid permission ID", http.StatusBadRequest)
		return
	}

	if err := h.storage.DeletePermission(r.Context(), permID); err != nil {
		if err == storage.ErrNotFound {
			http.Error(w, "Permission not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to delete permission", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("permission deleted", "id", permID)
	http.Redirect(w, r, "/admin/keys/"+keyIDStr, http.StatusSeeOther)
}

// parseCSV parses comma-separated values, trimming whitespace and filtering empty
func parseCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	items := make([]string, 0)
	for _, item := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}
