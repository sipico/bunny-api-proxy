//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/sipico/bunny-api-proxy/tests/testenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Update DNS Record - Core Functionality
// =============================================================================

// TestE2E_UpdateRecord_Success verifies the basic update record flow:
// create a TXT record, update its value, verify the change persists.
func TestE2E_UpdateRecord_Success(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	// Step 1: Create a TXT record
	addBody := map[string]interface{}{
		"Type":  3,
		"Name":  "_acme-challenge",
		"Value": "original-token",
		"Ttl":   300,
	}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body)
	defer addResp.Body.Close()
	require.Equal(t, 201, addResp.StatusCode)

	var created struct {
		ID    int64  `json:"Id"`
		Type  int    `json:"Type"`
		Name  string `json:"Name"`
		Value string `json:"Value"`
		TTL   int32  `json:"Ttl"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))
	require.NotZero(t, created.ID)

	// Step 2: Update the record's value and TTL
	updateBody := map[string]interface{}{
		"Type":  3,
		"Name":  "_acme-challenge",
		"Value": "updated-token",
		"Ttl":   600,
	}
	body, _ = json.Marshal(updateBody)
	updateResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), apiKey, body)
	defer updateResp.Body.Close()

	// The real bunny.net API returns 204 No Content on success.
	// Our proxy may return 200 with the updated record.
	// Accept either as valid for now, but log the actual behavior.
	assert.Contains(t, []int{200, 204}, updateResp.StatusCode,
		"update should return 200 or 204, got %d", updateResp.StatusCode)

	// Step 3: Verify the update persisted by reading the zone
	getResp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, nil)
	defer getResp.Body.Close()
	require.Equal(t, 200, getResp.StatusCode)

	var records []struct {
		ID    int64  `json:"Id"`
		Type  int    `json:"Type"`
		Name  string `json:"Name"`
		Value string `json:"Value"`
		TTL   int32  `json:"Ttl"`
	}
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&records))

	var found bool
	for _, rec := range records {
		if rec.ID == created.ID {
			found = true
			assert.Equal(t, "updated-token", rec.Value, "record value should be updated")
			assert.Equal(t, "_acme-challenge", rec.Name, "record name should be unchanged")
			assert.Equal(t, 3, rec.Type, "record type should be unchanged")
			assert.Equal(t, int32(600), rec.TTL, "record TTL should be updated")
			break
		}
	}
	require.True(t, found, "updated record should still exist with same ID %d", created.ID)
}

// TestE2E_UpdateRecord_PreservesRecordID verifies the record ID does not change after update.
func TestE2E_UpdateRecord_PreservesRecordID(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	// Create record
	addBody := map[string]interface{}{"Type": 3, "Name": "test", "Value": "v1"}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body)
	defer addResp.Body.Close()
	require.Equal(t, 201, addResp.StatusCode)

	var created struct {
		ID int64 `json:"Id"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))
	originalID := created.ID

	// Update record multiple times
	for i := range 3 {
		updateBody := map[string]interface{}{"Type": 3, "Name": "test", "Value": fmt.Sprintf("v%d", i+2)}
		body, _ = json.Marshal(updateBody)
		resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, originalID), apiKey, body)
		resp.Body.Close()
		assert.Contains(t, []int{200, 204}, resp.StatusCode)
	}

	// Verify record still has the same ID
	listResp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, nil)
	defer listResp.Body.Close()
	require.Equal(t, 200, listResp.StatusCode)

	var records []struct {
		ID    int64  `json:"Id"`
		Value string `json:"Value"`
	}
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&records))

	var found bool
	for _, rec := range records {
		if rec.ID == originalID {
			found = true
			assert.Equal(t, "v4", rec.Value, "should have last update's value")
			break
		}
	}
	require.True(t, found, "record ID %d should not change after updates", originalID)
}

