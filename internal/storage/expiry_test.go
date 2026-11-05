package storage

import (
	"context"
	"testing"
	"time"
)

// TestTokenExpiry_RoundTrip verifies that an optional expiry is stored and
// returned by every token query, and that CreateToken means "never expires"
// (issue #400).
func TestTokenExpiry_RoundTrip(t *testing.T) {
	t.Parallel()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()

	expires := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	withExpiry, err := s.CreateTokenWithExpiry(ctx, "expiring", false, hashToken("expiring"), &expires)
	if err != nil {
		t.Fatalf("CreateTokenWithExpiry: %v", err)
	}
	if withExpiry.ExpiresAt == nil || !withExpiry.ExpiresAt.Equal(expires) {
		t.Errorf("created token ExpiresAt = %v, want %v", withExpiry.ExpiresAt, expires)
	}
	noExpiry, err := s.CreateToken(ctx, "forever", false, hashToken("forever"))
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if noExpiry.ExpiresAt != nil {
		t.Errorf("CreateToken ExpiresAt = %v, want nil", noExpiry.ExpiresAt)
	}

	check := func(where string, tok *Token, want *time.Time) {
		t.Helper()
		switch {
		case want == nil && tok.ExpiresAt != nil:
			t.Errorf("%s: ExpiresAt = %v, want nil", where, tok.ExpiresAt)
		case want != nil && (tok.ExpiresAt == nil || !tok.ExpiresAt.Equal(*want)):
			t.Errorf("%s: ExpiresAt = %v, want %v", where, tok.ExpiresAt, *want)
		}
	}

	byHash, err := s.GetTokenByHash(ctx, hashToken("expiring"))
	if err != nil {
		t.Fatalf("GetTokenByHash: %v", err)
	}
	check("GetTokenByHash", byHash, &expires)

	byID, err := s.GetTokenByID(ctx, withExpiry.ID)
	if err != nil {
		t.Fatalf("GetTokenByID: %v", err)
	}
	check("GetTokenByID", byID, &expires)

	foreverByID, err := s.GetTokenByID(ctx, noExpiry.ID)
	if err != nil {
		t.Fatalf("GetTokenByID: %v", err)
	}
	check("GetTokenByID (no expiry)", foreverByID, nil)

	list, err := s.ListTokens(ctx)
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	for _, tok := range list {
		if tok.ID == withExpiry.ID {
			check("ListTokens", tok, &expires)
		} else {
			check("ListTokens (no expiry)", tok, nil)
		}
	}
}

// TestTokenIsExpired verifies the expiry decision, including "never".
func TestTokenIsExpired(t *testing.T) {
	t.Parallel()
	now := time.Now()
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)

	if (&Token{}).IsExpired(now) {
		t.Error("token without expiry must never be expired")
	}
	if !(&Token{ExpiresAt: &past}).IsExpired(now) {
		t.Error("token with past expiry must be expired")
	}
	if (&Token{ExpiresAt: &future}).IsExpired(now) {
		t.Error("token with future expiry must not be expired")
	}
}
