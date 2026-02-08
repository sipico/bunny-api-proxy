// Package bunny provides types and error handling for the bunny.net API client.
package bunny

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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

// ListZones retrieves all DNS zones, optionally filtered.
// Returns the full paginated response.
func (c *Client) ListZones(ctx context.Context, opts *ListZonesOptions) (*ListZonesResponse, error) {
	// Use defaults if opts is nil
	if opts == nil {
		opts = &ListZonesOptions{}
	}

	// Build URL with query parameters
	query := url.Values{}

	if opts.Page > 0 {
		query.Set("page", strconv.Itoa(opts.Page))
	}

	if opts.PerPage > 0 {
		query.Set("perPage", strconv.Itoa(opts.PerPage))
	}

	if opts.Search != "" {
		query.Set("search", opts.Search)
	}

	endpoint := c.baseURL + "/dnszone"
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Set authentication header
	req.Header.Set("AccessKey", c.apiKey)

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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp.StatusCode, body)
	}

	// Decode successful response
	var result ListZonesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetZone retrieves a single DNS zone by ID, including all its records.
func (c *Client) GetZone(ctx context.Context, id int64) (*Zone, error) {
	url := fmt.Sprintf("%s/dnszone/%d", c.baseURL, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("AccessKey", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Handle specific status codes
	if resp.StatusCode == http.StatusOK {
		var zone Zone
		if err := json.Unmarshal(body, &zone); err != nil {
			return nil, fmt.Errorf("failed to decode zone: %w", err)
		}
		return &zone, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	// Use generic error parser for all other cases (including 401)
	return nil, parseError(resp.StatusCode, body)
}

// AddRecordRequest represents the request body for creating a new DNS record.
type AddRecordRequest struct {
	Type     int    `json:"Type"` // 0 = A, 1 = AAAA, 2 = CNAME, 3 = TXT, 4 = MX, 5 = SPF, 6 = Flatten, 7 = PullZone, 8 = SRV, 9 = CAA, 10 = PTR, 11 = Script, 12 = NS
	Name     string `json:"Name"`
	Value    string `json:"Value"`
	TTL      int32  `json:"Ttl"`
	Priority int32  `json:"Priority"`
	Weight   int32  `json:"Weight"`
	Port     int32  `json:"Port"`
	Flags    int    `json:"Flags"`
	Tag      string `json:"Tag"`
	Disabled bool   `json:"Disabled"`
	Comment  string `json:"Comment"`
}

// AddRecord adds a new DNS record to a zone.
func (c *Client) AddRecord(ctx context.Context, zoneID int64, req *AddRecordRequest) (*Record, error) {
	url := fmt.Sprintf("%s/dnszone/%d/records", c.baseURL, zoneID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("AccessKey", c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Handle specific status codes
	if resp.StatusCode == http.StatusCreated {
		var record Record
		if err := json.Unmarshal(respBody, &record); err != nil {
			return nil, fmt.Errorf("failed to decode record: %w", err)
		}
		return &record, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	// Use generic error parser for all other cases (including 401)
	return nil, parseError(resp.StatusCode, respBody)
}

// UpdateRecord updates an existing DNS record in a zone.
func (c *Client) UpdateRecord(ctx context.Context, zoneID, recordID int64, req *AddRecordRequest) (*Record, error) {
	url := fmt.Sprintf("%s/dnszone/%d/records/%d", c.baseURL, zoneID, recordID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("AccessKey", c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Handle specific status codes
	if resp.StatusCode == http.StatusOK {
		var record Record
		if err := json.Unmarshal(respBody, &record); err != nil {
			return nil, fmt.Errorf("failed to decode record: %w", err)
		}
		return &record, nil
	}

	if resp.StatusCode == http.StatusNoContent {
		// 204 No Content - success with no response body (real bunny.net API behavior)
		return nil, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	// Use generic error parser for all other cases (including 401)
	return nil, parseError(resp.StatusCode, respBody)
}

// DeleteRecord removes a DNS record from the specified zone.
func (c *Client) DeleteRecord(ctx context.Context, zoneID, recordID int64) error {
	url := fmt.Sprintf("%s/dnszone/%d/records/%d", c.baseURL, zoneID, recordID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("AccessKey", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Handle specific status codes
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}

	// Use generic error parser for all other cases (including 401)
	return parseError(resp.StatusCode, body)
}

// CreateZone creates a new DNS zone.
// POST /dnszone
func (c *Client) CreateZone(ctx context.Context, domain string) (*Zone, error) {
	url := fmt.Sprintf("%s/dnszone", c.baseURL)

	req := &CreateZoneRequest{
		Domain: domain,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("AccessKey", c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Handle specific status codes
	if resp.StatusCode == http.StatusCreated {
		var zone Zone
		if err := json.Unmarshal(respBody, &zone); err != nil {
			return nil, fmt.Errorf("failed to decode zone: %w", err)
		}
		return &zone, nil
	}

	// Use generic error parser for all other cases (including 401, 400, 409)
	return nil, parseError(resp.StatusCode, respBody)
}

// DeleteZone deletes a DNS zone by ID.
// DELETE /dnszone/{id}
func (c *Client) DeleteZone(ctx context.Context, id int64) error {
	url := fmt.Sprintf("%s/dnszone/%d", c.baseURL, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("AccessKey", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Handle specific status codes
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}

	// Use generic error parser for all other cases (including 401)
	return parseError(resp.StatusCode, body)

}

// UpdateZone updates zone-level settings.
func (c *Client) UpdateZone(ctx context.Context, id int64, req *UpdateZoneRequest) (*Zone, error) {
	url := fmt.Sprintf("%s/dnszone/%d", c.baseURL, id)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("AccessKey", c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to update zone: %w", err)
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var zone Zone
		if err := json.Unmarshal(respBody, &zone); err != nil {
			return nil, fmt.Errorf("failed to parse zone: %w", err)
		}
		return &zone, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	return nil, parseError(resp.StatusCode, respBody)
}

// CheckZoneAvailability checks if a domain name is available to be added as a DNS zone.
func (c *Client) CheckZoneAvailability(ctx context.Context, name string) (*CheckAvailabilityResponse, error) {
	url := fmt.Sprintf("%s/dnszone/checkavailability", c.baseURL)

	reqBody := &CheckAvailabilityRequest{Name: name}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("AccessKey", c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to check availability: %w", err)
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var result CheckAvailabilityResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		return &result, nil
	}

	return nil, parseError(resp.StatusCode, respBody)
}

// ImportRecords imports DNS records from BIND zone file format.
// The body is forwarded as-is to the bunny.net API.
func (c *Client) ImportRecords(ctx context.Context, zoneID int64, body io.Reader, contentType string) (*ImportRecordsResponse, error) {
	url := fmt.Sprintf("%s/dnszone/%d/import", c.baseURL, zoneID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("AccessKey", c.apiKey)
	if contentType != "" {
		httpReq.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to import records: %w", err)
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var result ImportRecordsResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		return &result, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	return nil, parseError(resp.StatusCode, respBody)
}

// ExportRecords exports DNS records in BIND zone file format.
// Returns the raw text response body.
func (c *Client) ExportRecords(ctx context.Context, zoneID int64) (string, error) {
	url := fmt.Sprintf("%s/dnszone/%d/export", c.baseURL, zoneID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("AccessKey", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to export records: %w", err)
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		return string(body), nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return "", ErrNotFound
	}

	return "", parseError(resp.StatusCode, body)
}

// EnableDNSSEC enables DNSSEC for a DNS zone.
func (c *Client) EnableDNSSEC(ctx context.Context, zoneID int64) (*DNSSECResponse, error) {
	url := fmt.Sprintf("%s/dnszone/%d/dnssec", c.baseURL, zoneID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("AccessKey", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to enable DNSSEC: %w", err)
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var result DNSSECResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		return &result, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	return nil, parseError(resp.StatusCode, body)
}

// DisableDNSSEC disables DNSSEC for a DNS zone.
func (c *Client) DisableDNSSEC(ctx context.Context, zoneID int64) (*DNSSECResponse, error) {
	url := fmt.Sprintf("%s/dnszone/%d/dnssec", c.baseURL, zoneID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("AccessKey", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to disable DNSSEC: %w", err)
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var result DNSSECResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		return &result, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	return nil, parseError(resp.StatusCode, body)
}

// IssueCertificate triggers issuance of a wildcard SSL certificate for a zone.
func (c *Client) IssueCertificate(ctx context.Context, zoneID int64, domain string) error {
	url := fmt.Sprintf("%s/dnszone/%d/certificate/issue", c.baseURL, zoneID)

	reqBody := struct {
		Domain string `json:"Domain"`
	}{Domain: domain}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("AccessKey", c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to issue certificate: %w", err)
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}

	return parseError(resp.StatusCode, respBody)
}

// GetZoneStatistics retrieves DNS query statistics for a zone.
func (c *Client) GetZoneStatistics(ctx context.Context, zoneID int64, dateFrom, dateTo string) (*ZoneStatisticsResponse, error) {
	endpoint := fmt.Sprintf("%s/dnszone/%d/statistics", c.baseURL, zoneID)

	// Add query parameters if provided
	query := url.Values{}
	if dateFrom != "" {
		query.Set("dateFrom", dateFrom)
	}
	if dateTo != "" {
		query.Set("dateTo", dateTo)
	}
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("AccessKey", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get statistics: %w", err)
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var result ZoneStatisticsResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		return &result, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	return nil, parseError(resp.StatusCode, body)
}

// parseError parses API error responses and returns an appropriate error.
func parseError(statusCode int, body []byte) error {
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

// TriggerDNSScan triggers a background DNS record scan for a zone.
func (c *Client) TriggerDNSScan(ctx context.Context, zoneID int64) error {
	url := fmt.Sprintf("%s/dnszone/%d/recheckdns", c.baseURL, zoneID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("AccessKey", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to trigger DNS scan: %w", err)
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}

	return parseError(resp.StatusCode, body)
}

// GetDNSScanResult retrieves the latest DNS record scan result.
func (c *Client) GetDNSScanResult(ctx context.Context, zoneID int64) (*DNSScanResult, error) {
	url := fmt.Sprintf("%s/dnszone/%d/recheckdns", c.baseURL, zoneID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("AccessKey", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get scan result: %w", err)
	}
	defer func() {
		//nolint:errcheck
		resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var result DNSScanResult
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		return &result, nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	return nil, parseError(resp.StatusCode, body)
}
