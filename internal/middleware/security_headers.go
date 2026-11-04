package middleware

import "net/http"

// SecurityHeaders adds standard security headers to every response.
// They provide defense in depth even when the proxy runs behind a reverse
// proxy that sets its own headers (issue #398):
//   - X-Content-Type-Options: nosniff prevents MIME type sniffing
//   - X-Frame-Options: DENY prevents the responses from being framed
//   - Cache-Control: no-store keeps API responses, which can contain
//     tokens or zone data, out of shared and browser caches
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
