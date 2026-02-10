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

// Permission represents access rules for a token.
type Permission struct {
	ID             int64
	TokenID        int64
	ZoneID         int64
	AllowedActions []string // e.g., ["list_records", "add_record", "delete_record"]
	RecordTypes    []string // e.g., ["TXT", "A", "AAAA"]
	CreatedAt      time.Time
}
