// Package bunny provides a client for the bunny.net API.
package bunny

import (
	"net/http"
)

// Client wraps HTTP operations for bunny.net API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// New creates a new bunny.net API client.
func New(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{},
		baseURL:    "https://api.bunny.net",
		apiKey:     apiKey,
	}
}
