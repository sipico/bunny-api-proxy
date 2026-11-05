//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sipico/bunny-api-proxy/tests/testenv"
)

type recordPage struct {
	CurrentPage  int  `json:"CurrentPage"`
	TotalItems   int  `json:"TotalItems"`
	HasMoreItems bool `json:"HasMoreItems"`
	Items        []struct {
		ID   int64  `json:"Id"`
		Type int    `json:"Type"`
		Name string `json:"Name"`
	} `json:"Items"`
}

func getRecordPage(t *testing.T, key string, zoneID int64, query string) recordPage {
	t.Helper()
	path := fmt.Sprintf("/dnszone/%d/records", zoneID)
	if query != "" {
		path += "?" + query
	}
	resp := proxyRequest(t, "GET", path, key, nil)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var page recordPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	return page
}

// TestE2E_ListRecords_PaginatedPerSpec covers GET /dnszone/{id}/records in
// bunny.net's shape: paging, type and search filters, and that a scoped
// token's TotalItems and pages count only the record types it may see.
func TestE2E_ListRecords_PaginatedPerSpec(t *testing.T) {
	env := testenv.Setup(t)
	zone := env.CreateTestZones(t, 1)[0]
	adminKey := createScopedKeyWithRecordTypes(t, env.AdminToken, zone.ID, []string{"TXT", "A"})

	// 6 TXT records and 2 A records. The first one with PUT, bunny.net's
	// method for adding a record; the rest with POST (kept for older clients).
	for i := 0; i < 6; i++ {
		body, _ := json.Marshal(map[string]interface{}{"Type": 3, "Name": fmt.Sprintf("_acme%d", i), "Value": fmt.Sprintf("v%d", i), "Ttl": 300})
		method := "POST"
		if i == 0 {
			method = "PUT"
		}
		resp := proxyRequest(t, method, fmt.Sprintf("/dnszone/%d/records", zone.ID), adminKey, body)
		resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}
	for i := 0; i < 2; i++ {
		body, _ := json.Marshal(map[string]interface{}{"Type": 0, "Name": fmt.Sprintf("www%d", i), "Value": "192.0.2.1", "Ttl": 300})
		resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), adminKey, body)
		resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	all := getRecordPage(t, adminKey, zone.ID, "")
	require.Equal(t, 8, all.TotalItems)
	require.Len(t, all.Items, 8)
	require.False(t, all.HasMoreItems)
	require.Equal(t, 1, all.CurrentPage)

	p1 := getRecordPage(t, adminKey, zone.ID, "perPage=5")
	require.Len(t, p1.Items, 5)
	require.True(t, p1.HasMoreItems)
	p2 := getRecordPage(t, adminKey, zone.ID, "page=2&perPage=5")
	require.Len(t, p2.Items, 3)
	require.False(t, p2.HasMoreItems)
	require.Equal(t, 2, p2.CurrentPage)

	txt := getRecordPage(t, adminKey, zone.ID, "type=3")
	require.Equal(t, 6, txt.TotalItems)
	search := getRecordPage(t, adminKey, zone.ID, "search=www")
	require.Equal(t, 2, search.TotalItems)

	// A TXT-only token sees only TXT records, and the counts say so.
	txtOnly := createScopedKeyWithRecordTypes(t, env.AdminToken, zone.ID, []string{"TXT"})
	scoped := getRecordPage(t, txtOnly, zone.ID, "perPage=5")
	require.Equal(t, 6, scoped.TotalItems, "TotalItems must count only visible records")
	require.True(t, scoped.HasMoreItems)
	for _, r := range scoped.Items {
		require.Equal(t, 3, r.Type, "TXT-only token must not see type %d (%s)", r.Type, r.Name)
	}
	require.Empty(t, getRecordPage(t, txtOnly, zone.ID, "type=0").Items, "TXT-only token asking for A records")

	resp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records?perPage=1", zone.ID), adminKey, nil)
	resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "perPage below bunny.net's minimum of 5")
}