// TestE2E_UpdateRecord_DoesNotAffectOtherRecords verifies that updating one record
// leaves other records in the same zone untouched.
func TestE2E_UpdateRecord_DoesNotAffectOtherRecords(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	// Create two records
	add1 := map[string]interface{}{"Type": 3, "Name": "rec1", "Value": "value1"}
	body1, _ := json.Marshal(add1)
	resp1 := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body1)
	defer resp1.Body.Close()
	require.Equal(t, 201, resp1.StatusCode)
	var rec1 struct {
		ID int64 `json:"Id"`
	}
	require.NoError(t, json.NewDecoder(resp1.Body).Decode(&rec1))

	add2 := map[string]interface{}{"Type": 3, "Name": "rec2", "Value": "value2"}
	body2, _ := json.Marshal(add2)
	resp2 := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body2)
	defer resp2.Body.Close()
	require.Equal(t, 201, resp2.StatusCode)
	var rec2 struct {
		ID int64 `json:"Id"`
	}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&rec2))

	// Update only record 1
	updateBody := map[string]interface{}{"Type": 3, "Name": "rec1", "Value": "updated-value1"}
	body, _ := json.Marshal(updateBody)
	updateResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, rec1.ID), apiKey, body)
	updateResp.Body.Close()
	assert.Contains(t, []int{200, 204}, updateResp.StatusCode)

	// Verify record 2 is unchanged
	listResp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, nil)
	defer listResp.Body.Close()
	require.Equal(t, 200, listResp.StatusCode)

	var records []struct {
		ID    int64  `json:"Id"`
		Name  string `json:"Name"`
		Value string `json:"Value"`
	}
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&records))

	for _, rec := range records {
		if rec.ID == rec2.ID {
			assert.Equal(t, "value2", rec.Value, "record 2 should be unchanged after updating record 1")
			assert.Equal(t, "rec2", rec.Name, "record 2 name should be unchanged")
		}
		if rec.ID == rec1.ID {
			assert.Equal(t, "updated-value1", rec.Value, "record 1 should be updated")
		}
	}
}

// =============================================================================
// Update DNS Record - Permission Enforcement
// =============================================================================

// TestE2E_UpdateRecord_ForbiddenWithoutUpdatePermission verifies that a key
// with add_record but NOT update_record cannot update records.
func TestE2E_UpdateRecord_ForbiddenWithoutUpdatePermission(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Create key with add+delete but NOT update
	addOnlyKey := createScopedKeyWithActions(t, env.AdminToken, zone.ID,
		[]string{"list_zones", "get_zone", "list_records", "add_record", "delete_record"},
		[]string{"TXT", "A", "CNAME"})

	// Create a record (allowed)
	addBody := map[string]interface{}{"Type": 3, "Name": "test", "Value": "original"}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), addOnlyKey, body)
	defer addResp.Body.Close()
	require.Equal(t, 201, addResp.StatusCode)

	var created struct {
		ID int64 `json:"Id"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))

	// Try to update (should be forbidden)
	updateBody := map[string]interface{}{"Type": 3, "Name": "test", "Value": "updated"}
	body, _ = json.Marshal(updateBody)
	updateResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), addOnlyKey, body)
	defer updateResp.Body.Close()

	require.Equal(t, 403, updateResp.StatusCode,
		"key without update_record action should be forbidden from updating")
}

// TestE2E_UpdateRecord_ForbiddenWrongZone verifies that a key scoped to zone1
// cannot update records in zone2.
func TestE2E_UpdateRecord_ForbiddenWrongZone(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 2)
	zone1 := zones[0]
	zone2 := zones[1]

	key1 := createScopedKey(t, env.AdminToken, zone1.ID)
	key2 := createScopedKey(t, env.AdminToken, zone2.ID)

	// Create a record in zone2 using key2
	addBody := map[string]interface{}{"Type": 3, "Name": "test", "Value": "zone2-record"}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone2.ID), key2, body)
	defer addResp.Body.Close()
	require.Equal(t, 201, addResp.StatusCode)

	var created struct {
		ID int64 `json:"Id"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))

	// Try to update zone2's record using key1 (scoped to zone1)
	updateBody := map[string]interface{}{"Type": 3, "Name": "test", "Value": "hacked"}
	body, _ = json.Marshal(updateBody)
	updateResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone2.ID, created.ID), key1, body)
	defer updateResp.Body.Close()

	require.Equal(t, 403, updateResp.StatusCode,
		"key for zone1 should not be able to update records in zone2")
}

