package bunny

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// ErrNotConfigured is returned when the master API key is not configured.
var ErrNotConfigured = errors.New("master API key not configured")

// KeyStore provides access to the master API key.
type KeyStore interface {
	// GetMasterAPIKey retrieves and decrypts the master bunny.net API key.
	// Returns storage.ErrNotFound if no key is configured.
	GetMasterAPIKey(ctx context.Context) (string, error)
}

// StorageClient wraps the real bunny client, fetching the API key from storage
// on each request. This allows the master API key to be updated without
// restarting the service.
type StorageClient struct {
	keyStore   KeyStore
	baseURL    string
	httpClient *http.Client
}

// StorageClientOption configures a StorageClient.
type StorageClientOption func(*StorageClient)

// WithStorageClientBaseURL sets a custom base URL (useful for testing with mock server).
func WithStorageClientBaseURL(url string) StorageClientOption {
	return func(c *StorageClient) {
		c.baseURL = url
	}
}

// WithStorageClientHTTPClient sets a custom HTTP client.
func WithStorageClientHTTPClient(client *http.Client) StorageClientOption {
	return func(c *StorageClient) {
		c.httpClient = client
	}
}

// NewStorageClient creates a client that fetches API key from storage on each request.
func NewStorageClient(keyStore KeyStore, opts ...interface{}) *StorageClient {
	c := &StorageClient{
		keyStore:   keyStore,
		baseURL:    DefaultBaseURL,
		httpClient: http.DefaultClient,
	}

	// Apply options - handle both Client and StorageClient options
	for _, opt := range opts {
		switch o := opt.(type) {
		case StorageClientOption:
			o(c)
		case Option:
			// For backward compatibility, we'll use a workaround:
			// Create a temporary client to apply the option, then extract values
			tempClient := &Client{baseURL: c.baseURL, httpClient: c.httpClient}
			o(tempClient)
			c.baseURL = tempClient.baseURL
			c.httpClient = tempClient.httpClient
		}
	}

	return c
}

// ListZones retrieves all DNS zones, optionally filtered.
// Fetches the master API key from storage for each request.
func (c *StorageClient) ListZones(ctx context.Context, opts *ListZonesOptions) (*ListZonesResponse, error) {
	apiKey, err := c.keyStore.GetMasterAPIKey(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("failed to get master API key: %w", err)
	}

	// Create a real client with the fetched key
	client := NewClient(apiKey,
		WithBaseURL(c.baseURL),
		WithHTTPClient(c.httpClient),
	)

	return client.ListZones(ctx, opts)
}

// GetZone retrieves a single DNS zone by ID, including all its records.
// Fetches the master API key from storage for each request.
func (c *StorageClient) GetZone(ctx context.Context, id int64) (*Zone, error) {
	apiKey, err := c.keyStore.GetMasterAPIKey(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("failed to get master API key: %w", err)
	}

	// Create a real client with the fetched key
	client := NewClient(apiKey,
		WithBaseURL(c.baseURL),
		WithHTTPClient(c.httpClient),
	)

	return client.GetZone(ctx, id)
}

// AddRecord adds a new DNS record to a zone.
// Fetches the master API key from storage for each request.
func (c *StorageClient) AddRecord(ctx context.Context, zoneID int64, req *AddRecordRequest) (*Record, error) {
	apiKey, err := c.keyStore.GetMasterAPIKey(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("failed to get master API key: %w", err)
	}

	// Create a real client with the fetched key
	client := NewClient(apiKey,
		WithBaseURL(c.baseURL),
		WithHTTPClient(c.httpClient),
	)

	return client.AddRecord(ctx, zoneID, req)
}

// DeleteRecord removes a DNS record from the specified zone.
// Fetches the master API key from storage for each request.
func (c *StorageClient) DeleteRecord(ctx context.Context, zoneID, recordID int64) error {
	apiKey, err := c.keyStore.GetMasterAPIKey(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrNotConfigured
		}
		return fmt.Errorf("failed to get master API key: %w", err)
	}

	// Create a real client with the fetched key
	client := NewClient(apiKey,
		WithBaseURL(c.baseURL),
		WithHTTPClient(c.httpClient),
	)

	return client.DeleteRecord(ctx, zoneID, recordID)
}
