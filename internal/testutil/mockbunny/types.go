// Package mockbunny provides types and utilities for a mock bunny.net server
// used in testing the bunny-api-proxy.
package mockbunny

import (
	"sync"
	"time"
)

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

// State holds the internal mock server state.
type State struct {
	mu           sync.RWMutex
	zones        map[int64]*Zone
	nextZoneID   int64
	nextRecordID int64
}

// NewState creates a new State instance for the mock server.
func NewState() *State {
	s := &State{
		zones:        make(map[int64]*Zone),
		nextZoneID:   1,
		nextRecordID: 1,
	}
	_ = &s.mu // Mutex will be used by state management methods
	return s
}

// ListZonesResponse is a paginated response for the List Zones endpoint.
type ListZonesResponse struct {
	CurrentPage  int    `json:"CurrentPage"`
	TotalItems   int    `json:"TotalItems"`
	HasMoreItems bool   `json:"HasMoreItems"`
	Items        []Zone `json:"Items"`
}

// ErrorResponse represents an error response from the API.
type ErrorResponse struct {
	ErrorKey string `json:"ErrorKey"`
	Field    string `json:"Field"`
	Message  string `json:"Message"`
}