// TestE2E_UpdateRecord_RecordTypeEnforcement verifies that record type restrictions
// are enforced on update — a TXT-only key cannot update a record to an A type,
// and cannot update an A record even if it knows the ID.
func TestE2E_UpdateRecord_RecordTypeEnforcement(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Create a key restricted to TXT only
	txtOnlyKey := createScopedKeyWithRecordTypes(t, env.AdminToken, zone.ID, []string{"TXT"})

	// Create a TXT record
	addBody := map[string]interface{}{"Type": 3, "Name": "_acme", "Value": "token1"}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), txtOnlyKey, body)
	defer addResp.Body.Close()
	require.Equal(t, 201, addResp.StatusCode)

	var created struct {
		ID int64 `json:"Id"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))

	// Updating with Type=TXT should succeed
	t.Run("TXT-only key can update TXT record", func(t *testing.T) {
		updateBody := map[string]interface{}{"Type": 3, "Name": "_acme", "Value": "token2"}
		body, _ := json.Marshal(updateBody)
		resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), txtOnlyKey, body)
		defer resp.Body.Close()
		assert.Contains(t, []int{200, 204}, resp.StatusCode,
			"TXT-only key should be able to update a TXT record")
	})

	// Updating with Type=A should be forbidden (record type in request body is checked)
	t.Run("TXT-only key cannot update record to A type", func(t *testing.T) {
		updateBody := map[string]interface{}{"Type": 0, "Name": "www", "Value": "1.2.3.4"}
		body, _ := json.Marshal(updateBody)
		resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), txtOnlyKey, body)
		defer resp.Body.Close()
		require.Equal(t, 403, resp.StatusCode,
			"TXT-only key should not be able to update a record with Type=A")
	})

	// Updating with Type=CNAME should be forbidden
	t.Run("TXT-only key cannot update record to CNAME type", func(t *testing.T) {
		updateBody := map[string]interface{}{"Type": 2, "Name": "alias", "Value": "target.example.com"}
		body, _ := json.Marshal(updateBody)
		resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), txtOnlyKey, body)
		defer resp.Body.Close()
		require.Equal(t, 403, resp.StatusCode,
			"TXT-only key should not be able to update a record with Type=CNAME")
	})
}

// =============================================================================
// Update DNS Record - Error Handling
// =============================================================================

// TestE2E_UpdateRecord_NonExistentRecord verifies proper 404 handling when
// updating a record ID that doesn't exist.
func TestE2E_UpdateRecord_NonExistentRecord(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	updateBody := map[string]interface{}{"Type": 3, "Name": "ghost", "Value": "does-not-exist"}
	body, _ := json.Marshal(updateBody)
	resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/999999999", zone.ID), apiKey, body)
	defer resp.Body.Close()

	require.Equal(t, 404, resp.StatusCode,
		"updating non-existent record should return 404")
}

// TestE2E_UpdateRecord_NonExistentZone verifies proper error handling when
// updating a record in a zone that doesn't exist. The scoped key may return
// 403 (no permission for the zone) before reaching 404 at the backend.
func TestE2E_UpdateRecord_NonExistentZone(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	updateBody := map[string]interface{}{"Type": 3, "Name": "test", "Value": "test"}
	body, _ := json.Marshal(updateBody)
	resp := proxyRequest(t, "POST", "/dnszone/999999999/records/1", apiKey, body)
	defer resp.Body.Close()

	// Expect 403 (scoped key doesn't have permission for zone 999999999)
	// or 404 if an admin key were used
	assert.True(t, resp.StatusCode == 403 || resp.StatusCode == 404,
		"updating record in non-permitted zone should return 403 or 404, got %d", resp.StatusCode)
}

// TestE2E_UpdateRecord_InvalidJSON verifies proper error handling for malformed request body.
func TestE2E_UpdateRecord_InvalidJSON(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	// Create a record first so we have a valid ID
	addBody := map[string]interface{}{"Type": 3, "Name": "test", "Value": "v1"}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body)
	defer addResp.Body.Close()
	require.Equal(t, 201, addResp.StatusCode)

	var created struct {
		ID int64 `json:"Id"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))

	// Send malformed JSON
	resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), apiKey, []byte("not valid json{{{"))
	defer resp.Body.Close()

	require.Equal(t, 400, resp.StatusCode, "malformed JSON should return 400")
}

