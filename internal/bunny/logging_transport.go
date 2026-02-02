package bunny

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
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

	// Read request body
	var reqBodyBytes []byte
	if req.Body != nil {
		var err error
		reqBodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		// Restore body for transport
		req.Body = io.NopCloser(bytes.NewReader(reqBodyBytes))
	}

	// Prepare request headers for logging (with redaction)
	reqHeaders := make(map[string]string)
	for k, v := range req.Header {
		if strings.EqualFold(k, "AccessKey") || strings.EqualFold(k, "Authorization") {
			reqHeaders[k] = redactSensitiveData(strings.Join(v, ", "))
		} else {
			reqHeaders[k] = strings.Join(v, ", ")
		}
	}

	// Log the request
	t.Logger.Info("HTTP Request",
		"prefix", t.Prefix,
		"method", req.Method,
		"url", req.URL.String(),
		"headers", reqHeaders,
		"body", string(reqBodyBytes),
	)

	// Execute the request
	resp, err := t.transport().RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		// Log error
		t.Logger.Error("HTTP request failed",
			"prefix", t.Prefix,
			"method", req.Method,
			"url", req.URL.String(),
			"duration_ms", duration.Milliseconds(),
			"error", err,
		)
		return nil, err
	}

	// Read response body
	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// Restore body for caller
	resp.Body = io.NopCloser(bytes.NewReader(respBodyBytes))

	// Log the response
	t.Logger.Info("HTTP Response",
		"prefix", t.Prefix,
		"status_code", resp.StatusCode,
		"status", resp.Status,
		"duration_ms", duration.Milliseconds(),
		"headers", resp.Header,
		"body", string(respBodyBytes),
	)

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
