package mockbunny

import (
	"net/http"
	"testing"
)

func TestNew(t *testing.T) {
	s := New()
	defer s.Close()

	// Verify server is running
	if s.URL() == "" {
		t.Fatal("expected non-empty URL")
	}

	// Verify URL is accessible
	resp, err := http.Get(s.URL() + "/dnszone")
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	// Should get 501 Not Implemented for now
	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d", resp.StatusCode)
	}
}

func TestServerStructFields(t *testing.T) {
	s := New()
	defer s.Close()

	// Verify Server has access to underlying httptest.Server
	if s.Server == nil {
		t.Error("expected Server field to be non-nil")
	}

	// Verify state is initialized
	if s.state == nil {
		t.Error("expected state field to be non-nil")
	}

	// Verify router is initialized
	if s.router == nil {
		t.Error("expected router field to be non-nil")
	}
}

func TestURLMethod(t *testing.T) {
	s := New()
	defer s.Close()

	url := s.URL()
	if url == "" {
		t.Fatal("URL() returned empty string")
	}

	// URL should be accessible
	resp, err := http.Head(url + "/dnszone")
	if err != nil {
		t.Fatalf("failed to access URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 0 {
		t.Error("expected non-zero status code")
	}
}

func TestCloseMethod(t *testing.T) {
	s := New()
	url := s.URL()

	// Verify server is running
	resp, err := http.Head(url + "/dnszone")
	if err != nil {
		t.Fatalf("failed to connect before close: %v", err)
	}
	resp.Body.Close()

	// Close the server
	s.Close()

	// Verify server is no longer accessible (or returns error)
	// After close, the connection should fail
	_, err = http.Head(url + "/dnszone")
	if err == nil {
		t.Error("expected error after close, but request succeeded")
	}
}

func TestPlaceholderRoutes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{"GET /dnszone", "GET", "/dnszone", http.StatusNotImplemented},
		{"GET /dnszone/{id}", "GET", "/dnszone/123", http.StatusNotImplemented},
		{"PUT /dnszone/{zoneId}/records", "PUT", "/dnszone/456/records", http.StatusNotImplemented},
		{"DELETE /dnszone/{zoneId}/records/{id}", "DELETE", "/dnszone/789/records/321", http.StatusNotImplemented},
	}

	s := New()
	defer s.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, s.URL()+tt.path, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("failed to send request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("expected %d, got %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}