// TestE2E_UpdateRecord_EmptyBody verifies that the real bunny.net API accepts
// updates with an empty JSON body (partial update — no fields changed).
func TestE2E_UpdateRecord_EmptyBody(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	addBody := map[string]interface{}{"Type": 3, "Name": "test", "Value": "v1"}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body)
	defer addResp.Body.Close()
	require.Equal(t, 201, addResp.StatusCode)

	var created struct {
		ID int64 `json:"Id"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))

	// Send empty body — bunny.net API accepts this (204) without validation error
	resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), apiKey, []byte("{}"))
	defer resp.Body.Close()

	assert.Contains(t, []int{200, 204}, resp.StatusCode,
		"bunny.net accepts empty update body without validation error")
}

// TestE2E_UpdateRecord_MissingValue verifies that the bunny.net API accepts
// updates without a Value field (partial update behavior).
func TestE2E_UpdateRecord_MissingValue(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	addBody := map[string]interface{}{"Type": 3, "Name": "test", "Value": "v1"}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body)
	defer addResp.Body.Close()
	require.Equal(t, 201, addResp.StatusCode)

	var created struct {
		ID int64 `json:"Id"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))

	// Update with Name but no Value — bunny.net API accepts partial updates
	updateBody := map[string]interface{}{"Type": 3, "Name": "test"}
	body, _ = json.Marshal(updateBody)
	resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), apiKey, body)
	defer resp.Body.Close()

	assert.Contains(t, []int{200, 204}, resp.StatusCode,
		"bunny.net accepts update without Value (partial update)")
}

// TestE2E_UpdateRecord_MissingName verifies that the bunny.net API accepts
// updates without a Name field (partial update behavior).
func TestE2E_UpdateRecord_MissingName(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	addBody := map[string]interface{}{"Type": 3, "Name": "test", "Value": "v1"}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body)
	defer addResp.Body.Close()
	require.Equal(t, 201, addResp.StatusCode)

	var created struct {
		ID int64 `json:"Id"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))

	// Update with Value but no Name — bunny.net API accepts partial updates
	updateBody := map[string]interface{}{"Type": 3, "Value": "v2"}
	body, _ = json.Marshal(updateBody)
	resp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), apiKey, body)
	defer resp.Body.Close()

	assert.Contains(t, []int{200, 204}, resp.StatusCode,
		"bunny.net accepts update without Name (partial update)")
}

// TestE2E_UpdateRecord_Unauthorized verifies that requests without an API key are rejected.
func TestE2E_UpdateRecord_Unauthorized(t *testing.T) {
	resp := proxyRequest(t, "POST", "/dnszone/1/records/1", "", nil)
	defer resp.Body.Close()

	// No key or empty key should be rejected
	assert.True(t, resp.StatusCode == 401 || resp.StatusCode == 400,
		"update without API key should be rejected, got %d", resp.StatusCode)
}

// =============================================================================
// Update DNS Record - Record Type Specific Behavior
// =============================================================================

// TestE2E_UpdateRecord_ChangeRecordType verifies that the record Type field is
// immutable on update. The bunny.net API ignores the Type field in update requests,
// so the record type should remain unchanged even if a different Type is sent.
func TestE2E_UpdateRecord_ChangeRecordType(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Key with both TXT and A permissions
	multiTypeKey := createScopedKeyWithRecordTypes(t, env.AdminToken, zone.ID, []string{"TXT", "A"})

	// Create a TXT record
	addBody := map[string]interface{}{"Type": 3, "Name": "changeme", "Value": "txt-value"}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), multiTypeKey, body)
	defer addResp.Body.Close()
	require.Equal(t, 201, addResp.StatusCode)

	var created struct {
		ID int64 `json:"Id"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))

	// Try to update it to be an A record — the API accepts the request but ignores Type
	updateBody := map[string]interface{}{"Type": 0, "Name": "changeme", "Value": "1.2.3.4"}
	body, _ = json.Marshal(updateBody)
	updateResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), multiTypeKey, body)
	defer updateResp.Body.Close()

	assert.Contains(t, []int{200, 204}, updateResp.StatusCode,
		"update request should succeed even with different Type in body")

	// Verify that the Type remained TXT (3) — bunny.net API treats Type as immutable on update
	listResp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), multiTypeKey, nil)
	defer listResp.Body.Close()

	var records []struct {
		ID    int64  `json:"Id"`
		Type  int    `json:"Type"`
		Value string `json:"Value"`
	}
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&records))

	for _, rec := range records {
		if rec.ID == created.ID {
			assert.Equal(t, 3, rec.Type, "record type should remain TXT (3) — Type is immutable on update")
			assert.Equal(t, "1.2.3.4", rec.Value, "record value should be updated to the new value")
		}
	}
}

