// Package auth handles API key validation and permission checking.
package auth

import (
	"context"
	"errors"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// Action represents an API operation.
type Action string

const (
	// ActionListZones lists all zones accessible to the key.
	ActionListZones Action = "list_zones"
	// ActionGetZone gets details for a specific zone.
	ActionGetZone Action = "get_zone"
	// ActionListRecords lists records in a zone.
	ActionListRecords Action = "list_records"
	// ActionAddRecord adds a record to a zone.
	ActionAddRecord Action = "add_record"
	// ActionDeleteRecord deletes a record from a zone.
	ActionDeleteRecord Action = "delete_record"
)

// Errors for authentication and authorization failures.
var (
	// ErrMissingKey indicates no API key was provided.
	ErrMissingKey = errors.New("auth: missing API key")
	// ErrInvalidKey indicates the API key is not valid.
	ErrInvalidKey = errors.New("auth: invalid API key")
	// ErrForbidden indicates the key lacks required permissions.
	ErrForbidden = errors.New("auth: permission denied")
)

// Request represents a parsed API request.
type Request struct {
	Action     Action
	ZoneID     int64  // 0 for list_zones
	RecordType string // Only for add_record
}

// KeyInfo contains validated key information.
type KeyInfo struct {
	KeyID       int64
	KeyName     string
	Permissions []*storage.Permission
}

// Storage interface for dependency injection.
type Storage interface {
	ListScopedKeys(ctx context.Context) ([]*storage.ScopedKey, error)
	GetPermissions(ctx context.Context, scopedKeyID int64) ([]*storage.Permission, error)
}

// Validator handles key validation and permission checking.
type Validator struct {
	storage Storage
}

// NewValidator creates a new Validator.
func NewValidator(s Storage) *Validator {
	return &Validator{storage: s}
}

// ValidateKey checks if the API key is valid.
// Returns KeyInfo if valid, error if invalid.
func (v *Validator) ValidateKey(ctx context.Context, apiKey string) (*KeyInfo, error) {
	if apiKey == "" {
		return nil, ErrMissingKey
	}

	keys, err := v.storage.ListScopedKeys(ctx)
	if err != nil {
		return nil, err
	}

	// Must iterate all keys - bcrypt hashes are not comparable directly
	for _, key := range keys {
		if storage.VerifyKey(apiKey, key.KeyHash) == nil {
			// Found match - load permissions
			perms, err := v.storage.GetPermissions(ctx, key.ID)
			if err != nil {
				return nil, err
			}
			return &KeyInfo{
				KeyID:       key.ID,
				KeyName:     key.Name,
				Permissions: perms,
			}, nil
		}
	}

	return nil, ErrInvalidKey
}

// CheckPermission verifies if the key has permission for the request.
func (v *Validator) CheckPermission(keyInfo *KeyInfo, req *Request) error {
	// list_zones: always allowed if key is valid
	if req.Action == ActionListZones {
		return nil
	}

	// Find permission for this zone
	var zonePerm *storage.Permission
	for _, p := range keyInfo.Permissions {
		if p.ZoneID == req.ZoneID {
			zonePerm = p
			break
		}
	}

	if zonePerm == nil {
		return ErrForbidden
	}

	// get_zone: allowed if any permission exists for zone
	if req.Action == ActionGetZone {
		return nil
	}

	// Check if action is in allowed actions
	actionAllowed := false
	for _, a := range zonePerm.AllowedActions {
		if a == string(req.Action) {
			actionAllowed = true
			break
		}
	}

	if !actionAllowed {
		return ErrForbidden
	}

	// add_record: also check record type
	if req.Action == ActionAddRecord {
		typeAllowed := false
		for _, t := range zonePerm.RecordTypes {
			if t == req.RecordType {
				typeAllowed = true
				break
			}
		}
		if !typeAllowed {
			return ErrForbidden
		}
	}

	return nil
}

// GetPermittedZoneIDs returns the zone IDs that the key has permission for.
// If any permission has ZoneID = 0 (all zones), returns nil (meaning "all zones").
func GetPermittedZoneIDs(keyInfo *KeyInfo) []int64 {
	if keyInfo == nil {
		return nil
	}

	// Check if key has "all zones" permission (ZoneID = 0)
	for _, perm := range keyInfo.Permissions {
		if perm.ZoneID == 0 {
			return nil // nil means "all zones"
		}
	}

	// Collect all specific zone IDs
	zoneIDs := make([]int64, 0, len(keyInfo.Permissions))
	for _, perm := range keyInfo.Permissions {
		zoneIDs = append(zoneIDs, perm.ZoneID)
	}
	return zoneIDs
}

// HasAllZonesPermission returns true if the key has permission for all zones (ZoneID = 0).
func HasAllZonesPermission(keyInfo *KeyInfo) bool {
	if keyInfo == nil {
		return false
	}

	for _, perm := range keyInfo.Permissions {
		if perm.ZoneID == 0 {
			return true
		}
	}
	return false
}

// IsRecordTypePermitted checks if a record type is permitted for a zone.
// Returns true if the type is allowed, or if no RecordTypes restriction exists.
func IsRecordTypePermitted(keyInfo *KeyInfo, zoneID int64, recordType string) bool {
	if keyInfo == nil {
		return false
	}

	// Find the permission for this zone
	var zonePerm *storage.Permission

	// First, try to find an exact zone match
	for _, perm := range keyInfo.Permissions {
		if perm.ZoneID == zoneID {
			zonePerm = perm
			break
		}
	}

	// If no exact match and zone is not 0, try the wildcard (ZoneID = 0)
	if zonePerm == nil && zoneID != 0 {
		for _, perm := range keyInfo.Permissions {
			if perm.ZoneID == 0 {
				zonePerm = perm
				break
			}
		}
	}

	if zonePerm == nil {
		return false
	}

	// Empty RecordTypes means no restriction - all types allowed
	if len(zonePerm.RecordTypes) == 0 {
		return true
	}

	// Check if recordType is in the allowed list
	for _, t := range zonePerm.RecordTypes {
		if t == recordType {
			return true
		}
	}
	return false
}

// GetPermittedRecordTypes returns the allowed record types for a zone.
// Returns nil if all types are permitted (empty RecordTypes list means no restriction).
func GetPermittedRecordTypes(keyInfo *KeyInfo, zoneID int64) []string {
	if keyInfo == nil {
		return nil
	}

	// Find the permission for this zone
	var zonePerm *storage.Permission

	// First, try to find an exact zone match
	for _, perm := range keyInfo.Permissions {
		if perm.ZoneID == zoneID {
			zonePerm = perm
			break
		}
	}

	// If no exact match and zone is not 0, try the wildcard (ZoneID = 0)
	if zonePerm == nil && zoneID != 0 {
		for _, perm := range keyInfo.Permissions {
			if perm.ZoneID == 0 {
				zonePerm = perm
				break
			}
		}
	}

	if zonePerm == nil {
		return nil
	}

	// Empty RecordTypes means no restriction - return nil
	if len(zonePerm.RecordTypes) == 0 {
		return nil
	}

	return zonePerm.RecordTypes
}
