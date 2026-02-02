// Package bunny provides types and error handling for the bunny.net API client.
package bunny

import "time"

// Record represents a DNS record within a zone.
type Record struct {
	ID                    int64   `json:"Id"`
	Type                  string  `json:"Type"`
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
	MonitorStatus         string  `json:"MonitorStatus"`
	MonitorType           string  `json:"MonitorType"`
	GeolocationLatitude   float64 `json:"GeolocationLatitude"`
	GeolocationLongitude  float64 `json:"GeolocationLongitude"`
	LatencyZone           string  `json:"LatencyZone"`
	SmartRoutingType      string  `json:"SmartRoutingType"`
	Disabled              bool    `json:"Disabled"`
	Comment               string  `json:"Comment"`
}

// Zone represents a DNS zone.
type Zone struct {
	ID                       int64     `json:"Id"`
	Domain                   string    `json:"Domain"`
	Records                  []Record  `json:"Records"`
	DateModified             time.Time `json:"DateModified"`
	DateCreated              time.Time `json:"DateCreated"`
	NameserversDetected      bool      `json:"NameserversDetected"`
	CustomNameserversEnabled bool      `json:"CustomNameserversEnabled"`
	Nameserver1              string    `json:"Nameserver1"`
	Nameserver2              string    `json:"Nameserver2"`
	SoaEmail                 string    `json:"SoaEmail"`
	NameserversNextCheck     time.Time `json:"NameserversNextCheck,omitempty"`
	LoggingEnabled           bool      `json:"LoggingEnabled"`
	LoggingIPAnonymization   bool      `json:"LoggingIPAnonymizationEnabled"`
	LogAnonymizationType     string    `json:"LogAnonymizationType"`
	DnsSecEnabled            bool      `json:"DnsSecEnabled"`
	CertificateKeyType       string    `json:"CertificateKeyType"`
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
