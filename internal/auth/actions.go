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

// URL patterns for DNS API endpoints (matching bunny.net API paths)
var (
	listZonesPattern         = regexp.MustCompile(`^/dnszone/?$`)
	getZonePattern           = regexp.MustCompile(`^/dnszone/(\d+)/?$`)
	updateZonePattern        = regexp.MustCompile(`^/dnszone/(\d+)/?$`)
	recordsPattern           = regexp.MustCompile(`^/dnszone/(\d+)/records/?$`)
	updateRecordPattern      = regexp.MustCompile(`^/dnszone/(\d+)/records/(\d+)/?$`)
	deleteRecordPattern      = regexp.MustCompile(`^/dnszone/(\d+)/records/(\d+)/?$`)
	checkAvailabilityPattern = regexp.MustCompile(`^/dnszone/checkavailability/?$`)
	importRecordsPattern     = regexp.MustCompile(`^/dnszone/(\d+)/import/?$`)
	exportRecordsPattern     = regexp.MustCompile(`^/dnszone/(\d+)/export/?$`)
	dnssecPattern            = regexp.MustCompile(`^/dnszone/(\d+)/dnssec/?$`)
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
	// POST /dnszone/checkavailability - check zone availability (admin only)
	// POST /dnszone/{id}/import - import records (admin only)
	if r.Method == http.MethodPost {
		if matches := importRecordsPattern.FindStringSubmatch(path); matches != nil {
			zoneID, _ := strconv.ParseInt(matches[1], 10, 64) //nolint:errcheck // regex ensures valid number
			return &Request{Action: ActionImportRecords, ZoneID: zoneID}, nil
		}
	}
	if r.Method == http.MethodPost && checkAvailabilityPattern.MatchString(path) {
		return &Request{Action: ActionCheckAvailability}, nil
	}
	// POST /dnszone - create zone
	if r.Method == http.MethodPost && listZonesPattern.MatchString(path) {
		return &Request{Action: ActionCreateZone}, nil
	}

	// POST /dnszone/{id}/dnssec - enable DNSSEC (admin only)
	if r.Method == http.MethodPost {
		if matches := dnssecPattern.FindStringSubmatch(path); matches != nil {
			zoneID, _ := strconv.ParseInt(matches[1], 10, 64) //nolint:errcheck // regex ensures valid number
			return &Request{Action: ActionEnableDNSSEC, ZoneID: zoneID}, nil
		}
	}

	// DELETE /dnszone/{id}/dnssec - disable DNSSEC (admin only)
	if r.Method == http.MethodDelete {
		if matches := dnssecPattern.FindStringSubmatch(path); matches != nil {
			zoneID, _ := strconv.ParseInt(matches[1], 10, 64) //nolint:errcheck // regex ensures valid number
			return &Request{Action: ActionDisableDNSSEC, ZoneID: zoneID}, nil
		}
	}
	// POST /dnszone/{id} - update zone (admin only, no permission checking needed)
	if r.Method == http.MethodPost {
		if matches := updateZonePattern.FindStringSubmatch(path); matches != nil {
			// Check if this is the update zone pattern (not followed by /records)
			if !recordsPattern.MatchString(path) {
				zoneID, _ := strconv.ParseInt(matches[1], 10, 64) //nolint:errcheck // regex ensures valid number
				return &Request{Action: ActionUpdateZone, ZoneID: zoneID}, nil
			}
		}
	}

	// GET /dnszone/{id}/export - export records (admin only)
	if r.Method == http.MethodGet {
		if matches := exportRecordsPattern.FindStringSubmatch(path); matches != nil {
			zoneID, _ := strconv.ParseInt(matches[1], 10, 64) //nolint:errcheck // regex ensures valid number
			return &Request{Action: ActionExportRecords, ZoneID: zoneID}, nil
		}
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
			Type int `json:"Type"`
		}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			return nil, fmt.Errorf("failed to parse request body: %w", err)
		}

		// Map type integer to string for permissions checking
		recordType := MapRecordTypeToString(payload.Type)

		return &Request{
			Action:     ActionAddRecord,
			ZoneID:     zoneID,
			RecordType: recordType,
		}, nil
	}

	// POST /dnszone/{id}/records/{rid} - update record
	if r.Method == http.MethodPost {
		if matches := updateRecordPattern.FindStringSubmatch(path); matches != nil {
			zoneID, _ := strconv.ParseInt(matches[1], 10, 64) //nolint:errcheck // regex ensures valid number

			// Read and restore body for later use
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read request body: %w", err)
			}
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			// Extract record type
			var payload struct {
				Type int `json:"Type"`
			}
			if err := json.Unmarshal(bodyBytes, &payload); err != nil {
				return nil, fmt.Errorf("failed to parse request body: %w", err)
			}

			// Map type integer to string for permissions checking
			recordType := MapRecordTypeToString(payload.Type)

			return &Request{
				Action:     ActionUpdateRecord,
				ZoneID:     zoneID,
				RecordType: recordType,
			}, nil
		}
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

// MapRecordTypeToString converts a bunny.net record type integer to its string name.
// Record types: 0 = A, 1 = AAAA, 2 = CNAME, 3 = TXT, 4 = MX, 5 = SPF, 6 = Flatten, 7 = PullZone, 8 = SRV, 9 = CAA, 10 = PTR, 11 = Script, 12 = NS
func MapRecordTypeToString(typeInt int) string {
	switch typeInt {
	case 0:
		return "A"
	case 1:
		return "AAAA"
	case 2:
		return "CNAME"
	case 3:
		return "TXT"
	case 4:
		return "MX"
	case 5:
		return "SPF"
	case 6:
		return "Flatten"
	case 7:
		return "PullZone"
	case 8:
		return "SRV"
	case 9:
		return "CAA"
	case 10:
		return "PTR"
	case 11:
		return "Script"
	case 12:
		return "NS"
	default:
		return "" // Unknown type
	}
}
