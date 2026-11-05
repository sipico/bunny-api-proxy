package auth

import (
	"errors"
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// TestSPFPermissionFailsClosed verifies that a stored "SPF" permission grants
// nothing now that bunny.net type 5 is "Redirect": it must not be silently
// treated as "Redirect" (that would grant more than the admin intended).
func TestSPFPermissionFailsClosed(t *testing.T) {
	t.Parallel()
	key := &KeyInfo{KeyID: 2, Permissions: []*storage.Permission{
		{ZoneID: 123, AllowedActions: []string{"add_record", "update_record", "list_records"}, RecordTypes: []string{"SPF"}},
	}}
	for _, action := range []Action{ActionAddRecord, ActionUpdateRecord} {
		err := CheckPermission(key, &Request{Action: action, ZoneID: 123, RecordType: MapRecordTypeToString(5)})
		if !errors.Is(err, ErrForbidden) {
			t.Errorf("%s of a type 5 (Redirect) record with only an SPF grant: err = %v, want ErrForbidden", action, err)
		}
	}

	redirect := &KeyInfo{KeyID: 3, Permissions: []*storage.Permission{
		{ZoneID: 123, AllowedActions: []string{"add_record"}, RecordTypes: []string{"Redirect"}},
	}}
	if err := CheckPermission(redirect, &Request{Action: ActionAddRecord, ZoneID: 123, RecordType: MapRecordTypeToString(5)}); err != nil {
		t.Errorf("an explicit Redirect grant must allow type 5: %v", err)
	}
}

func TestDeprecatedRecordTypes(t *testing.T) {
	t.Parallel()
	if got := ValidateRecordTypes([]string{"TXT", "SPF"}); got != "SPF" {
		t.Errorf("ValidateRecordTypes accepted SPF (got %q)", got)
	}
	if got := ValidateRecordTypes([]string{"Redirect", "SVCB", "HTTPS", "TLSA"}); got != "" {
		t.Errorf("ValidateRecordTypes rejected spec type %q", got)
	}
	if w := DeprecatedRecordTypeWarnings([]string{"TXT", "SPF", "A"}); len(w) != 1 {
		t.Errorf("warnings = %q, want one for SPF", w)
	}
	if w := DeprecatedRecordTypeWarnings([]string{"TXT"}); w != nil {
		t.Errorf("warnings = %q, want none", w)
	}
	// Every name MapRecordTypeToString can produce must be grantable, and no
	// deprecated name may be produced.
	for i := 0; i < 16; i++ {
		name := MapRecordTypeToString(i)
		if !ValidRecordTypes[name] {
			t.Errorf("type %d maps to %q, which cannot be granted", i, name)
		}
		if _, dep := DeprecatedRecordTypes[name]; dep {
			t.Errorf("type %d maps to deprecated name %q", i, name)
		}
	}
	if len(ValidRecordTypes) != 16 {
		t.Errorf("ValidRecordTypes has %d names, want 16 (bunny.net DnsRecordTypes 0-15)", len(ValidRecordTypes))
	}
}
