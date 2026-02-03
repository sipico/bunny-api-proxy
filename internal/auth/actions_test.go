package auth

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseRequest(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantAction Action
		wantZoneID int64
		wantType   string
		wantErr    bool
	}{
		{
			name:       "list zones",
			method:     "GET",
			path:       "/dnszone",
			wantAction: ActionListZones,
		},
		{
			name:       "list zones with trailing slash",
			method:     "GET",
			path:       "/dnszone/",
			wantAction: ActionListZones,
		},
		{
			name:       "get zone",
			method:     "GET",
			path:       "/dnszone/123",
			wantAction: ActionGetZone,
			wantZoneID: 123,
		},
		{
			name:       "list records",
			method:     "GET",
			path:       "/dnszone/456/records",
			wantAction: ActionListRecords,
			wantZoneID: 456,
		},
		{
			name:       "add record",
			method:     "POST",
			path:       "/dnszone/789/records",
			body:       `{"Type":3,"Name":"test","Value":"hello"}`,
			wantAction: ActionAddRecord,
			wantZoneID: 789,
			wantType:   "TXT",
		},
		{
			name:       "delete record",
			method:     "DELETE",
			path:       "/dnszone/123/records/456",
			wantAction: ActionDeleteRecord,
			wantZoneID: 123,
		},
		{
			name:    "invalid path",
			method:  "GET",
			path:    "/invalid",
			wantErr: true,
		},
		{
			name:    "invalid method for path",
			method:  "PUT",
			path:    "/dnszone/123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, tt.path, body)

			got, err := ParseRequest(req)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Action != tt.wantAction {
				t.Errorf("Action = %v, want %v", got.Action, tt.wantAction)
			}
			if got.ZoneID != tt.wantZoneID {
				t.Errorf("ZoneID = %v, want %v", got.ZoneID, tt.wantZoneID)
			}
			if got.RecordType != tt.wantType {
				t.Errorf("RecordType = %v, want %v", got.RecordType, tt.wantType)
			}
		})
	}
}

func TestParseRequest_BodyPreserved(t *testing.T) {
	body := `{"Type":0,"Name":"www","Value":"1.2.3.4"}`
	req := httptest.NewRequest("POST", "/dnszone/123/records", strings.NewReader(body))

	_, err := ParseRequest(req)
	if err != nil {
		t.Fatalf("ParseRequest failed: %v", err)
	}

	// Body should still be readable
	restored, _ := io.ReadAll(req.Body)
	if string(restored) != body {
		t.Errorf("Body not preserved: got %q, want %q", restored, body)
	}
}

func TestParseRequest_InvalidJSON(t *testing.T) {
	// Test with malformed JSON body
	req := httptest.NewRequest("POST", "/dnszone/123/records", strings.NewReader(`{invalid json`))

	_, err := ParseRequest(req)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
		return
	}

	if !strings.Contains(err.Error(), "failed to parse request body") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestMapRecordTypeToString(t *testing.T) {
	tests := []struct {
		typeInt int
		want    string
	}{
		{0, "A"},
		{1, "AAAA"},
		{2, "CNAME"},
		{3, "TXT"},
		{4, "MX"},
		{5, "SPF"},
		{6, "Flatten"},
		{7, "PullZone"},
		{8, "SRV"},
		{9, "CAA"},
		{10, "PTR"},
		{11, "Script"},
		{12, "NS"},
		{13, ""}, // Unknown type
		{-1, ""}, // Invalid type
		{999, ""}, // Out of range
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := MapRecordTypeToString(tt.typeInt)
			if got != tt.want {
				t.Errorf("MapRecordTypeToString(%d) = %q, want %q", tt.typeInt, got, tt.want)
			}
		})
	}
}
