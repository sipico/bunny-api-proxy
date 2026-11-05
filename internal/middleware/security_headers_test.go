package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// expectedSecurityHeaders are set on every response by SecurityHeaders (issue #398).
var expectedSecurityHeaders = map[string]string{
	"X-Content-Type-Options": "nosniff",
	"X-Frame-Options":        "DENY",
	"Cache-Control":          "no-store",
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
	}{
		{"success response", http.StatusOK},
		{"error response", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			called := false
			handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte("body"))
			}))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/anything", nil))

			if !called {
				t.Fatal("next handler was not called")
			}
			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
			}
			if rec.Body.String() != "body" {
				t.Errorf("body = %q, want %q", rec.Body.String(), "body")
			}
			for name, want := range expectedSecurityHeaders {
				if got := rec.Header().Get(name); got != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
			}
		})
	}
}
