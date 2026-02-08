// Package mockbunny provides types and utilities for a mock bunny.net server
// used in testing the bunny-api-proxy.
package mockbunny

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// MockBunnyTime wraps time.Time to serialize in bunny.net's format
// (with sub-second precision and Z suffix, matching real API behavior).
//
//nolint:revive // MockBunnyTime is descriptive and distinguishes from time.Time
type MockBunnyTime struct {
	time.Time
}

// MarshalJSON implements json.Marshaler for MockBunnyTime.
// It returns timestamps in "2006-01-02T15:04:05.0000000Z" format (with sub-second precision and Z suffix),
// matching bunny.net's actual API creation endpoint.
func (t MockBunnyTime) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	// Format with sub-second precision and Z suffix to match real bunny.net API creation endpoint
	formatted := `"` + t.UTC().Format("2006-01-02T15:04:05.0000000") + `Z"`
	return []byte(formatted), nil
}

// MockBunnyTimeShort wraps time.Time to serialize in bunny.net's short format
// (without sub-second precision, no Z suffix, matching list/get endpoints).
//
//nolint:revive // MockBunnyTimeShort is descriptive and distinguishes from MockBunnyTime
type MockBunnyTimeShort struct {
	time.Time
}

// MarshalJSON implements json.Marshaler for MockBunnyTimeShort.
// It returns timestamps in "2006-01-02T15:04:05" format (without sub-second precision or Z suffix),
// matching bunny.net's actual API behavior on GET/list endpoints.
func (t MockBunnyTimeShort) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	// Format without sub-second precision or Z suffix to match real bunny.net API GET/list endpoints
	formatted := `"` + t.UTC().Format("2006-01-02T15:04:05") + `"`
	return []byte(formatted), nil
}

// UnmarshalJSON implements json.Unmarshaler for MockBunnyTimeShort.
// It handles both RFC3339 format (with timezone) and bunny.net's format without timezone.
func (t *MockBunnyTimeShort) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		return nil
	}

	// Try RFC3339 first (with timezone like "2006-01-02T15:04:05Z")
	if parsed, err := time.Parse(time.RFC3339, s); err == nil {
		t.Time = parsed
		return nil
	}

	// No timezone suffix - treat as UTC by appending "Z"
	if parsed, err := time.Parse(time.RFC3339, s+"Z"); err == nil {
		t.Time = parsed
		return nil
	}

	return fmt.Errorf("invalid timestamp format: %s", s)
}

// UnmarshalJSON implements json.Unmarshaler for MockBunnyTime.
// It handles both RFC3339 format (with timezone) and bunny.net's
// format without timezone (treated as UTC).
func (t *MockBunnyTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		return nil
	}

	// Try RFC3339 first (with timezone like "2006-01-02T15:04:05Z")
	if parsed, err := time.Parse(time.RFC3339, s); err == nil {
		t.Time = parsed
		return nil
	}

	// No timezone suffix - treat as UTC by appending "Z"
	if parsed, err := time.Parse(time.RFC3339, s+"Z"); err == nil {
		t.Time = parsed
		return nil
	}

	return fmt.Errorf("invalid timestamp format: %s", s)
}

// Record represents a DNS record within a zone.
type Record struct {
	ID                    int64         `json:"Id"`
	Type                  int           `json:"Type"` // 0 = A, 1 = AAAA, 2 = CNAME, 3 = TXT, 4 = MX, 5 = SPF, 6 = Flatten, 7 = PullZone, 8 = SRV, 9 = CAA, 10 = PTR, 11 = Script, 12 = NS
	TTL                   int32         `json:"Ttl"`
	Value                 string        `json:"Value"`
	Name                  string        `json:"Name"`
	Weight                int32         `json:"Weight"`
	Priority              int32         `json:"Priority"`
	Port                  int32         `json:"Port"`
	Flags                 int           `json:"Flags"`
	Tag                   string        `json:"Tag"`
	Accelerated           bool          `json:"Accelerated"`
	AcceleratedPullZoneID int64         `json:"AcceleratedPullZoneId"`
	LinkName              string        `json:"LinkName"`
	IPGeoLocationInfo     interface{}   `json:"IPGeoLocationInfo"`
	GeolocationInfo       interface{}   `json:"GeolocationInfo"`
	MonitorStatus         int           `json:"MonitorStatus"` // 0 = Unknown, 1 = Online, 2 = Offline
	MonitorType           int           `json:"MonitorType"`   // 0 = None, 1 = Ping, 2 = Http, 3 = Monitor
	GeolocationLatitude   float64       `json:"GeolocationLatitude"`
	GeolocationLongitude  float64       `json:"GeolocationLongitude"`
	EnviromentalVariables []interface{} `json:"EnviromentalVariables"`
	LatencyZone           *string       `json:"LatencyZone"`
	SmartRoutingType      int           `json:"SmartRoutingType"` // 0 = None, 1 = Latency, 2 = Geolocation
	Disabled              bool          `json:"Disabled"`
	Comment               string        `json:"Comment"`
	AutoSslIssuance       bool          `json:"AutoSslIssuance"`
	AccelerationStatus    int           `json:"AccelerationStatus"`
}

