package auth

import (
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// TestGetPermittedZoneIDs_SpecificZones tests extracting specific zone IDs.
func TestGetPermittedZoneIDs_SpecificZones(t *testing.T) {
	t.Parallel()
	keyInfo := &KeyInfo{
		KeyID:   1,
		KeyName: "test-key",
		Permissions: []*storage.Permission{
			{ID: 1, TokenID: 1, ZoneID: 10},
			{ID: 2, TokenID: 1, ZoneID: 20},
			{ID: 3, TokenID: 1, ZoneID: 30},
		},
	}

	zones := GetPermittedZoneIDs(keyInfo)

	if zones == nil {
		t.Fatalf("expected non-nil zones, got nil (this means all zones)")
	}

	if len(zones) != 3 {
		t.Errorf("expected 3 zone IDs, got %d", len(zones))
	}

	// Check that all expected zones are present
	zoneMap := make(map[int64]bool)
	for _, z := range zones {
		zoneMap[z] = true
	}

	expectedZones := []int64{10, 20, 30}
	for _, expected := range expectedZones {
		if !zoneMap[expected] {
			t.Errorf("expected zone ID %d in result", expected)
		}
	}
}

// TestGetPermittedZoneIDs_AllZones tests that ZoneID=0 returns nil (all zones).
func TestGetPermittedZoneIDs_AllZones(t *testing.T) {
	t.Parallel()
	keyInfo := &KeyInfo{
		KeyID:   1,
		KeyName: "test-key",
		Permissions: []*storage.Permission{
			{ID: 1, TokenID: 1, ZoneID: 0}, // All zones
			{ID: 2, TokenID: 1, ZoneID: 10},
		},
	}

	zones := GetPermittedZoneIDs(keyInfo)

	if zones != nil {
		t.Errorf("expected nil (all zones), got %v", zones)
	}
}

// TestGetPermittedZoneIDs_NilKeyInfo tests handling of nil KeyInfo.
func TestGetPermittedZoneIDs_NilKeyInfo(t *testing.T) {
	t.Parallel()
	zones := GetPermittedZoneIDs(nil)

	if zones != nil {
		t.Errorf("expected nil for nil KeyInfo, got %v", zones)
	}
}

// TestGetPermittedZoneIDs_EmptyPermissions tests empty permissions list.
func TestGetPermittedZoneIDs_EmptyPermissions(t *testing.T) {
	t.Parallel()
	keyInfo := &KeyInfo{
		KeyID:       1,
		KeyName:     "test-key",
		Permissions: []*storage.Permission{},
	}

	zones := GetPermittedZoneIDs(keyInfo)

	if zones == nil {
		t.Errorf("expected empty slice, got nil")
	}

	if len(zones) != 0 {
		t.Errorf("expected 0 zone IDs, got %d", len(zones))
	}
}

// TestHasAllZonesPermission tests detection of all zones permission.
func TestHasAllZonesPermission(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		keyInfo  *KeyInfo
		expected bool
	}{
		{
			name:     "has all zones permission",
			expected: true,
			keyInfo: &KeyInfo{
				KeyID:   1,
				KeyName: "test-key",
				Permissions: []*storage.Permission{
					{ID: 1, TokenID: 1, ZoneID: 0},
				},
			},
		},
		{
			name:     "has all zones with other zones",
			expected: true,
			keyInfo: &KeyInfo{
				KeyID:   1,
				KeyName: "test-key",
				Permissions: []*storage.Permission{
					{ID: 1, TokenID: 1, ZoneID: 0},
					{ID: 2, TokenID: 1, ZoneID: 10},
				},
			},
		},
		{
			name:     "only specific zones",
			expected: false,
			keyInfo: &KeyInfo{
				KeyID:   1,
				KeyName: "test-key",
				Permissions: []*storage.Permission{
					{ID: 1, TokenID: 1, ZoneID: 10},
					{ID: 2, TokenID: 1, ZoneID: 20},
				},
			},
		},
		{
			name:     "nil key info",
			expected: false,
			keyInfo:  nil,
		},
		{
			name:     "empty permissions",
			expected: false,
			keyInfo: &KeyInfo{
				KeyID:       1,
				KeyName:     "test-key",
				Permissions: []*storage.Permission{},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := HasAllZonesPermission(tc.keyInfo)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestIsRecordTypePermitted_Allowed tests allowing specific record types.
func TestIsRecordTypePermitted_Allowed(t *testing.T) {
	t.Parallel()
	keyInfo := &KeyInfo{
		KeyID:   1,
		KeyName: "test-key",
		Permissions: []*storage.Permission{
			{
				ID:             1,
				TokenID:        1,
				ZoneID:         10,
				RecordTypes:    []string{"A", "AAAA", "TXT"},
				AllowedActions: []string{"list_records"},
			},
		},
	}

	if !IsRecordTypePermitted(keyInfo, 10, "A") {
		t.Errorf("expected A record to be permitted")
	}

	if !IsRecordTypePermitted(keyInfo, 10, "TXT") {
		t.Errorf("expected TXT record to be permitted")
	}

	if IsRecordTypePermitted(keyInfo, 10, "CNAME") {
		t.Errorf("expected CNAME record to be denied")
	}
}

// TestIsRecordTypePermitted_AllTypes tests allowing all record types.
func TestIsRecordTypePermitted_AllTypes(t *testing.T) {
	t.Parallel()
	keyInfo := &KeyInfo{
		KeyID:   1,
		KeyName: "test-key",
		Permissions: []*storage.Permission{
			{
				ID:             1,
				TokenID:        1,
				ZoneID:         10,
				RecordTypes:    []string{}, // Empty means all types
				AllowedActions: []string{"list_records"},
			},
		},
	}

	if !IsRecordTypePermitted(keyInfo, 10, "A") {
		t.Errorf("expected A record to be permitted")
	}

	if !IsRecordTypePermitted(keyInfo, 10, "TXT") {
		t.Errorf("expected TXT record to be permitted")
	}

	if !IsRecordTypePermitted(keyInfo, 10, "CNAME") {
		t.Errorf("expected CNAME record to be permitted (all types allowed)")
	}
}

// TestIsRecordTypePermitted_WildcardZone tests wildcard zone (ZoneID=0).
func TestIsRecordTypePermitted_WildcardZone(t *testing.T) {
	t.Parallel()
	keyInfo := &KeyInfo{
		KeyID:   1,
		KeyName: "test-key",
		Permissions: []*storage.Permission{
			{
				ID:             1,
				TokenID:        1,
				ZoneID:         0, // Wildcard zone
				RecordTypes:    []string{"TXT"},
				AllowedActions: []string{"list_records"},
			},
		},
	}

	// Any zone with TXT type should be allowed
	if !IsRecordTypePermitted(keyInfo, 10, "TXT") {
		t.Errorf("expected TXT record to be permitted with wildcard zone")
	}

	if !IsRecordTypePermitted(keyInfo, 999, "TXT") {
		t.Errorf("expected TXT record to be permitted for any zone")
	}

	if IsRecordTypePermitted(keyInfo, 10, "A") {
		t.Errorf("expected A record to be denied with wildcard zone")
	}
}

// TestIsRecordTypePermitted_NoPermission tests when zone has no permission.
func TestIsRecordTypePermitted_NoPermission(t *testing.T) {
	t.Parallel()
	keyInfo := &KeyInfo{
		KeyID:   1,
		KeyName: "test-key",
		Permissions: []*storage.Permission{
			{
				ID:             1,
				TokenID:        1,
				ZoneID:         10,
				RecordTypes:    []string{"A"},
				AllowedActions: []string{"list_records"},
			},
		},
	}

	if IsRecordTypePermitted(keyInfo, 999, "A") {
		t.Errorf("expected no permission for zone 999")
	}
}

// TestGetPermittedRecordTypes tests getting record types for a zone.
func TestGetPermittedRecordTypes(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		keyInfo   *KeyInfo
		zoneID    int64
		expected  []string
		expectNil bool
	}{
		{
			name: "specific record types",
			keyInfo: &KeyInfo{
				KeyID:   1,
				KeyName: "test-key",
				Permissions: []*storage.Permission{
					{
						ID:          1,
						TokenID:     1,
						ZoneID:      10,
						RecordTypes: []string{"A", "AAAA", "TXT"},
					},
				},
			},
			zoneID:    10,
			expected:  []string{"A", "AAAA", "TXT"},
			expectNil: false,
		},
		{
			name: "all record types (empty list)",
			keyInfo: &KeyInfo{
				KeyID:   1,
				KeyName: "test-key",
				Permissions: []*storage.Permission{
					{
						ID:          1,
						TokenID:     1,
						ZoneID:      10,
						RecordTypes: []string{}, // Empty means all
					},
				},
			},
			zoneID:    10,
			expected:  nil,
			expectNil: true,
		},
		{
			name: "wildcard zone",
			keyInfo: &KeyInfo{
				KeyID:   1,
				KeyName: "test-key",
				Permissions: []*storage.Permission{
					{
						ID:          1,
						TokenID:     1,
						ZoneID:      0, // Wildcard
						RecordTypes: []string{"TXT"},
					},
				},
			},
			zoneID:    10,
			expected:  []string{"TXT"},
			expectNil: false,
		},
		{
			name:      "nil key info",
			keyInfo:   nil,
			zoneID:    10,
			expected:  nil,
			expectNil: true,
		},
		{
			name: "no permission for zone",
			keyInfo: &KeyInfo{
				KeyID:   1,
				KeyName: "test-key",
				Permissions: []*storage.Permission{
					{
						ID:          1,
						TokenID:     1,
						ZoneID:      10,
						RecordTypes: []string{"A"},
					},
				},
			},
			zoneID:    999,
			expected:  nil,
			expectNil: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := GetPermittedRecordTypes(tc.keyInfo, tc.zoneID)

			if tc.expectNil && result != nil {
				t.Errorf("expected nil, got %v", result)
				return
			}

			if !tc.expectNil && result == nil {
				t.Errorf("expected non-nil result")
				return
			}

			if result != nil && tc.expected != nil {
				if len(result) != len(tc.expected) {
					t.Errorf("expected %d types, got %d", len(tc.expected), len(result))
					return
				}

				// Check each type
				typeMap := make(map[string]bool)
				for _, t := range result {
					typeMap[t] = true
				}

				for _, expected := range tc.expected {
					if !typeMap[expected] {
						t.Errorf("expected type %s in result", expected)
					}
				}
			}
		})
	}
}
