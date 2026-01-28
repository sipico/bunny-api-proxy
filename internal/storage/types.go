package storage

import "time"

// Token represents a unified token for admin or scoped access.
type Token struct {
	ID        int64
	KeyHash   string
	Name      string
	IsAdmin   bool
	CreatedAt time.Time
}

// ScopedKey represents a proxy API key with hashed value.
type ScopedKey struct {
	ID        int64
	KeyHash   string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Permission represents access rules for a token.
type Permission struct {
	ID             int64
	TokenID        int64 // Updated from ScopedKeyID to TokenID for unified schema
	ZoneID         int64
	AllowedActions []string // e.g., ["list_records", "add_record", "delete_record"]
	RecordTypes    []string // e.g., ["TXT", "A", "AAAA"]
	CreatedAt      time.Time
}

// AdminToken represents an admin API token (optional for MVP).
type AdminToken struct {
	ID        int64
	TokenHash string
	Name      string
	CreatedAt time.Time
}
