package admin

import (
	"net/http"

	"github.com/sipico/bunny-api-proxy/internal/auth"
)

// RequireAdmin is middleware that requires admin privileges.
// It must be used after TokenAuthMiddleware.
// Returns 403 Forbidden if the request is not from an admin token.
func (h *Handler) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.IsAdminFromContext(r.Context()) {
			WriteErrorWithHint(w, http.StatusForbidden, ErrCodeAdminRequired,
				"This endpoint requires an admin token",
				"Use an admin token (is_admin: true) to access admin-only endpoints")
			return
		}
		next.ServeHTTP(w, r)
	})
}
