package admin

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// mockStorageForSession implements Storage interface
type mockStorageForSession struct {
	closeErr error
}

func (m *mockStorageForSession) Close() error {
	return m.closeErr
}

func (m *mockStorageForSession) ValidateAdminToken(ctx context.Context, token string) (*storage.AdminToken, error) {
	return nil, storage.ErrNotFound
}

// TestSessionStoreCreateSession tests session creation
func TestSessionStoreCreateSession(t *testing.T) {
	t.Run("generates unique session IDs", func(t *testing.T) {
		store := NewSessionStore(1 * time.Hour)

		session1, err := store.CreateSession(context.Background())
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		session2, err := store.CreateSession(context.Background())
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		if session1.ID == session2.ID {
			t.Error("expected unique session IDs")
		}

		// Check length (32 bytes = 64 hex chars)
		if len(session1.ID) != 64 {
			t.Errorf("expected session ID length 64, got %d", len(session1.ID))
		}
	})

	t.Run("default timeout 24 hours", func(t *testing.T) {
		store := NewSessionStore(0)
		if store.timeout != 24*time.Hour {
			t.Errorf("expected timeout 24h, got %v", store.timeout)
		}
	})

	t.Run("custom timeout", func(t *testing.T) {
		timeout := 2 * time.Hour
		store := NewSessionStore(timeout)
		if store.timeout != timeout {
			t.Errorf("expected timeout %v, got %v", timeout, store.timeout)
		}
	})
}

// TestSessionStoreGetSession tests session retrieval
func TestSessionStoreGetSession(t *testing.T) {
	t.Run("returns valid session", func(t *testing.T) {
		store := NewSessionStore(1 * time.Hour)

		session, err := store.CreateSession(context.Background())
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		retrieved, ok := store.GetSession(context.Background(), session.ID)
		if !ok {
			t.Fatal("expected session to be found")
		}

		if retrieved.ID != session.ID {
			t.Errorf("expected session ID %s, got %s", session.ID, retrieved.ID)
		}
	})

	t.Run("returns false for invalid ID", func(t *testing.T) {
		store := NewSessionStore(1 * time.Hour)

		_, ok := store.GetSession(context.Background(), "invalid-id")
		if ok {
			t.Error("expected session not found")
		}
	})

	t.Run("returns false for expired session", func(t *testing.T) {
		store := NewSessionStore(1 * time.Millisecond)

		session, err := store.CreateSession(context.Background())
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		// Wait for expiration
		time.Sleep(10 * time.Millisecond)

		_, ok := store.GetSession(context.Background(), session.ID)
		if ok {
			t.Error("expected expired session not found")
		}
	})
}

// TestSessionStoreDeleteSession tests session deletion
func TestSessionStoreDeleteSession(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)

	session, err := store.CreateSession(context.Background())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Delete the session
	store.DeleteSession(context.Background(), session.ID)

	// Verify it's gone
	_, ok := store.GetSession(context.Background(), session.ID)
	if ok {
		t.Error("expected deleted session not found")
	}
}

// TestSessionStoreCleanup tests expired session cleanup
func TestSessionStoreCleanup(t *testing.T) {
	store := NewSessionStore(1 * time.Millisecond)

	// Create a session
	session, err := store.CreateSession(context.Background())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Run cleanup
	store.Cleanup(context.Background())

	// Verify expired session was removed
	store.mu.RLock()
	_, exists := store.sessions[session.ID]
	store.mu.RUnlock()

	if exists {
		t.Error("expected expired session to be cleaned up")
	}
}

// TestHandleLoginValidPassword tests successful login
func TestHandleLoginValidPassword(t *testing.T) {
	// Set ADMIN_PASSWORD environment variable
	testPassword := "secure-password-123"
	oldPassword := os.Getenv("ADMIN_PASSWORD")
	defer os.Setenv("ADMIN_PASSWORD", oldPassword)
	os.Setenv("ADMIN_PASSWORD", testPassword)

	store := NewSessionStore(1 * time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &Handler{
		storage:      &mockStorageForSession{},
		sessionStore: store,
		logger:       logger,
	}

	// Create request
	form := url.Values{}
	form.Set("password", testPassword)
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	h.HandleLogin(w, req)

	// Verify redirect
	if w.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", w.Code)
	}

	// Verify location
	loc := w.Header().Get("Location")
	if loc != "/admin" {
		t.Errorf("expected redirect to /admin, got %s", loc)
	}

	// Verify session cookie
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "admin_session" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("expected session cookie")
	}

	if sessionCookie.HttpOnly != true {
		t.Error("expected cookie to be HttpOnly")
	}

	if sessionCookie.Path != "/admin" {
		t.Errorf("expected cookie path /admin, got %s", sessionCookie.Path)
	}

	if sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSite=Lax, got %v", sessionCookie.SameSite)
	}
}

