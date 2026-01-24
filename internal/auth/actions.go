// Package auth provides API key validation and permission checking.
package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
)

// URL patterns for DNS API endpoints
var (
	listZonesPattern    = regexp.MustCompile(`^/dnszone/?$`)
	getZonePattern      = regexp.MustCompile(`^/dnszone/(\d+)/?$`)
	recordsPattern      = regexp.MustCompile(`^/dnszone/(\d+)/records/?$`)
	deleteRecordPattern = regexp.MustCompile(`^/dnszone/(\d+)/records/(\d+)/?$`)
)

// ParseRequest extracts action, zone ID, and record type from HTTP request.
func ParseRequest(r *http.Request) (*Request, error) {
	path := r.URL.Path

	// GET /dnszone - list zones
	if r.Method == http.MethodGet && listZonesPattern.MatchString(path) {
		return &Request{Action: ActionListZones}, nil
	}

	// GET /dnszone/{id} - get zone
	if r.Method == http.MethodGet {
		if matches := getZonePattern.FindStringSubmatch(path); matches != nil {
			zoneID, _ := strconv.ParseInt(matches[1], 10, 64) //nolint:errcheck // regex ensures valid number
			return &Request{Action: ActionGetZone, ZoneID: zoneID}, nil
		}
	}

	// GET /dnszone/{id}/records - list records
	if r.Method == http.MethodGet && recordsPattern.MatchString(path) {
		matches := recordsPattern.FindStringSubmatch(path)
		zoneID, _ := strconv.ParseInt(matches[1], 10, 64) //nolint:errcheck // regex ensures valid number
		return &Request{Action: ActionListRecords, ZoneID: zoneID}, nil
	}

	// POST /dnszone/{id}/records - add record
	if r.Method == http.MethodPost && recordsPattern.MatchString(path) {
		matches := recordsPattern.FindStringSubmatch(path)
		zoneID, _ := strconv.ParseInt(matches[1], 10, 64) //nolint:errcheck // regex ensures valid number

		// Read and restore body for later use
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Extract record type
		var payload struct {
			Type string `json:"Type"`
		}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			return nil, fmt.Errorf("failed to parse request body: %w", err)
		}

		return &Request{
			Action:     ActionAddRecord,
			ZoneID:     zoneID,
			RecordType: payload.Type,
		}, nil
	}

	// DELETE /dnszone/{id}/records/{rid} - delete record
	if r.Method == http.MethodDelete {
		if matches := deleteRecordPattern.FindStringSubmatch(path); matches != nil {
			zoneID, _ := strconv.ParseInt(matches[1], 10, 64) //nolint:errcheck // regex ensures valid number
			return &Request{Action: ActionDeleteRecord, ZoneID: zoneID}, nil
		}
	}

	return nil, fmt.Errorf("unrecognized endpoint: %s %s", r.Method, path)
}
