package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestGetPort(t *testing.T) {
	// Test that getPort() respects PORT environment variable
	tests := []struct {
		name     string
		port     string
		expected string
	}{
		{"default port when not set", "", "8081"},
		{"custom port 9000", "9000", "9000"},
		{"custom port 3000", "3000", "3000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.port == "" {
				os.Unsetenv("PORT")
			} else {
				os.Setenv("PORT", tt.port)
			}
			defer os.Unsetenv("PORT")

			port := getPort()
			if port != tt.expected {
				t.Errorf("expected port %s, got %s", tt.expected, port)
			}
		})
	}
}

func TestCreateServer(t *testing.T) {
	// Test that createServer() creates a server with Handler() method
	server := createServer()
	defer server.Close()

	// Verify the server has the Handler method
	handler := server.Handler()
	if handler == nil {
		t.Error("expected Handler() to return non-nil handler")
	}
}

func TestGetPortAddr(t *testing.T) {
	// Test that getPortAddr() formats the port correctly
	tests := []struct {
		port     string
		expected string
	}{
		{"8081", ":8081"},
		{"9000", ":9000"},
		{"3000", ":3000"},
	}

	for _, tt := range tests {
		t.Run(tt.port, func(t *testing.T) {
			addr := getPortAddr(tt.port)
			if addr != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, addr)
			}
		})
	}
}

func TestCreateHTTPServer(t *testing.T) {
	// Test that createHTTPServer() creates a properly configured http.Server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	httpServer := createHTTPServer("8081", handler)

	// Verify server is properly configured
	if httpServer.Addr != ":8081" {
		t.Errorf("expected Addr to be :8081, got %s", httpServer.Addr)
	}

	if httpServer.Handler == nil {
		t.Error("expected Handler to be non-nil")
	}
}

func TestHandlerIsHTTPHandler(t *testing.T) {
	// Test that the handler returned from mockbunny.New().Handler()
	// is a valid http.Handler that responds to requests
	server := createServer()
	defer server.Close()

	handler := server.Handler()

	// Test with httptest
	req := httptest.NewRequest(http.MethodGet, "/dnszone", nil)
	w := httptest.NewRecorder()

	// Should not panic and should respond
	handler.ServeHTTP(w, req)

	if w.Code == 0 {
		t.Error("expected handler to write a response")
	}
}

func TestAdminStateEndpoint(t *testing.T) {
	// Test that the /admin/state endpoint (used by health check) responds
	server := createServer()
	defer server.Close()

	handler := server.Handler()

	req := httptest.NewRequest(http.MethodGet, "/admin/state", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Should return JSON
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
}

func TestDNSZoneEndpoint(t *testing.T) {
	// Test that the /dnszone endpoint responds
	server := createServer()
	defer server.Close()

	handler := server.Handler()

	req := httptest.NewRequest(http.MethodGet, "/dnszone", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return a response (200 for empty list)
	if w.Code < 200 || w.Code >= 300 {
		t.Errorf("expected 2xx status, got %d", w.Code)
	}
}

func TestHTTPServerPortConfiguration(t *testing.T) {
	// Test different port configurations
	tests := []struct {
		port string
		addr string
	}{
		{"8081", ":8081"},
		{"9000", ":9000"},
		{"3000", ":3000"},
	}

	for _, tt := range tests {
		t.Run(tt.port, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			httpServer := createHTTPServer(tt.port, handler)

			if httpServer.Addr != tt.addr {
				t.Errorf("expected Addr to be %s, got %s", tt.addr, httpServer.Addr)
			}
		})
	}
}

func TestServerHandlerIntegration(t *testing.T) {
	// Test that a server created by createServer() works with
	// an HTTP server created by createHTTPServer()
	port := getPort()
	server := createServer()
	defer server.Close()

	handler := server.Handler()
	httpServer := createHTTPServer(port, handler)

	// Verify integration
	if httpServer.Handler != handler {
		t.Error("expected HTTP server to have the correct handler")
	}

	// Test a request through the handler
	req := httptest.NewRequest(http.MethodGet, "/admin/state", nil)
	w := httptest.NewRecorder()

	httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func BenchmarkGetPort(b *testing.B) {
	os.Setenv("PORT", "8081")
	defer os.Unsetenv("PORT")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getPort()
	}
}

func TestSetupShutdownHandler(t *testing.T) {
	// Test that setupShutdownHandler() creates a channel for shutdown signaling
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	httpServer := createHTTPServer("8081", handler)
	done := setupShutdownHandler(httpServer)

	// Verify done channel is created
	if done == nil {
		t.Error("expected setupShutdownHandler to return a non-nil channel")
	}

	// Verify channel is readable (won't block on write attempt)
	select {
	case <-done:
		t.Error("expected done channel to be empty initially")
	default:
		// Expected: channel is empty
	}
}

func BenchmarkCreateServer(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server := createServer()
		server.Close()
	}
}

func BenchmarkGetPortAddr(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getPortAddr("8081")
	}
}
