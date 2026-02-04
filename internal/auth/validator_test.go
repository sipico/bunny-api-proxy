package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

type mockStorage struct {
	keys        []*storage.ScopedKey
	permissions map[int64][]*storage.Permission
	listErr     error
	permErr     error
}

func (m *mockStorage) ListScopedKeys(ctx context.Context) ([]*storage.ScopedKey, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.keys, nil
}

func (m *mockStorage) GetPermissions(ctx context.Context, keyID int64) ([]*storage.Permission, error) {
	if m.permErr != nil {
		return nil, m.permErr
	}
	return m.permissions[keyID], nil
}

func TestValidateKey_EmptyKey(t *testing.T) {
	t.Parallel()
	v := NewValidator(&mockStorage{})
	ctx := context.Background()

	_, err := v.ValidateKey(ctx, "")
	if !errors.Is(err, ErrMissingKey) {
		t.Errorf("expected ErrMissingKey, got %v", err)
	}
}

func TestValidateKey_InvalidKey(t *testing.T) {
	t.Parallel()
	mock := &mockStorage{
		keys: []*storage.ScopedKey{
			{ID: 1, Name: "test-key", KeyHash: "some-hash"},
		},
		permissions: make(map[int64][]*storage.Permission),
	}
	v := NewValidator(mock)
	ctx := context.Background()

	_, err := v.ValidateKey(ctx, "invalid-key")
	if !errors.Is(err, ErrInvalidKey) {
		t.Errorf("expected ErrInvalidKey, got %v", err)
	}
}

func TestValidateKey_StorageError(t *testing.T) {
	t.Parallel()
	storageErr := errors.New("db error")
	mock := &mockStorage{
		listErr: storageErr,
	}
	v := NewValidator(mock)
	ctx := context.Background()

	_, err := v.ValidateKey(ctx, "some-key")
	if !errors.Is(err, storageErr) {
		t.Errorf("expected storage error, got %v", err)
	}
}

func TestValidateKey_PermissionsError(t *testing.T) {
	t.Parallel()
	permErr := errors.New("perm error")
	hash, _ := storage.HashKey("test-key")
	mock := &mockStorage{
		keys: []*storage.ScopedKey{
			{ID: 1, Name: "test-key", KeyHash: hash},
		},
		permissions: make(map[int64][]*storage.Permission),
		permErr:     permErr,
	}
	v := NewValidator(mock)
	ctx := context.Background()

	_, err := v.ValidateKey(ctx, "test-key")
	if !errors.Is(err, permErr) {
		t.Errorf("expected perm error, got %v", err)
	}
}

func TestValidateKey_ValidKey(t *testing.T) {
	t.Parallel()
	hash, _ := storage.HashKey("valid-key")
	perms := []*storage.Permission{
		{
			ID:             1,
			TokenID:        1,
			ZoneID:         100,
			AllowedActions: []string{"list_records", "add_record"},
			RecordTypes:    []string{"TXT", "A"},
		},
	}
	mock := &mockStorage{
		keys: []*storage.ScopedKey{
			{ID: 1, Name: "key1", KeyHash: hash},
		},
		permissions: map[int64][]*storage.Permission{
			1: perms,
		},
	}
	v := NewValidator(mock)
	ctx := context.Background()

	keyInfo, err := v.ValidateKey(ctx, "valid-key")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if keyInfo.KeyID != 1 {
		t.Errorf("expected KeyID 1, got %d", keyInfo.KeyID)
	}
	if keyInfo.KeyName != "key1" {
		t.Errorf("expected KeyName 'key1', got %s", keyInfo.KeyName)
	}
	if len(keyInfo.Permissions) != 1 {
		t.Errorf("expected 1 permission, got %d", len(keyInfo.Permissions))
	}
}

