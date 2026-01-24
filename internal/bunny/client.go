// Package bunny provides types and error handling for the bunny.net API client.
package bunny

import (
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
