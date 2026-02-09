package mockbunny

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// LoggingMiddleware logs all HTTP requests and responses to mockbunny.
// Only active when logger is provided (non-nil).
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if logger == nil {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Read and log request body
			var reqBody []byte
			if r.Body != nil {
				var err error
				reqBody, err = io.ReadAll(r.Body)
				if err != nil {
					logger.Error("Failed to read request body", "error", err)
					http.Error(w, "Failed to read request body", http.StatusInternalServerError)
					return
				}
				// Restore body for handler
				r.Body = io.NopCloser(bytes.NewReader(reqBody))
			}

			// Log request with redacted headers
			reqHeaders := redactHeaders(r.Header)
			logger.Info("MockBunny received request",
				"method", r.Method,
				"url", r.URL.String(),
				"headers", reqHeaders,
				"body", string(reqBody),
			)

			// Capture response
			rec := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           new(bytes.Buffer),
			}

			next.ServeHTTP(rec, r)

			duration := time.Since(start)

			// Log response
			logger.Info("MockBunny sent response",
				"method", r.Method,
				"url", r.URL.String(),
				"status_code", rec.statusCode,
				"duration_ms", duration.Milliseconds(),
				"body", rec.body.String(),
			)
		})
	}
}

// responseRecorder captures response details for logging.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

// WriteHeader captures the status code and writes it to the response.
func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Write captures the response body and writes it to the response.
func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b) // Capture for logging
	return r.ResponseWriter.Write(b)
}

// redactHeaders redacts sensitive header values.
func redactHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range headers {
		if strings.EqualFold(k, "AccessKey") || strings.EqualFold(k, "Authorization") {
			result[k] = redactAPIKey(strings.Join(v, ", "))
		} else {
			result[k] = strings.Join(v, ", ")
		}
	}
	return result
}

// redactAPIKey redacts API keys showing only first 4 and last 4 chars.
// Keys with fewer than 12 characters are completely redacted with "****".
func redactAPIKey(key string) string {
	if len(key) < 12 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// VaryAcceptEncodingMiddleware adds the Vary: Accept-Encoding header to GET responses.
// This header indicates that the response content may vary based on the Accept-Encoding header.
// It mimics the behavior of the real bunny.net API which includes this header on GET responses.
func VaryAcceptEncodingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only add Vary header for GET requests
		if r.Method == http.MethodGet {
			w.Header().Set("Vary", "Accept-Encoding")
		}
		next.ServeHTTP(w, r)
	})
}

// FailureInjectionMiddleware creates a middleware that applies configured failure injection rules.
// It handles: error injection, latency injection, rate limiting, and malformed responses.
// Admin endpoints are excluded from failure injection to allow control of the mock server.
func FailureInjectionMiddleware(state *State) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip failure injection for admin endpoints
			if strings.HasPrefix(r.URL.Path, "/admin") {
				next.ServeHTTP(w, r)
				return
			}

			state.mu.Lock()
			fi := state.failureInjection

			// Check for pending latency
			if fi.latencyRemaining > 0 {
				latency := fi.latencyDuration
				state.failureInjection.latencyRemaining--
				state.mu.Unlock()

				// Sleep outside the lock
				time.Sleep(latency)

				state.mu.Lock()
			}

			// Check for rate limiting (only if enabled: rateLimitAfter >= 0)
			if fi.rateLimitAfter >= 0 && fi.rateLimitCounter >= fi.rateLimitAfter {
				state.mu.Unlock()
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusTooManyRequests)
				//nolint:errcheck
				w.Write([]byte(`{"ErrorKey":"rate_limit","Message":"Too many requests"}`))
				return
			}

			// Check for pending errors
			if fi.nextErrorRemaining > 0 {
				statusCode := fi.nextErrorStatus
				message := fi.nextErrorMessage
				state.failureInjection.nextErrorRemaining--
				state.mu.Unlock()

				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(statusCode)
				//nolint:errcheck
				w.Write([]byte(`{"ErrorKey":"injected_error","Message":"` + message + `"}`))
				return
			}

			// Check for malformed response
			if fi.malformedRemaining > 0 {
				state.failureInjection.malformedRemaining--
				state.mu.Unlock()

				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				// Return intentionally invalid JSON
				//nolint:errcheck
				w.Write([]byte(`{invalid json`))
				return
			}

			// If rate limit is active, increment counter for successful requests
			if fi.rateLimitAfter >= 0 {
				state.failureInjection.rateLimitCounter++
			}

			state.mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}