// TestHandleLoginInvalidPassword tests failed login with wrong password
func TestHandleLoginInvalidPassword(t *testing.T) {
	// Set ADMIN_PASSWORD environment variable
	testPassword := "secure-password-123"
	oldPassword := os.Getenv("ADMIN_PASSWORD")
	defer os.Setenv("ADMIN_PASSWORD", oldPassword)
	os.Setenv("ADMIN_PASSWORD", testPassword)

	store := NewSessionStore(1 * time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &Handler{
		storage:      &mockStorageForSession{},
		sessionStore: store,
		logger:       logger,
	}

	// Create request with wrong password
	form := url.Values{}
	form.Set("password", "wrong-password")
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.168.1.1:12345"

	w := httptest.NewRecorder()
	h.HandleLogin(w, req)

	// Verify unauthorized response
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestHandleLoginMissingPassword tests login without password
func TestHandleLoginMissingPassword(t *testing.T) {
	// Set ADMIN_PASSWORD environment variable
	testPassword := "secure-password-123"
	oldPassword := os.Getenv("ADMIN_PASSWORD")
	defer os.Setenv("ADMIN_PASSWORD", oldPassword)
	os.Setenv("ADMIN_PASSWORD", testPassword)

	store := NewSessionStore(1 * time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &Handler{
		storage:      &mockStorageForSession{},
		sessionStore: store,
		logger:       logger,
	}

	// Create request without password
	form := url.Values{}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	h.HandleLogin(w, req)

	// Verify unauthorized response
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestHandleLoginMethodNotAllowed tests login with GET instead of POST
func TestHandleLoginMethodNotAllowed(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &Handler{
		storage:      &mockStorageForSession{},
		sessionStore: store,
		logger:       logger,
	}

	req := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()
	h.HandleLogin(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

// TestHandleLoginNoAdminPassword tests login when ADMIN_PASSWORD not set
func TestHandleLoginNoAdminPassword(t *testing.T) {
	oldPassword := os.Getenv("ADMIN_PASSWORD")
	defer os.Setenv("ADMIN_PASSWORD", oldPassword)
	os.Unsetenv("ADMIN_PASSWORD")

	store := NewSessionStore(1 * time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &Handler{
		storage:      &mockStorageForSession{},
		sessionStore: store,
		logger:       logger,
	}

	form := url.Values{}
	form.Set("password", "any-password")
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	h.HandleLogin(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// TestHandleLogoutValidSession tests logout with valid session
func TestHandleLogoutValidSession(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &Handler{
		storage:      &mockStorageForSession{},
		sessionStore: store,
		logger:       logger,
	}

	// Create a session
	session, err := store.CreateSession(context.Background())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Create request with session in context
	ctx := WithSessionID(context.Background(), session.ID)
	req := httptest.NewRequest("POST", "/logout", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	h.HandleLogout(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify session was deleted
	_, ok := store.GetSession(context.Background(), session.ID)
	if ok {
		t.Error("expected session to be deleted")
	}

	// Verify cookie was deleted
	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "admin_session" && c.MaxAge == -1 {
			return // Cookie deletion verified
		}
	}
	t.Error("expected session cookie to be deleted")
}

// TestHandleLogoutNoSession tests logout without session
func TestHandleLogoutNoSession(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &Handler{
		storage:      &mockStorageForSession{},
		sessionStore: store,
		logger:       logger,
	}

	// Create request without session in context
	req := httptest.NewRequest("POST", "/logout", nil)
	w := httptest.NewRecorder()

	h.HandleLogout(w, req)

	// Should still return 200 (idempotent)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestHandleLogoutMethodNotAllowed tests logout with GET instead of POST
func TestHandleLogoutMethodNotAllowed(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &Handler{
		storage:      &mockStorageForSession{},
		sessionStore: store,
		logger:       logger,
	}

	req := httptest.NewRequest("GET", "/logout", nil)
	w := httptest.NewRecorder()
	h.HandleLogout(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

// TestSessionMiddlewareValidSession tests middleware with valid session
func TestSessionMiddlewareValidSession(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &Handler{
		storage:      &mockStorageForSession{},
		sessionStore: store,
		logger:       logger,
	}

	// Create a session
	session, err := store.CreateSession(context.Background())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Create a next handler that verifies context
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		// Verify session ID in context
		sessionID, ok := GetSessionID(r.Context())
		if !ok || sessionID != session.ID {
			t.Errorf("expected session ID in context")
		}
	})

	middleware := h.SessionMiddleware(nextHandler)

	// Create request with session cookie
	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{
		Name:  "admin_session",
		Value: session.ID,
	})

	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if !nextCalled {
		t.Error("expected next handler to be called")
	}
}

// TestSessionMiddlewareMissingCookie tests middleware without cookie
func TestSessionMiddlewareMissingCookie(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &Handler{
		storage:      &mockStorageForSession{},
		sessionStore: store,
		logger:       logger,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	})

	middleware := h.SessionMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestSessionMiddlewareInvalidSessionID tests middleware with invalid session ID
func TestSessionMiddlewareInvalidSessionID(t *testing.T) {
	store := NewSessionStore(1 * time.Hour)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &Handler{
		storage:      &mockStorageForSession{},
		sessionStore: store,
		logger:       logger,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	})

	middleware := h.SessionMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{
		Name:  "admin_session",
		Value: "invalid-session-id",
	})

	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestSessionMiddlewareExpiredSession tests middleware with expired session
func TestSessionMiddlewareExpiredSession(t *testing.T) {
	store := NewSessionStore(1 * time.Millisecond)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &Handler{
		storage:      &mockStorageForSession{},
		sessionStore: store,
		logger:       logger,
	}

	// Create a session
	session, err := store.CreateSession(context.Background())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	})

	middleware := h.SessionMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{
		Name:  "admin_session",
		Value: session.ID,
	})

	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}
