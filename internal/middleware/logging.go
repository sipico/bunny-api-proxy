// Package middleware provides HTTP middleware components for the proxy.
package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/sipico/bunny-api-proxy/internal/logging"
)

// HTTPLogging creates a middleware that logs HTTP requests and responses.
// Only active when logger level is DEBUG.
//
// Parameters:
// - logger: slog.Logger instance for writing logs
// - allowlist: Fields to preserve in JSON bodies (nil = log everything)
//
// Logs include:
// - Request: method, URL, headers (masked), body (masked), query params
// - Response: status code, headers (masked), body (masked), duration
// - Request ID from context (if present)
func HTTPLogging(logger *slog.Logger, allowlist []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip logging if logger level is not DEBUG
			if logger.Enabled(r.Context(), slog.LevelDebug) {
				logRequest(logger, r, allowlist)
			} else {
				// Level is not DEBUG, just pass through
				next.ServeHTTP(w, r)
				return
			}

			// Record response
			rec := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           new(bytes.Buffer),
			}

			start := time.Now()
			next.ServeHTTP(rec, r)
			duration := time.Since(start)

			// Log response
			if logger.Enabled(r.Context(), slog.LevelDebug) {
				logResponse(logger, r, rec, duration, allowlist)
			}
		})
	}
}

// logRequest logs the incoming HTTP request
func logRequest(logger *slog.Logger, r *http.Request, allowlist []string) {
	requestID := GetRequestID(r.Context())

	// Read request body
	var reqBody []byte
	if r.Body != nil {
		var err error
		reqBody, err = io.ReadAll(r.Body)
		if err != nil {
			logger.Error("Failed to read request body", "error", err)
			return
		}
		// Restore body for handler
		r.Body = io.NopCloser(bytes.NewReader(reqBody))
	}

	// Mask request headers
	reqHeaders := maskHeaders(r.Header)

	// Mask request body
	maskedBody := maskBody(reqBody, allowlist)

	// Extract query parameters
	queryParams := r.URL.RawQuery

	// Log request
	logger.Debug("HTTP Request",
		"request_id", requestID,
		"method", r.Method,
		"url", r.URL.Path,
		"query_params", queryParams,
		"headers", reqHeaders,
		"body", maskedBody,
	)
}

// logResponse logs the HTTP response
func logResponse(logger *slog.Logger, r *http.Request, rec *responseRecorder, duration time.Duration, allowlist []string) {
	requestID := GetRequestID(r.Context())

	// Mask response headers
	respHeaders := maskHeaders(rec.Header())

	// Mask response body
	maskedBody := maskBody(rec.body.Bytes(), allowlist)

	// Log response
	logger.Debug("HTTP Response",
		"request_id", requestID,
		"method", r.Method,
		"url", r.URL.Path,
		"status_code", rec.statusCode,
		"headers", respHeaders,
		"body", maskedBody,
		"duration_ms", duration.Milliseconds(),
	)
}

// maskHeaders masks sensitive header values
func maskHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range headers {
		if len(v) > 0 {
			maskedValue := logging.MaskHeader(k, v[0])
			result[k] = maskedValue
		}
	}
	return result
}

// maskBody masks sensitive data in request/response body
func maskBody(body []byte, allowlist []string) string {
	if len(body) == 0 {
		return ""
	}

	// Check if body is valid UTF-8
	if !utf8.Valid(body) {
		return logging.FormatBinaryData(body)
	}

	// Mask JSON body with allowlist
	maskedBody := logging.MaskJSONBody(body, allowlist)

	return string(maskedBody)
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