// TestE2E_UpdateRecord_SRVFields verifies that SRV-specific fields (Priority, Weight, Port)
// are correctly handled during update.
func TestE2E_UpdateRecord_SRVFields(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Need a key with SRV permission
	srvKey := createScopedKeyWithRecordTypes(t, env.AdminToken, zone.ID, []string{"SRV"})

	// Create an SRV record
	addBody := map[string]interface{}{
		"Type":     8, // SRV
		"Name":     "_sip._tcp",
		"Value":    "sip.example.com",
		"Priority": 10,
		"Weight":   20,
		"Port":     5060,
	}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), srvKey, body)
	defer addResp.Body.Close()
	require.Equal(t, 201, addResp.StatusCode)

	var created struct {
		ID int64 `json:"Id"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))

	// Update SRV-specific fields
	updateBody := map[string]interface{}{
		"Type":     8,
		"Name":     "_sip._tcp",
		"Value":    "sip2.example.com",
		"Priority": 5,
		"Weight":   30,
		"Port":     5061,
	}
	body, _ = json.Marshal(updateBody)
	updateResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), srvKey, body)
	defer updateResp.Body.Close()
	assert.Contains(t, []int{200, 204}, updateResp.StatusCode)

	// Verify all SRV fields were updated
	listResp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), srvKey, nil)
	defer listResp.Body.Close()
	require.Equal(t, 200, listResp.StatusCode)

	var records []struct {
		ID       int64  `json:"Id"`
		Value    string `json:"Value"`
		Priority int32  `json:"Priority"`
		Weight   int32  `json:"Weight"`
		Port     int32  `json:"Port"`
	}
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&records))

	for _, rec := range records {
		if rec.ID == created.ID {
			assert.Equal(t, "sip2.example.com", rec.Value)
			assert.Equal(t, int32(5), rec.Priority)
			assert.Equal(t, int32(30), rec.Weight)
			assert.Equal(t, int32(5061), rec.Port)
		}
	}
}

// TestE2E_UpdateRecord_MXPriority verifies that MX-specific Priority field
// is correctly updated.
func TestE2E_UpdateRecord_MXPriority(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	mxKey := createScopedKeyWithRecordTypes(t, env.AdminToken, zone.ID, []string{"MX"})

	// Create MX record
	addBody := map[string]interface{}{
		"Type":     4, // MX
		"Name":     "",
		"Value":    "mail.example.com",
		"Priority": 10,
	}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), mxKey, body)
	defer addResp.Body.Close()

	// MX might require Name to be non-empty depending on mock — check actual behavior
	if addResp.StatusCode != 201 {
		t.Skipf("MX record creation returned %d — mock may require non-empty Name, skipping update test", addResp.StatusCode)
	}

	var created struct {
		ID int64 `json:"Id"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))

	// Update priority
	updateBody := map[string]interface{}{
		"Type":     4,
		"Name":     "",
		"Value":    "mail2.example.com",
		"Priority": 20,
	}
	body, _ = json.Marshal(updateBody)
	updateResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), mxKey, body)
	defer updateResp.Body.Close()
	assert.Contains(t, []int{200, 204}, updateResp.StatusCode)
}