func TestValidateKey_MultipleKeys(t *testing.T) {
	t.Parallel()
	hash1, _ := storage.HashKey("key1")
	hash2, _ := storage.HashKey("key2")

	perms2 := []*storage.Permission{
		{
			ID:             2,
			TokenID:        2,
			ZoneID:         200,
			AllowedActions: []string{"list_records"},
			RecordTypes:    []string{"CNAME"},
		},
	}

	mock := &mockStorage{
		keys: []*storage.ScopedKey{
			{ID: 1, Name: "first", KeyHash: hash1},
			{ID: 2, Name: "second", KeyHash: hash2},
		},
		permissions: map[int64][]*storage.Permission{
			2: perms2,
		},
	}
	v := NewValidator(mock)
	ctx := context.Background()

	keyInfo, err := v.ValidateKey(ctx, "key2")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if keyInfo.KeyID != 2 {
		t.Errorf("expected KeyID 2, got %d", keyInfo.KeyID)
	}
	if keyInfo.KeyName != "second" {
		t.Errorf("expected KeyName 'second', got %s", keyInfo.KeyName)
	}
}

func TestCheckPermission_ListZonesAllowed(t *testing.T) {
	t.Parallel()
	v := NewValidator(&mockStorage{})

	keyInfo := &KeyInfo{
		KeyID:       1,
		KeyName:     "key1",
		Permissions: []*storage.Permission{},
	}
	req := &Request{
		Action: ActionListZones,
	}

	err := v.CheckPermission(keyInfo, req)
	if err != nil {
		t.Errorf("expected no error for list_zones, got %v", err)
	}
}

func TestCheckPermission_GetZoneAllowed(t *testing.T) {
	t.Parallel()
	v := NewValidator(&mockStorage{})

	keyInfo := &KeyInfo{
		KeyID:   1,
		KeyName: "key1",
		Permissions: []*storage.Permission{
			{
				ID:             1,
				TokenID:        1,
				ZoneID:         100,
				AllowedActions: []string{"list_records"},
				RecordTypes:    []string{"TXT"},
			},
		},
	}
	req := &Request{
		Action: ActionGetZone,
		ZoneID: 100,
	}

	err := v.CheckPermission(keyInfo, req)
	if err != nil {
		t.Errorf("expected no error for get_zone with permission, got %v", err)
	}
}

func TestCheckPermission_GetZoneDenied(t *testing.T) {
	t.Parallel()
	v := NewValidator(&mockStorage{})

	keyInfo := &KeyInfo{
		KeyID:       1,
		KeyName:     "key1",
		Permissions: []*storage.Permission{},
	}
	req := &Request{
		Action: ActionGetZone,
		ZoneID: 100,
	}

	err := v.CheckPermission(keyInfo, req)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden for get_zone without permission, got %v", err)
	}
}

func TestCheckPermission_ListRecordsAllowed(t *testing.T) {
	t.Parallel()
	v := NewValidator(&mockStorage{})

	keyInfo := &KeyInfo{
		KeyID:   1,
		KeyName: "key1",
		Permissions: []*storage.Permission{
			{
				ID:             1,
				TokenID:        1,
				ZoneID:         100,
				AllowedActions: []string{"list_records"},
				RecordTypes:    []string{"TXT"},
			},
		},
	}
	req := &Request{
		Action: ActionListRecords,
		ZoneID: 100,
	}

	err := v.CheckPermission(keyInfo, req)
	if err != nil {
		t.Errorf("expected no error for list_records with action, got %v", err)
	}
}

func TestCheckPermission_ListRecordsDenied(t *testing.T) {
	t.Parallel()
	v := NewValidator(&mockStorage{})

	keyInfo := &KeyInfo{
		KeyID:   1,
		KeyName: "key1",
		Permissions: []*storage.Permission{
			{
				ID:             1,
				TokenID:        1,
				ZoneID:         100,
				AllowedActions: []string{"add_record"},
				RecordTypes:    []string{"TXT"},
			},
		},
	}
	req := &Request{
		Action: ActionListRecords,
		ZoneID: 100,
	}

	err := v.CheckPermission(keyInfo, req)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden for list_records without action, got %v", err)
	}
}

