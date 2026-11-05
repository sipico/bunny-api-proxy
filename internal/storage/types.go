package storage

import "time"

// Token represents a unified token for admin or scoped access.
type Token struct {
	ID int64
	// KeyHash is the SHA-256 hash of the token. SECURITY: only populated by
	// GetTokenByHash (authentication); never include it in API responses.
	KeyHash   string
	Name      string
	IsAdmin   bool
	CreatedAt time.Time
	// ExpiresAt is the optional expiry time; nil means the token never expires.
	ExpiresAt *time.Time
}

// IsExpired reports whether the token has an expiry time that lies before now.
// A token without an expiry never expires.
func (t *Token) IsExpired(now time.Time) bool {
	return t.ExpiresAt != nil && now.After(*t.ExpiresAt)
}

// Permission represents access rules for a token.
type Permission struct {
	ID             int64
	TokenID        int64
	ZoneID         int64
	AllowedActions []string // e.g., ["list_records", "add_record", "delete_record"]
	RecordTypes    []string // e.g., ["TXT", "A", "AAAA"]
	CreatedAt      time.Time
}