// Zone represents a DNS zone.
type Zone struct {
	ID                       int64         `json:"Id"`
	Domain                   string        `json:"Domain"`
	Records                  []Record      `json:"Records"`
	DateModified             MockBunnyTime `json:"DateModified"`
	DateCreated              MockBunnyTime `json:"DateCreated"`
	NameserversDetected      bool          `json:"NameserversDetected"`
	CustomNameserversEnabled bool          `json:"CustomNameserversEnabled"`
	Nameserver1              string        `json:"Nameserver1"`
	Nameserver2              string        `json:"Nameserver2"`
	SoaEmail                 string        `json:"SoaEmail"`
	NameserversNextCheck     MockBunnyTime `json:"NameserversNextCheck,omitempty"`
	LoggingEnabled           bool          `json:"LoggingEnabled"`
	LoggingIPAnonymization   bool          `json:"LoggingIPAnonymizationEnabled"`
	LogAnonymizationType     int           `json:"LogAnonymizationType"` // 0 = OneDigit, 1 = Drop
	DnsSecEnabled            bool          `json:"DnsSecEnabled"`
	CertificateKeyType       int           `json:"CertificateKeyType"` // 0 = Ecdsa, 1 = Rsa
}

// ZoneShortTime returns a copy of the zone with timestamps formatted using MockBunnyTimeShort
// (no sub-second precision, no Z suffix). Used for GET/list endpoint responses to match
// the real bunny.net API behavior.
func (z *Zone) ZoneShortTime() *ZoneShortTime {
	return &ZoneShortTime{
		ID:                       z.ID,
		Domain:                   z.Domain,
		Records:                  z.Records,
		DateModified:             MockBunnyTimeShort{Time: z.DateModified.Time},
		DateCreated:              MockBunnyTimeShort{Time: z.DateCreated.Time},
		NameserversDetected:      z.NameserversDetected,
		CustomNameserversEnabled: z.CustomNameserversEnabled,
		Nameserver1:              z.Nameserver1,
		Nameserver2:              z.Nameserver2,
		SoaEmail:                 z.SoaEmail,
		NameserversNextCheck:     MockBunnyTimeShort{Time: z.NameserversNextCheck.Time},
		LoggingEnabled:           z.LoggingEnabled,
		LoggingIPAnonymization:   z.LoggingIPAnonymization,
		LogAnonymizationType:     z.LogAnonymizationType,
		DnsSecEnabled:            z.DnsSecEnabled,
		CertificateKeyType:       z.CertificateKeyType,
	}
}

// ZoneShortTime is a variant of Zone that uses MockBunnyTimeShort for timestamps.
// Used for GET/list endpoint responses to match the real bunny.net API behavior.
type ZoneShortTime struct {
	ID                       int64              `json:"Id"`
	Domain                   string             `json:"Domain"`
	Records                  []Record           `json:"Records"`
	DateModified             MockBunnyTimeShort `json:"DateModified"`
	DateCreated              MockBunnyTimeShort `json:"DateCreated"`
	NameserversDetected      bool               `json:"NameserversDetected"`
	CustomNameserversEnabled bool               `json:"CustomNameserversEnabled"`
	Nameserver1              string             `json:"Nameserver1"`
	Nameserver2              string             `json:"Nameserver2"`
	SoaEmail                 string             `json:"SoaEmail"`
	NameserversNextCheck     MockBunnyTimeShort `json:"NameserversNextCheck,omitempty"`
	LoggingEnabled           bool               `json:"LoggingEnabled"`
	LoggingIPAnonymization   bool               `json:"LoggingIPAnonymizationEnabled"`
	LogAnonymizationType     int                `json:"LogAnonymizationType"` // 0 = OneDigit, 1 = Drop
	DnsSecEnabled            bool               `json:"DnsSecEnabled"`
	CertificateKeyType       int                `json:"CertificateKeyType"` // 0 = Ecdsa, 1 = Rsa
}

// State holds the internal mock server state.
type State struct {
	mu            sync.RWMutex
	zones         map[int64]*Zone
	nextZoneID    int64
	nextRecordID  int64
	scanTriggered map[int64]bool // tracks which zones have had a scan triggered
	scanCallCount map[int64]int  // tracks how many times scan result has been polled per zone
}

// NewState creates a new State instance for the mock server.
func NewState() *State {
	s := &State{
		zones:         make(map[int64]*Zone),
		nextZoneID:    1,
		nextRecordID:  1,
		scanTriggered: make(map[int64]bool),
		scanCallCount: make(map[int64]int),
	}
	_ = &s.mu // Mutex will be used by state management methods
	return s
}

// ListZonesResponse is a paginated response for the List Zones endpoint.
type ListZonesResponse struct {
	Items        []ZoneShortTime `json:"Items"`
	CurrentPage  int             `json:"CurrentPage"`
	TotalItems   int             `json:"TotalItems"`
	HasMoreItems bool            `json:"HasMoreItems"`
}

// ErrorResponse represents an error response from the API.
type ErrorResponse struct {
	ErrorKey string `json:"ErrorKey"`
	Field    string `json:"Field"`
	Message  string `json:"Message"`
}
