package bunny

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
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