func TestCheckPermission_AddRecordAllowed(t *testing.T) {
	t.Parallel()
	v := NewValidator(&mockStorage{})

	keyInfo := &KeyInfo{
		KeyID:   1,
		KeyName: "key1",
		Permissions: []*storage.Permission{
			{
				ID:             1,
				TokenID:        1,
				ZoneID:         100,
				AllowedActions: []string{"add_record"},
				RecordTypes:    []string{"TXT", "A"},
			},
		},
	}
	req := &Request{
		Action:     ActionAddRecord,
		ZoneID:     100,
		RecordType: "TXT",
	}

	err := v.CheckPermission(keyInfo, req)
	if err != nil {
		t.Errorf("expected no error for add_record with action and type, got %v", err)
	}
}

func TestCheckPermission_AddRecordWrongType(t *testing.T) {
	t.Parallel()
	v := NewValidator(&mockStorage{})

	keyInfo := &KeyInfo{
		KeyID:   1,
		KeyName: "key1",
		Permissions: []*storage.Permission{
			{
				ID:             1,
				TokenID:        1,
				ZoneID:         100,
				AllowedActions: []string{"add_record"},
				RecordTypes:    []string{"TXT"},
			},
		},
	}
	req := &Request{
		Action:     ActionAddRecord,
		ZoneID:     100,
		RecordType: "CNAME",
	}

	err := v.CheckPermission(keyInfo, req)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden for add_record with wrong type, got %v", err)
	}
}

func TestCheckPermission_AddRecordNoAction(t *testing.T) {
	t.Parallel()
	v := NewValidator(&mockStorage{})

	keyInfo := &KeyInfo{
		KeyID:   1,
		KeyName: "key1",
		Permissions: []*storage.Permission{
			{
				ID:             1,
				TokenID:        1,
				ZoneID:         100,
				AllowedActions: []string{"list_records"},
				RecordTypes:    []string{"TXT"},
			},
		},
	}
	req := &Request{
		Action:     ActionAddRecord,
		ZoneID:     100,
		RecordType: "TXT",
	}

	err := v.CheckPermission(keyInfo, req)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden for add_record without action, got %v", err)
	}
}

func TestCheckPermission_DeleteRecordAllowed(t *testing.T) {
	t.Parallel()
	v := NewValidator(&mockStorage{})

	keyInfo := &KeyInfo{
		KeyID:   1,
		KeyName: "key1",
		Permissions: []*storage.Permission{
			{
				ID:             1,
				TokenID:        1,
				ZoneID:         100,
				AllowedActions: []string{"delete_record"},
				RecordTypes:    []string{"TXT"},
			},
		},
	}
	req := &Request{
		Action: ActionDeleteRecord,
		ZoneID: 100,
	}

	err := v.CheckPermission(keyInfo, req)
	if err != nil {
		t.Errorf("expected no error for delete_record with action, got %v", err)
	}
}

func TestCheckPermission_DeleteRecordDenied(t *testing.T) {
	t.Parallel()
	v := NewValidator(&mockStorage{})

	keyInfo := &KeyInfo{
		KeyID:   1,
		KeyName: "key1",
		Permissions: []*storage.Permission{
			{
				ID:             1,
				TokenID:        1,
				ZoneID:         100,
				AllowedActions: []string{"list_records"},
				RecordTypes:    []string{"TXT"},
			},
		},
	}
	req := &Request{
		Action: ActionDeleteRecord,
		ZoneID: 100,
	}

	err := v.CheckPermission(keyInfo, req)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden for delete_record without action, got %v", err)
	}
}

func TestCheckPermission_NoZonePermission(t *testing.T) {
	t.Parallel()
	v := NewValidator(&mockStorage{})

	keyInfo := &KeyInfo{
		KeyID:   1,
		KeyName: "key1",
		Permissions: []*storage.Permission{
			{
				ID:             1,
				TokenID:        1,
				ZoneID:         100,
				AllowedActions: []string{"list_records"},
				RecordTypes:    []string{"TXT"},
			},
		},
	}
	req := &Request{
		Action: ActionListRecords,
		ZoneID: 200,
	}

	err := v.CheckPermission(keyInfo, req)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden for zone without permission, got %v", err)
	}
}
