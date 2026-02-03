// Package bunny provides types and error handling for the bunny.net API client.
package bunny

import (
	"fmt"
	"strings"
	"time"
)

// BunnyTime handles bunny.net timestamps which may omit timezone suffix.
// When no timezone is present, treats the time as UTC.
//
//nolint:revive // BunnyTime is descriptive and distinguishes from time.Time
type BunnyTime struct {
	time.Time
}

// UnmarshalJSON implements json.Unmarshaler for BunnyTime.
// It handles both RFC3339 format (with timezone) and bunny.net's
// format without timezone (treated as UTC).
func (bt *BunnyTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		return nil
	}

	// Try RFC3339 first (with timezone like "2006-01-02T15:04:05Z")
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		bt.Time = t
		return nil
	}

	// No timezone suffix - treat as UTC by appending "Z"
	if t, err := time.Parse(time.RFC3339, s+"Z"); err == nil {
		bt.Time = t
		return nil
	}

	return fmt.Errorf("invalid timestamp format: %s", s)
}

// Record represents a DNS record within a zone.
type Record struct {
	ID                    int64   `json:"Id"`
	Type                  int     `json:"Type"` // 0 = A, 1 = AAAA, 2 = CNAME, 3 = TXT, 4 = MX, 5 = SPF, 6 = Flatten, 7 = PullZone, 8 = SRV, 9 = CAA, 10 = PTR, 11 = Script, 12 = NS
	Name                  string  `json:"Name"`
	Value                 string  `json:"Value"`
	TTL                   int32   `json:"Ttl"`
	Priority              int32   `json:"Priority"`
	Weight                int32   `json:"Weight"`
	Port                  int32   `json:"Port"`
	Flags                 int     `json:"Flags"`
	Tag                   string  `json:"Tag"`
	Accelerated           bool    `json:"Accelerated"`
	AcceleratedPullZoneID int64   `json:"AcceleratedPullZoneId"`
	MonitorStatus         int     `json:"MonitorStatus"`   // 0 = Unknown, 1 = Online, 2 = Offline
	MonitorType           int     `json:"MonitorType"`     // 0 = None, 1 = Ping, 2 = Http, 3 = Monitor
	GeolocationLatitude   float64 `json:"GeolocationLatitude"`
	GeolocationLongitude  float64 `json:"GeolocationLongitude"`
	LatencyZone           string  `json:"LatencyZone"`
	SmartRoutingType      int     `json:"SmartRoutingType"` // 0 = None, 1 = Latency, 2 = Geolocation
	Disabled              bool    `json:"Disabled"`
	Comment               string  `json:"Comment"`
}

// Zone represents a DNS zone.
type Zone struct {
	ID                       int64     `json:"Id"`
	Domain                   string    `json:"Domain"`
	Records                  []Record  `json:"Records"`
	DateModified             BunnyTime `json:"DateModified"`
	DateCreated              BunnyTime `json:"DateCreated"`
	NameserversDetected      bool      `json:"NameserversDetected"`
	CustomNameserversEnabled bool      `json:"CustomNameserversEnabled"`
	Nameserver1              string    `json:"Nameserver1"`
	Nameserver2              string    `json:"Nameserver2"`
	SoaEmail                 string    `json:"SoaEmail"`
	NameserversNextCheck     BunnyTime `json:"NameserversNextCheck,omitempty"`
	LoggingEnabled           bool      `json:"LoggingEnabled"`
	LoggingIPAnonymization   bool      `json:"LoggingIPAnonymizationEnabled"`
	LogAnonymizationType     int       `json:"LogAnonymizationType"` // 0 = OneDigit, 1 = Drop
	DnsSecEnabled            bool      `json:"DnsSecEnabled"`
	CertificateKeyType       int       `json:"CertificateKeyType"` // 0 = Ecdsa, 1 = Rsa
}

// ListZonesResponse is a paginated response for the List Zones endpoint.
type ListZonesResponse struct {
	CurrentPage  int    `json:"CurrentPage"`
	TotalItems   int    `json:"TotalItems"`
	HasMoreItems bool   `json:"HasMoreItems"`
	Items        []Zone `json:"Items"`
}

// ListZonesOptions contains optional parameters for ListZones.
type ListZonesOptions struct {
	Page    int
	PerPage int
	Search  string
}

// CreateZoneRequest represents the request body for creating a new DNS zone.
type CreateZoneRequest struct {
	Domain string `json:"Domain"`
}
