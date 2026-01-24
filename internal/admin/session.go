package admin

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"os"
	"sync"
	"time"
)

// Session represents an admin session
type Session struct {
	ID        string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// SessionStore manages admin sessions
type SessionStore struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	timeout  time.Duration
}

// NewSessionStore creates a session store
func NewSessionStore(timeout time.Duration) *SessionStore {
	if timeout == 0 {
		timeout = 24 * time.Hour // Default 24 hours
	}
	return &SessionStore{
		sessions: make(map[string]*Session),
		timeout:  timeout,
	}
}

// CreateSession generates a new session
func (s *SessionStore) CreateSession(ctx context.Context) (*Session, error) {
	// Generate random session ID (32 bytes = 64 hex chars)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(b)

	now := time.Now()
	session := &Session{
		ID:        id,
		CreatedAt: now,
		ExpiresAt: now.Add(s.timeout),
	}

	s.mu.Lock()
	s.sessions[id] = session
	s.mu.Unlock()

	return session, nil
}

// GetSession retrieves a session by ID
func (s *SessionStore) GetSession(ctx context.Context, id string) (*Session, bool) {
	s.mu.RLock()
	session, ok := s.sessions[id]
	s.mu.RUnlock()

	if !ok {
		return nil, false
	}

	// Check expiration
	if time.Now().After(session.ExpiresAt) {
		s.DeleteSession(ctx, id)
		return nil, false
	}

	return session, true
}

// DeleteSession removes a session
func (s *SessionStore) DeleteSession(ctx context.Context, id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// Cleanup removes expired sessions (call periodically)
func (s *SessionStore) Cleanup(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, id)
		}
	}
}

// HandleLogin processes admin login
// POST /admin/login
// Form data: password=<value>
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	password := r.FormValue("password")

	// Get expected password from environment
	expectedPassword := os.Getenv("ADMIN_PASSWORD")
	if expectedPassword == "" {
		h.logger.Error("ADMIN_PASSWORD not configured")
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	// Validate password (constant-time comparison)
	if subtle.ConstantTimeCompare([]byte(password), []byte(expectedPassword)) != 1 {
		h.logger.Warn("failed login attempt", "remote_addr", r.RemoteAddr)
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	// Create session
	session, err := h.sessionStore.CreateSession(r.Context())
	if err != nil {
		h.logger.Error("failed to create session", "error", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    session.ID,
		Path:     "/admin",
		MaxAge:   int(h.sessionStore.timeout.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil, // Secure in production
		SameSite: http.SameSiteLaxMode,
	})

	h.logger.Info("admin login successful", "session_id", session.ID)

	// Redirect to dashboard
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// HandleLogout invalidates the session
// POST /admin/logout
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session ID from context (set by middleware)
	sessionID, ok := GetSessionID(r.Context())
	if ok {
		h.sessionStore.DeleteSession(r.Context(), sessionID)
		h.logger.Info("admin logout", "session_id", sessionID)
	}

	// Delete cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    "",
		Path:     "/admin",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// Return 200 OK
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("Logged out"))
	if err != nil {
		// Write errors are not critical for logout responses
		_ = err
	}
}

// SessionMiddleware validates session cookie
func (h *Handler) SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get session cookie
		cookie, err := r.Cookie("admin_session")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Validate session
		session, ok := h.sessionStore.GetSession(r.Context(), cookie.Value)
		if !ok {
			http.Error(w, "Invalid or expired session", http.StatusUnauthorized)
			return
		}

		// Add session ID to context
		ctx := WithSessionID(r.Context(), session.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
