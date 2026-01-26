package storage

import "time"

// ScopedKey represents a proxy API key with hashed value.
type ScopedKey struct {
	ID        int64
	KeyHash   string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Permission represents access rules for a scoped key.
type Permission struct {
	ID             int64
	ScopedKeyID    int64
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
