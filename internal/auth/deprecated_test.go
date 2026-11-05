package auth

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestParseRequest_AddRecordWithPUT verifies that PUT, bunny.net's method for
// adding a record, is checked exactly like POST: add_record plus the type.
func TestParseRequest_AddRecordWithPUT(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodPut, http.MethodPost} {
		r := httptest.NewRequest(method, "/dnszone/123/records", strings.NewReader(`{"Type":3,"Name":"_acme-challenge","Value":"x"}`))
		req, err := ParseRequest(r)
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		if req.Action != ActionAddRecord || req.ZoneID != 123 || req.RecordType != "TXT" {
			t.Errorf("%s: parsed %+v", method, req)
		}
		// The body is still readable for the handler.
		if b, _ := io.ReadAll(r.Body); !strings.Contains(string(b), "_acme-challenge") {
			t.Errorf("%s: body not restored", method)
		}
	}
	// A TXT-only token cannot add an A record via PUT.
	key := &KeyInfo{Permissions: []*storage.Permission{{ZoneID: 123, AllowedActions: []string{"add_record"}, RecordTypes: []string{"TXT"}}}}
	req, err := ParseRequest(httptest.NewRequest(http.MethodPut, "/dnszone/123/records", strings.NewReader(`{"Type":0,"Name":"www","Value":"192.0.2.1"}`)))
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckPermission(key, req); !errors.Is(err, ErrForbidden) {
		t.Errorf("PUT of an A record with a TXT-only grant: err = %v, want ErrForbidden", err)
	}
}
