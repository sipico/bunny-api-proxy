package bunny

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sipico/bunny-api-proxy/internal/middleware"
)

// LoggingTransport wraps an http.RoundTripper and logs all HTTP interactions.
// It redacts sensitive headers like AccessKey and Authorization.
type LoggingTransport struct {
	Transport http.RoundTripper
	Logger    *slog.Logger
	Prefix    string // e.g., "MOCK" or "REAL"
}

// RoundTrip implements http.RoundTripper interface
func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	// Extract request ID from context
	requestID := middleware.GetRequestID(req.Context())

	isDebug := t.Logger.Enabled(req.Context(), slog.LevelDebug)

	// Only buffer request body if DEBUG logging is enabled
	var reqBodyBytes []byte
	if isDebug && req.Body != nil {
		var err error
		reqBodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		// Restore body for transport
		req.Body = io.NopCloser(bytes.NewReader(reqBodyBytes))
	}

	// DEBUG: Log full request details
	if isDebug {
		reqHeaders := make(map[string]string)
		for k, v := range req.Header {
			if strings.EqualFold(k, "AccessKey") || strings.EqualFold(k, "Authorization") {
				reqHeaders[k] = redactSensitiveData(strings.Join(v, ", "))
			} else {
				reqHeaders[k] = strings.Join(v, ", ")
			}
		}

		t.Logger.Debug("Bunny API request",
			"request_id", requestID,
			"prefix", t.Prefix,
			"method", req.Method,
			"url", req.URL.String(),
			"headers", reqHeaders,
			"body", string(reqBodyBytes),
		)
	}

	// Execute the request
	resp, err := t.transport().RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		// Log error
		t.Logger.Error("Bunny API call failed",
			"request_id", requestID,
			"prefix", t.Prefix,
			"method", req.Method,
			"path", req.URL.Path,
			"duration_ms", duration.Milliseconds(),
			"error", err,
		)
		return nil, err
	}

	// Only buffer response body if DEBUG logging is enabled
	var respBodyBytes []byte
	if isDebug {
		var err error
		respBodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		// Restore body for caller
		resp.Body = io.NopCloser(bytes.NewReader(respBodyBytes))
	}

	// INFO: Log operational summary
	t.Logger.Info("Bunny API call",
		"request_id", requestID,
		"prefix", t.Prefix,
		"method", req.Method,
		"path", req.URL.Path,
		"status", resp.StatusCode,
		"duration_ms", duration.Milliseconds(),
	)

	// DEBUG: Log full response details
	if isDebug {
		t.Logger.Debug("Bunny API response",
			"request_id", requestID,
			"prefix", t.Prefix,
			"status_code", resp.StatusCode,
			"status", resp.Status,
			"headers", resp.Header,
			"body", string(respBodyBytes),
		)
	}

	return resp, nil
}

// transport returns the underlying transport or DefaultTransport if nil
func (t *LoggingTransport) transport() http.RoundTripper {
	if t.Transport != nil {
		return t.Transport
	}
	return http.DefaultTransport
}

// redactSensitiveData redacts API keys showing only first 4 and last 4 chars.
// Keys with fewer than 12 characters are completely redacted with "****".
func redactSensitiveData(key string) string {
	if len(key) < 12 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// RetryTransport wraps an http.RoundTripper and retries on timeout errors and 5xx responses.
// Retries idempotent methods (GET, HEAD, OPTIONS, DELETE, PUT) on both timeout and 5xx errors.
// Retries non-idempotent methods (POST, PATCH) only on timeout errors (request didn't reach server).
// Uses exponential backoff (1s, 2s, 4s) with max 3 retries (4 total attempts).
type RetryTransport struct {
	Transport http.RoundTripper
	Logger    *slog.Logger
}

// RoundTrip implements http.RoundTripper interface with retry logic.
func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	const maxRetries = 3
	backoff := 1 * time.Second

	var resp *http.Response
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Execute request
		resp, err = t.Transport.RoundTrip(req)

		// Determine if we should retry
		shouldRetry := false
		var retryReason string

		if err != nil {
			// Network/timeout errors: retry for ALL methods (request likely didn't reach server)
			if isTimeoutError(err) {
				shouldRetry = true
				retryReason = fmt.Sprintf("timeout error: %v", err)
			}
		} else if resp != nil {
			// 5xx errors: only retry for idempotent methods
			if is5xxError(resp.StatusCode) && isIdempotentMethod(req.Method) {
				shouldRetry = true
				retryReason = fmt.Sprintf("server error: %d %s", resp.StatusCode, resp.Status)
				// Close the response body before retrying
				if resp.Body != nil {
					//nolint:errcheck
					resp.Body.Close()
				}
			}
		}

		// If we shouldn't retry or we've exhausted retries, return
		if !shouldRetry || attempt == maxRetries {
			if attempt > 0 && err != nil {
				t.Logger.Error("HTTP request failed after retries",
					"method", req.Method,
					"url", req.URL.String(),
					"attempts", attempt+1,
					"error", err,
				)
			} else if attempt > 0 && resp != nil && is5xxError(resp.StatusCode) {
				t.Logger.Error("HTTP request failed after retries",
					"method", req.Method,
					"url", req.URL.String(),
					"attempts", attempt+1,
					"status", resp.StatusCode,
				)
			}
			return resp, err
		}

		// Log retry attempt
		t.Logger.Warn("HTTP request failed, retrying",
			"method", req.Method,
			"url", req.URL.String(),
			"attempt", attempt+1,
			"reason", retryReason,
			"backoff_ms", backoff.Milliseconds(),
		)

		// Wait with exponential backoff, respecting context cancellation
		select {
		case <-time.After(backoff):
			// Backoff completed, continue to retry
		case <-req.Context().Done():
			// Context cancelled during backoff
			t.Logger.Warn("HTTP request cancelled during retry backoff",
				"method", req.Method,
				"url", req.URL.String(),
			)
			return resp, req.Context().Err()
		}

		// Exponential backoff: 1s, 2s, 4s
		backoff *= 2
	}

	return resp, err
}

// isIdempotentMethod returns true for HTTP methods that are safe to retry on server errors.
// GET, HEAD, OPTIONS: standard idempotent read operations
// DELETE: idempotent (deleting twice yields same result)
// PUT: typically idempotent (creating/replacing same resource)
// POST: NOT idempotent (may have side effects)
func isIdempotentMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodDelete, http.MethodPut:
		return true
	default:
		return false
	}
}

// is5xxError returns true if the status code is a server error (5xx).
func is5xxError(statusCode int) bool {
	return statusCode >= 500 && statusCode < 600
}

// isTimeoutError checks if an error is a timeout error (context deadline or network timeout).
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context deadline exceeded
	if err == context.DeadlineExceeded {
		return true
	}

	// Check for net.Error with Timeout() method
	if os.IsTimeout(err) {
		return true
	}

	// Check string representation for timeout patterns
	errStr := err.Error()
	return strings.Contains(errStr, "Client.Timeout exceeded") ||
		strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "i/o timeout")
}
