package proxy

import (
	"testing"

	"github.com/sipico/bunny-api-proxy/internal/auth"
	"github.com/sipico/bunny-api-proxy/internal/bunny"
	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// TestFilterRecords_SPFGrantHidesRedirect verifies that a stored "SPF" grant
// does not reveal type 5 records, which bunny.net now calls "Redirect".
func TestFilterRecords_SPFGrantHidesRedirect(t *testing.T) {
	t.Parallel()
	records := []bunny.Record{{ID: 1, Type: 3, Name: "txt"}, {ID: 2, Type: 5, Name: "redirect"}, {ID: 3, Type: 15, Name: "tlsa"}}
	key := func(types ...string) *auth.KeyInfo {
		return &auth.KeyInfo{Permissions: []*storage.Permission{{ZoneID: 9, RecordTypes: types}}}
	}

	if got := filterRecordsByPermission(records, key("TXT", "SPF"), 9); len(got) != 1 || got[0].ID != 1 {
		t.Errorf("TXT+SPF grant sees %v, want only the TXT record", got)
	}
	if got := filterRecordsByPermission(records, key("Redirect", "TLSA"), 9); len(got) != 2 || got[0].ID != 2 || got[1].ID != 3 {
		t.Errorf("Redirect+TLSA grant sees %v, want records 2 and 3", got)
	}
}
