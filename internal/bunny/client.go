// Package bunny provides types and error handling for the bunny.net API client.
package bunny

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	// DefaultBaseURL is the default base URL for the bunny.net API.
	DefaultBaseURL = "https://api.bunny.net"
)

// Client is an HTTP client for the bunny.net DNS API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL sets a custom base URL (useful for testing with mock server).
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// NewClient creates a new bunny.net API client.
func NewClient(apiKey string, opts ...Option) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		apiKey:     apiKey,
		httpClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// AddRecord creates a new DNS record in the specified zone.
// Returns the created record with its assigned ID.
// Returns ErrNotFound if the zone does not exist.
// Returns ErrUnauthorized if the API key is invalid.
// Returns APIError for validation errors (400 status).
func (c *Client) AddRecord(ctx context.Context, zoneID int64, record Record) (*Record, error) {
	// Build URL with zone ID in path
	endpoint := fmt.Sprintf("%s/dnszone/%d/records", c.baseURL, zoneID)

	// Marshal record to JSON for request body
	body, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal record: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("AccessKey", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Handle error responses
	switch resp.StatusCode {
	case http.StatusCreated:
		// Success: 201 Created, decode response
		var result Record
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("failed to decode record response: %w", err)
		}
		return &result, nil
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case http.StatusBadRequest:
		// Return APIError for validation errors
		return nil, c.parseError(resp.StatusCode, respBody)
	default:
		return nil, c.parseError(resp.StatusCode, respBody)
	}
}

// DeleteRecord removes a DNS record from the specified zone.
// Returns ErrNotFound if the zone or record does not exist.
// Returns ErrUnauthorized if the API key is invalid.
func (c *Client) DeleteRecord(ctx context.Context, zoneID, recordID int64) error {
	// Build URL with zone ID and record ID in path
	endpoint := fmt.Sprintf("%s/dnszone/%d/records/%d", c.baseURL, zoneID, recordID)

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	// Set authentication header
	req.Header.Set("AccessKey", c.apiKey)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	// Read response body for error handling
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Handle error responses
	switch resp.StatusCode {
	case http.StatusNoContent:
		// Success: 204 No Content
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusUnauthorized:
		return ErrUnauthorized
	default:
		return c.parseError(resp.StatusCode, respBody)
	}
}

// parseError parses API error responses and returns an appropriate error.
func (c *Client) parseError(statusCode int, body []byte) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusInternalServerError, http.StatusServiceUnavailable:
		// Try to parse as structured error
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Message != "" {
			apiErr.StatusCode = statusCode
			return &apiErr
		}
		// Fall back to generic error
		return fmt.Errorf("bunny: server error (status %d)", statusCode)
	default:
		// Try to parse as structured error
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Message != "" {
			apiErr.StatusCode = statusCode
			return &apiErr
		}
		// Fall back to generic error
		return fmt.Errorf("bunny: request failed (status %d)", statusCode)
	}
}