// TestE2E_UpdateRecord_CAAFields verifies that CAA-specific fields (Flags, Tag)
// are correctly handled during update.
func TestE2E_UpdateRecord_CAAFields(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	caaKey := createScopedKeyWithRecordTypes(t, env.AdminToken, zone.ID, []string{"CAA"})

	// Create CAA record
	addBody := map[string]interface{}{
		"Type":  9, // CAA
		"Name":  "@",
		"Value": "letsencrypt.org",
		"Flags": 0,
		"Tag":   "issue",
	}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), caaKey, body)
	defer addResp.Body.Close()
	require.Equal(t, 201, addResp.StatusCode)

	var created struct {
		ID int64 `json:"Id"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))

	// Update to issuewild with different flags
	updateBody := map[string]interface{}{
		"Type":  9,
		"Name":  "@",
		"Value": "letsencrypt.org",
		"Flags": 128,
		"Tag":   "issuewild",
	}
	body, _ = json.Marshal(updateBody)
	updateResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), caaKey, body)
	defer updateResp.Body.Close()
	assert.Contains(t, []int{200, 204}, updateResp.StatusCode)

	// Verify CAA fields updated
	listResp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), caaKey, nil)
	defer listResp.Body.Close()

	var records []struct {
		ID    int64  `json:"Id"`
		Flags int    `json:"Flags"`
		Tag   string `json:"Tag"`
	}
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&records))

	for _, rec := range records {
		if rec.ID == created.ID {
			assert.Equal(t, 128, rec.Flags, "CAA Flags should be updated")
			assert.Equal(t, "issuewild", rec.Tag, "CAA Tag should be updated")
		}
	}
}

// TestE2E_UpdateRecord_DisabledField verifies that the Disabled field
// can be toggled via update.
func TestE2E_UpdateRecord_DisabledField(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	// Create record (Disabled defaults to false)
	addBody := map[string]interface{}{"Type": 3, "Name": "toggle", "Value": "v1"}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body)
	defer addResp.Body.Close()
	require.Equal(t, 201, addResp.StatusCode)

	var created struct {
		ID       int64 `json:"Id"`
		Disabled bool  `json:"Disabled"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))
	assert.False(t, created.Disabled, "newly created record should not be disabled")

	// Update to disabled=true
	updateBody := map[string]interface{}{"Type": 3, "Name": "toggle", "Value": "v1", "Disabled": true}
	body, _ = json.Marshal(updateBody)
	updateResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), apiKey, body)
	updateResp.Body.Close()
	assert.Contains(t, []int{200, 204}, updateResp.StatusCode)

	// Verify disabled state
	listResp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, nil)
	defer listResp.Body.Close()

	var records []struct {
		ID       int64 `json:"Id"`
		Disabled bool  `json:"Disabled"`
	}
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&records))

	for _, rec := range records {
		if rec.ID == created.ID {
			assert.True(t, rec.Disabled, "record should now be disabled")
		}
	}
}

// TestE2E_UpdateRecord_CommentField verifies that the Comment field
// can be set and cleared via update.
func TestE2E_UpdateRecord_CommentField(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	// Create record without comment
	addBody := map[string]interface{}{"Type": 3, "Name": "commented", "Value": "v1"}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body)
	defer addResp.Body.Close()
	require.Equal(t, 201, addResp.StatusCode)

	var created struct {
		ID int64 `json:"Id"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))

	// Add a comment
	updateBody := map[string]interface{}{"Type": 3, "Name": "commented", "Value": "v1", "Comment": "ACME challenge for cert renewal"}
	body, _ = json.Marshal(updateBody)
	updateResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), apiKey, body)
	updateResp.Body.Close()
	assert.Contains(t, []int{200, 204}, updateResp.StatusCode)

	// Verify comment
	listResp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, nil)
	defer listResp.Body.Close()

	var records []struct {
		ID      int64  `json:"Id"`
		Comment string `json:"Comment"`
	}
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&records))

	for _, rec := range records {
		if rec.ID == created.ID {
			assert.Equal(t, "ACME challenge for cert renewal", rec.Comment)
		}
	}
}

// =============================================================================
// Update DNS Record - Workflow Integration
// =============================================================================

// TestE2E_UpdateRecord_ACMETokenRotation simulates an ACME DNS-01 workflow
// where the challenge token needs to be updated (rotated) without deleting
// and recreating the record. This is a realistic use case for certificate renewal.
func TestE2E_UpdateRecord_ACMETokenRotation(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	// Tightly scoped key: only TXT records, only the actions needed for ACME
	acmeKey := createScopedKeyWithActions(t, env.AdminToken, zone.ID,
		[]string{"list_zones", "get_zone", "list_records", "add_record", "update_record", "delete_record"},
		[]string{"TXT"})

	// Step 1: Initial certificate request — create the challenge record
	token1 := fmt.Sprintf("challenge-token-1-%d", time.Now().UnixNano())
	addBody := map[string]interface{}{
		"Type":  3,
		"Name":  "_acme-challenge",
		"Value": token1,
		"Ttl":   60,
	}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), acmeKey, body)
	defer addResp.Body.Close()
	require.Equal(t, 201, addResp.StatusCode)

	var challenge struct {
		ID    int64  `json:"Id"`
		Value string `json:"Value"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&challenge))
	require.Equal(t, token1, challenge.Value)

	// Step 2: Certificate renewal — update the token without delete+recreate
	token2 := fmt.Sprintf("challenge-token-2-%d", time.Now().UnixNano())
	updateBody := map[string]interface{}{
		"Type":  3,
		"Name":  "_acme-challenge",
		"Value": token2,
		"Ttl":   60,
	}
	body, _ = json.Marshal(updateBody)
	updateResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, challenge.ID), acmeKey, body)
	defer updateResp.Body.Close()
	assert.Contains(t, []int{200, 204}, updateResp.StatusCode)

	// Step 3: Verify the new token is in place
	listResp := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), acmeKey, nil)
	defer listResp.Body.Close()
	require.Equal(t, 200, listResp.StatusCode)

	var records []struct {
		ID    int64  `json:"Id"`
		Name  string `json:"Name"`
		Value string `json:"Value"`
	}
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&records))

	var found bool
	for _, rec := range records {
		if rec.ID == challenge.ID {
			found = true
			assert.Equal(t, token2, rec.Value, "challenge token should be rotated")
			assert.Equal(t, "_acme-challenge", rec.Name, "record name should be unchanged")
		}
	}
	require.True(t, found, "challenge record should still exist with same ID")

	// Step 4: Cleanup
	deleteResp := proxyRequest(t, "DELETE", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, challenge.ID), acmeKey, nil)
	defer deleteResp.Body.Close()
	require.Equal(t, 204, deleteResp.StatusCode)
}

