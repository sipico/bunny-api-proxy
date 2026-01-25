package bunny

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/storage"
	"github.com/sipico/bunny-api-proxy/internal/testutil/mockbunny"
)

// MockKeyStore is a mock implementation of KeyStore for testing.
type MockKeyStore struct {
	apiKey string
	err    error
}

// GetMasterAPIKey returns the mock API key or error.
func (m *MockKeyStore) GetMasterAPIKey(ctx context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.apiKey, nil
}

func TestStorageClient_ListZones_Success(t *testing.T) {
	// Setup mock server
	server := mockbunny.New()
	defer server.Close()

	// Add a test zone
	server.AddZone("example.com")

	// Setup mock KeyStore
	mockKS := &MockKeyStore{
		apiKey: "test-key",
	}

	// Create StorageClient with mock server
	client := NewStorageClient(mockKS, WithStorageClientBaseURL(server.URL()))

	// Call ListZones
	ctx := context.Background()
	result, err := client.ListZones(ctx, nil)

	// Assert success
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatalf("expected result, got nil")
	}

	// Verify it returns mock data
	if len(result.Items) == 0 {
		t.Fatalf("expected at least one zone in result")
	}
}

func TestStorageClient_ListZones_KeyNotConfigured(t *testing.T) {
	// Setup mock KeyStore that returns ErrNotFound
	mockKS := &MockKeyStore{
		err: storage.ErrNotFound,
	}

	client := NewStorageClient(mockKS)

	// Call ListZones
	ctx := context.Background()
	result, err := client.ListZones(ctx, nil)

	// Assert ErrNotConfigured
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got: %v", err)
	}

	if result != nil {
		t.Fatalf("expected nil result, got: %v", result)
	}
}

func TestStorageClient_ListZones_StorageError(t *testing.T) {
	// Setup mock KeyStore that returns a different error
	mockKS := &MockKeyStore{
		err: errors.New("database connection failed"),
	}

	client := NewStorageClient(mockKS)

	// Call ListZones
	ctx := context.Background()
	result, err := client.ListZones(ctx, nil)

	// Assert error handling
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected storage error, got ErrNotConfigured")
	}

	if result != nil {
		t.Fatalf("expected nil result, got: %v", result)
	}
}

func TestStorageClient_GetZone_Success(t *testing.T) {
	// Setup mock server
	server := mockbunny.New()
	defer server.Close()

	// Add a test zone
	zoneID := server.AddZone("example.com")

	// Setup mock KeyStore
	mockKS := &MockKeyStore{
		apiKey: "test-key",
	}

	// Create StorageClient with mock server
	client := NewStorageClient(mockKS, WithStorageClientBaseURL(server.URL()))

	// Call GetZone
	ctx := context.Background()
	result, err := client.GetZone(ctx, zoneID)

	// Assert success
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatalf("expected result, got nil")
	}

	if result.ID != zoneID {
		t.Fatalf("expected zone ID %d, got: %d", zoneID, result.ID)
	}
}

func TestStorageClient_GetZone_KeyNotConfigured(t *testing.T) {
	// Setup mock KeyStore that returns ErrNotFound
	mockKS := &MockKeyStore{
		err: storage.ErrNotFound,
	}

	client := NewStorageClient(mockKS)

	// Call GetZone
	ctx := context.Background()
	result, err := client.GetZone(ctx, 1)

	// Assert ErrNotConfigured
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got: %v", err)
	}

	if result != nil {
		t.Fatalf("expected nil result, got: %v", result)
	}
}

func TestStorageClient_AddRecord_Success(t *testing.T) {
	// Setup mock server
	server := mockbunny.New()
	defer server.Close()

	// Add a test zone
	zoneID := server.AddZone("example.com")

	// Setup mock KeyStore
	mockKS := &MockKeyStore{
		apiKey: "test-key",
	}

	// Create StorageClient with mock server
	client := NewStorageClient(mockKS, WithStorageClientBaseURL(server.URL()))

	// Create a test record request
	req := &AddRecordRequest{
		Type:  "A",
		Name:  "test",
		Value: "192.0.2.1",
		TTL:   3600,
	}

	// Call AddRecord
	ctx := context.Background()
	result, err := client.AddRecord(ctx, zoneID, req)

	// Assert success
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatalf("expected result, got nil")
	}
}

