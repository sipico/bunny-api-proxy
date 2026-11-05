// Package auth handles API key validation and permission checking.
package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"maps"
	"slices"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// HashToken computes the SHA256 hash of a token for storage lookup.
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

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
	// ActionUpdateRecord updates a record in a zone.
	ActionUpdateRecord Action = "update_record"
	// ActionDeleteRecord deletes a record from a zone.
	ActionDeleteRecord Action = "delete_record"
	// ActionCreateZone creates a new DNS zone (admin only).
	ActionCreateZone Action = "create_zone"
	// ActionUpdateZone updates zone-level settings (admin only).
	ActionUpdateZone Action = "update_zone"
	// ActionDeleteZone deletes a DNS zone (admin only).
	ActionDeleteZone Action = "delete_zone"
	// ActionCheckAvailability checks DNS zone availability (admin only).
	ActionCheckAvailability Action = "check_availability"
	// ActionImportRecords imports DNS records (admin only).
	ActionImportRecords Action = "import_records"
	// ActionExportRecords exports DNS records (admin only).
	ActionExportRecords Action = "export_records"
	// ActionEnableDNSSEC enables DNSSEC for a zone (admin only).
	ActionEnableDNSSEC Action = "enable_dnssec"
	// ActionDisableDNSSEC disables DNSSEC for a zone (admin only).
	ActionDisableDNSSEC Action = "disable_dnssec"
	// ActionIssueCertificate issues a wildcard SSL certificate (admin only).
	ActionIssueCertificate Action = "issue_certificate"
	// ActionGetStatistics retrieves DNS query statistics (admin only).
	ActionGetStatistics Action = "get_statistics"
	// ActionTriggerDNSScan triggers a DNS record scan (admin only).
	ActionTriggerDNSScan Action = "trigger_dns_scan"
	// ActionGetDNSScanResult retrieves DNS scan results (admin only).
	ActionGetDNSScanResult Action = "get_dns_scan_result"
)

// ValidActions is the set of actions that can be granted to scoped tokens.
// Admin-only actions are excluded: scoped tokens can never use them, so a
// permission listing one would silently do nothing (issue #392).
var ValidActions = map[string]bool{
	string(ActionListZones):    true,
	string(ActionGetZone):      true,
	string(ActionListRecords):  true,
	string(ActionAddRecord):    true,
	string(ActionUpdateRecord): true,
	string(ActionDeleteRecord): true,
}

// ValidRecordTypes is the set of DNS record type names that can be granted.
// They are exactly the names MapRecordTypeToString produces, which is what
// CheckPermission compares against (case-sensitive), and they follow the
// x-enumNames of DnsRecordTypes in bunny.net's OpenAPI spec.
var ValidRecordTypes = map[string]bool{
	"A":        true,
	"AAAA":     true,
	"CNAME":    true,
	"TXT":      true,
	"MX":       true,
	"Redirect": true,
	"Flatten":  true,
	"PullZone": true,
	"SRV":      true,
	"CAA":      true,
	"PTR":      true,
	"Script":   true,
	"NS":       true,
	"SVCB":     true,
	"HTTPS":    true,
	"TLSA":     true,
}

// DeprecatedRecordTypes are names that older versions accepted but that no
// longer exist in bunny.net's API, with the reason. They can no longer be
// granted. Permissions that still contain them keep them in storage but they
// match nothing (fail closed): there is no silent conversion, because that
// could grant more than the admin intended.
var DeprecatedRecordTypes = map[string]string{
	"SPF": `"SPF" has no effect anymore: bunny.net record type 5 is now "Redirect" (an HTTP redirect, ` +
		`served through an automatically created pull zone). SPF records are TXT records. ` +
		`Grant "TXT" for SPF records, or "Redirect" if redirects are really intended.`,
}

// DeprecatedRecordTypeWarnings returns one warning per deprecated record type
// in types, in the order they appear.
func DeprecatedRecordTypeWarnings(types []string) []string {
	var out []string
	for _, t := range types {
		if why, ok := DeprecatedRecordTypes[t]; ok {
			out = append(out, why)
		}
	}
	return out
}

// ValidateActions returns the first action that is not in ValidActions,
// or "" if all actions are valid.
func ValidateActions(actions []string) string {
	for _, a := range actions {
		if !ValidActions[a] {
			return a
		}
	}
	return ""
}

// ValidateRecordTypes returns the first record type that is not in
// ValidRecordTypes, or "" if all record types are valid.
func ValidateRecordTypes(types []string) string {
	for _, t := range types {
		if !ValidRecordTypes[t] {
			return t
		}
	}
	return ""
}

// ValidActionsList returns the valid actions, sorted, for error messages.
func ValidActionsList() []string {
	return slices.Sorted(maps.Keys(ValidActions))
}

// ValidRecordTypesList returns the valid record types, sorted, for error messages.
func ValidRecordTypesList() []string {
	return slices.Sorted(maps.Keys(ValidRecordTypes))
}

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

// CheckPermission verifies if the key has permission for the request.
func CheckPermission(keyInfo *KeyInfo, req *Request) error {
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

	// add_record and update_record: also check record type
	if req.Action == ActionAddRecord || req.Action == ActionUpdateRecord {
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

// findZonePermission finds the permission entry for a zone.
// First tries exact zone match, then falls back to wildcard (ZoneID=0).
// Returns nil if no matching permission is found.
func findZonePermission(keyInfo *KeyInfo, zoneID int64) *storage.Permission {
	if keyInfo == nil {
		return nil
	}
	// Try exact zone match first
	for _, perm := range keyInfo.Permissions {
		if perm.ZoneID == zoneID {
			return perm
		}
	}
	// Fall back to wildcard (ZoneID=0) if not an exact match
	if zoneID != 0 {
		for _, perm := range keyInfo.Permissions {
			if perm.ZoneID == 0 {
				return perm
			}
		}
	}
	return nil
}

// IsRecordTypePermitted checks if a record type is permitted for a zone.
// Returns true if the type is allowed, or if no RecordTypes restriction exists.
func IsRecordTypePermitted(keyInfo *KeyInfo, zoneID int64, recordType string) bool {
	if keyInfo == nil {
		return false
	}

	zonePerm := findZonePermission(keyInfo, zoneID)
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

	zonePerm := findZonePermission(keyInfo, zoneID)
	if zonePerm == nil {
		return nil
	}

	// Empty RecordTypes means no restriction - return nil
	if len(zonePerm.RecordTypes) == 0 {
		return nil
	}

	return zonePerm.RecordTypes
}
