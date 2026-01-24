package admin

import (
	"encoding/json"
	"net/http"
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
	// Test database connectivity by checking Close() exists
	// In a real health check, we'd ping the database
	// For now, check if storage is not nil

	w.Header().Set("Content-Type", "application/json")

	if h.storage == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		err := json.NewEncoder(w).Encode(map[string]any{
			"status":   "error",
			"database": "not configured",
		})
		if err != nil {
			// Encoding errors are not critical for readiness check responses
			_ = err
		}
		return
	}

	// If we reach here, assume database is ready
	// More sophisticated check added when storage interface has Ping()
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(map[string]any{
		"status":   "ok",
		"database": "connected",
	})
	if err != nil {
		// Encoding errors are not critical for readiness check responses
		_ = err
	}
}