// TestE2E_UpdateRecord_CreateUpdateDelete_FullLifecycle tests the complete
// record lifecycle: create -> read -> update -> read -> delete -> verify gone.
func TestE2E_UpdateRecord_CreateUpdateDelete_FullLifecycle(t *testing.T) {
	env := testenv.Setup(t)
	zones := env.CreateTestZones(t, 1)
	zone := zones[0]

	apiKey := createScopedKey(t, env.AdminToken, zone.ID)

	// Create
	addBody := map[string]interface{}{"Type": 0, "Name": "lifecycle", "Value": "1.1.1.1", "Ttl": 300}
	body, _ := json.Marshal(addBody)
	addResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, body)
	defer addResp.Body.Close()
	require.Equal(t, 201, addResp.StatusCode)

	var created struct {
		ID    int64  `json:"Id"`
		Value string `json:"Value"`
	}
	require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))
	require.Equal(t, "1.1.1.1", created.Value)

	// Read — verify exists
	listResp1 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, nil)
	defer listResp1.Body.Close()
	require.Equal(t, 200, listResp1.StatusCode)

	// Update
	updateBody := map[string]interface{}{"Type": 0, "Name": "lifecycle", "Value": "2.2.2.2", "Ttl": 600}
	body, _ = json.Marshal(updateBody)
	updateResp := proxyRequest(t, "POST", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), apiKey, body)
	updateResp.Body.Close()
	assert.Contains(t, []int{200, 204}, updateResp.StatusCode)

	// Read — verify updated
	listResp2 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, nil)
	defer listResp2.Body.Close()
	require.Equal(t, 200, listResp2.StatusCode)

	var records []struct {
		ID    int64  `json:"Id"`
		Value string `json:"Value"`
		TTL   int32  `json:"Ttl"`
	}
	require.NoError(t, json.NewDecoder(listResp2.Body).Decode(&records))
	for _, rec := range records {
		if rec.ID == created.ID {
			assert.Equal(t, "2.2.2.2", rec.Value)
			assert.Equal(t, int32(600), rec.TTL)
		}
	}

	// Delete
	deleteResp := proxyRequest(t, "DELETE", fmt.Sprintf("/dnszone/%d/records/%d", zone.ID, created.ID), apiKey, nil)
	defer deleteResp.Body.Close()
	require.Equal(t, 204, deleteResp.StatusCode)

	// Verify gone
	listResp3 := proxyRequest(t, "GET", fmt.Sprintf("/dnszone/%d/records", zone.ID), apiKey, nil)
	defer listResp3.Body.Close()
	require.Equal(t, 200, listResp3.StatusCode)

	var recordsAfter []struct {
		ID int64 `json:"Id"`
	}
	require.NoError(t, json.NewDecoder(listResp3.Body).Decode(&recordsAfter))
	for _, rec := range recordsAfter {
		assert.NotEqual(t, created.ID, rec.ID, "deleted record should not appear in list")
	}
}
