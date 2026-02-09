package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// HandleHealth returns basic health status
// GET /health
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
	if err != nil {
		// Encoding errors are not critical for health check responses
		_ = err
	}
}

// HandleReady checks database connectivity
// GET /ready
// Returns 200 if database is accessible, 503 otherwise
func (h *Handler) HandleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h.storage == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		//nolint:errcheck // Response write errors are unrecoverable
		json.NewEncoder(w).Encode(map[string]any{
			"status":   "error",
			"database": "not configured",
		})
		return
	}

	// Check database connectivity with a lightweight ping
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := h.storage.Ping(ctx)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		//nolint:errcheck // Response write errors are unrecoverable
		json.NewEncoder(w).Encode(map[string]any{
			"status":   "error",
			"database": "unavailable",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	//nolint:errcheck // Response write errors are unrecoverable
	json.NewEncoder(w).Encode(map[string]any{
		"status":   "ok",
		"database": "connected",
	})
}
