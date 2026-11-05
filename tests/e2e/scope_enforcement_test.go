//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/sipico/bunny-api-proxy/tests/testenv"
	"github.com/stretchr/testify/require"
)

// TestE2E_ScopeEnforcement covers, against mockbunny and the real bunny.net
// API, the four scope bugs the authorization matrix
// (cmd/bunny-api-proxy/authz_matrix_test.go) found:
//
//  1. an admin token saw no zones in GET /dnszone;
//  2. GET /dnszone showed a scoped token every record of its zones, also of
//     types it may not see;
//  3. a scoped token could rewrite a record of another type (POST
//     /dnszone/{id}/records/{recordId} with a body of its own type);
//  4. a scoped token could delete a record of another type.
func TestE2E_ScopeEnforcement(t *testing.T) {
	env := testenv.Setup(t)
	zone := env.CreateTestZones(t, 1)[0]
	records := fmt.Sprintf("/dnszone/%d/records", zone.ID)

	addRecord := func(typ int, name, value string) int64 {
		body, _ := json.Marshal(map[string]interface{}{"Type": typ, "Name": name, "Value": value, "Ttl": 300})
		resp := proxyRequest(t, "PUT", records, env.AdminToken, body)
		defer resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		var rec struct {
			ID int64 `json:"Id"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&rec))
		require.NotZero(t, rec.ID)
		return rec.ID
	}
	txtID := addRecord(3, "_acme-challenge", "scope-test")
	aID := addRecord(0, "scope-www", "192.0.2.10")

	txtOnly := createScopedKeyWithActions(t, env.AdminToken, zone.ID,
		[]string{"list_records", "add_record", "update_record", "delete_record"}, []string{"TXT"})

	type zoneList struct {
		Items []struct {
			ID      int64 `json:"Id"`
			Records []struct {
				ID   int64 `json:"Id"`
				Type int   `json:"Type"`
			} `json:"Records"`
		} `json:"Items"`
	}
	listZones := func(key string) zoneList {
		resp := proxyRequest(t, "GET", "/dnszone?search="+zone.Domain, key, nil)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var l zoneList
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&l))
		return l
	}

	t.Run("admin sees the zone and all its records in GET /dnszone", func(t *testing.T) {
		l := listZones(env.AdminToken)
		var found bool
		for _, z := range l.Items {
			if z.ID == zone.ID {
				found = true
				ids := map[int64]bool{}
				for _, r := range z.Records {
					ids[r.ID] = true
				}
				require.True(t, ids[txtID] && ids[aID], "admin should see the TXT and the A record, got %+v", z.Records)
			}
		}
		require.True(t, found, "admin should see zone %d in GET /dnszone, got %+v", zone.ID, l.Items)
	})

	t.Run("TXT-only token sees only TXT records in GET /dnszone", func(t *testing.T) {
		l := listZones(txtOnly)
		require.Len(t, l.Items, 1, "TXT-only token should see exactly its zone")
		for _, r := range l.Items[0].Records {
			require.Equal(t, 3, r.Type, "TXT-only token sees a record of type %d (Id %d)", r.Type, r.ID)
		}
	})

	t.Run("TXT-only token cannot rewrite the A record", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{"Type": 3, "Name": "scope-www", "Value": "taken-over", "Ttl": 300})
		resp := proxyRequest(t, "POST", fmt.Sprintf("%s/%d", records, aID), txtOnly, body)
		resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("TXT-only token cannot delete the A record", func(t *testing.T) {
		resp := proxyRequest(t, "DELETE", fmt.Sprintf("%s/%d", records, aID), txtOnly, nil)
		resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("the A record is unchanged and the TXT token still manages TXT", func(t *testing.T) {
		resp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d", zone.ID), env.AdminToken, nil)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var z struct {
			Records []struct {
				ID    int64  `json:"Id"`
				Type  int    `json:"Type"`
				Value string `json:"Value"`
			} `json:"Records"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&z))
		var aValue string
		for _, r := range z.Records {
			if r.ID == aID {
				aValue = r.Value
				require.Equal(t, 0, r.Type)
			}
		}
		require.Equal(t, "192.0.2.10", aValue, "the A record must still exist, unchanged")

		del := proxyRequest(t, "DELETE", fmt.Sprintf("%s/%d", records, txtID), txtOnly, nil)
		del.Body.Close()
		require.Equal(t, http.StatusNoContent, del.StatusCode, "TXT-only token deletes its TXT record")
	})
}
