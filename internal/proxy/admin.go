// Package proxy implements the API proxy for bunny.net requests.
package proxy

import (
	"encoding/json"
	"net/http"

	"github.com/sipico/bunny-api-proxy/internal/auth"
)

// requireAdmin is middleware that restricts access to admin tokens only.
// Returns 403 with a JSON error for non-admin requests.
func requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.IsAdminFromContext(r.Context()) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			//nolint:errcheck
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "admin_required",
				"message": "This endpoint requires an admin token.",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}