func TestStorageClient_AddRecord_KeyNotConfigured(t *testing.T) {
	// Setup mock KeyStore that returns ErrNotFound
	mockKS := &MockKeyStore{
		err: storage.ErrNotFound,
	}

	client := NewStorageClient(mockKS)

	// Create a test record request
	req := &AddRecordRequest{
		Type:  "A",
		Name:  "test",
		Value: "192.0.2.1",
		TTL:   3600,
	}

	// Call AddRecord
	ctx := context.Background()
	result, err := client.AddRecord(ctx, 1, req)

	// Assert ErrNotConfigured
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got: %v", err)
	}

	if result != nil {
		t.Fatalf("expected nil result, got: %v", result)
	}
}

func TestStorageClient_DeleteRecord_Success(t *testing.T) {
	// Setup mock server
	server := mockbunny.New()
	defer server.Close()

	// Add a test zone
	zoneID := server.AddZone("example.com")

	// Setup mock KeyStore
	mockKS := &MockKeyStore{
		apiKey: "test-key",
	}

	// Create StorageClient with mock server
	client := NewStorageClient(mockKS, WithStorageClientBaseURL(server.URL()))

	// First add a record so we can delete it
	req := &AddRecordRequest{
		Type:  "A",
		Name:  "test",
		Value: "192.0.2.1",
		TTL:   3600,
	}
	record, err := client.AddRecord(context.Background(), zoneID, req)
	if err != nil {
		t.Fatalf("failed to add record: %v", err)
	}

	// Call DeleteRecord
	ctx := context.Background()
	err = client.DeleteRecord(ctx, zoneID, record.ID)

	// Assert success
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStorageClient_DeleteRecord_KeyNotConfigured(t *testing.T) {
	// Setup mock KeyStore that returns ErrNotFound
	mockKS := &MockKeyStore{
		err: storage.ErrNotFound,
	}

	client := NewStorageClient(mockKS)

	// Call DeleteRecord
	ctx := context.Background()
	err := client.DeleteRecord(ctx, 1, 1)

	// Assert ErrNotConfigured
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got: %v", err)
	}
}

func TestStorageClient_ListZones_WithOptions(t *testing.T) {
	// Setup mock server
	server := mockbunny.New()
	defer server.Close()

	// Add a test zone
	server.AddZone("example.com")

	// Setup mock KeyStore
	mockKS := &MockKeyStore{
		apiKey: "test-key",
	}

	// Create StorageClient with mock server
	client := NewStorageClient(mockKS, WithStorageClientBaseURL(server.URL()))

	// Call ListZones with options
	ctx := context.Background()
	opts := &ListZonesOptions{
		Page:    1,
		PerPage: 10,
		Search:  "example",
	}
	result, err := client.ListZones(ctx, opts)

	// Assert success
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatalf("expected result, got nil")
	}
}

func TestStorageClient_MultipleRequests_FetchesKeyEachTime(t *testing.T) {
	// Setup mock server
	server := mockbunny.New()
	defer server.Close()

	// Add a test zone
	zoneID := server.AddZone("example.com")

	// Setup mock KeyStore that tracks calls
	callCount := 0
	mockKS := &MockKeyStore{
		apiKey: "test-key",
	}

	// Wrap the mock to count calls
	countingKS := &countingKeyStore{
		underlying: mockKS,
		callCount:  &callCount,
	}

	// Create StorageClient with mock server
	client := NewStorageClient(countingKS, WithStorageClientBaseURL(server.URL()))

	// Make multiple requests
	ctx := context.Background()
	_, _ = client.ListZones(ctx, nil)
	_, _ = client.GetZone(ctx, zoneID)
	_, _ = client.AddRecord(ctx, zoneID, &AddRecordRequest{Type: "A", Name: "test", Value: "192.0.2.1"})

	// Verify KeyStore was called 3 times (once per request)
	if callCount != 3 {
		t.Fatalf("expected 3 calls to GetMasterAPIKey, got: %d", callCount)
	}
}

func TestNewStorageClient_WithCustomHTTPClient(t *testing.T) {
	mockKS := &MockKeyStore{
		apiKey: "test-key",
	}

	customHTTPClient := &http.Client{}

	// Create StorageClient with custom HTTP client
	client := NewStorageClient(mockKS, WithStorageClientHTTPClient(customHTTPClient))

	// Verify the client was created
	if client == nil {
		t.Fatalf("expected client, got nil")
	}

	// Verify httpClient is set
	if client.httpClient != customHTTPClient {
		t.Fatalf("expected custom HTTP client, got different client")
	}
}

// countingKeyStore wraps a KeyStore and counts calls to GetMasterAPIKey
type countingKeyStore struct {
	underlying KeyStore
	callCount  *int
}

func (c *countingKeyStore) GetMasterAPIKey(ctx context.Context) (string, error) {
	*c.callCount++
	return c.underlying.GetMasterAPIKey(ctx)
}
