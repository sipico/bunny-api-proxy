package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"

	"github.com/sipico/bunny-api-proxy/internal/storage"
)

// BootstrapState represents the system configuration state
type BootstrapState int

const (
	// StateUnconfigured means no admin tokens exist yet
	// Master key authentication is allowed in this state
	StateUnconfigured BootstrapState = iota

	// StateConfigured means at least one admin token exists
	// Master key authentication is locked out in this state
	StateConfigured
)

// String returns the string representation of the bootstrap state
func (s BootstrapState) String() string {
	switch s {
	case StateUnconfigured:
		return "UNCONFIGURED"
	case StateConfigured:
		return "CONFIGURED"
	default:
		return "UNKNOWN"
	}
}

// BootstrapService manages the bootstrap state machine
type BootstrapService struct {
	tokens        storage.TokenStore
	masterKeyHash string // SHA-256 hash of BUNNY_API_KEY
}

// NewBootstrapService creates a new bootstrap service
// masterKey is the raw BUNNY_API_KEY value
func NewBootstrapService(tokens storage.TokenStore, masterKey string) *BootstrapService {
	hash := sha256.Sum256([]byte(masterKey))
	return &BootstrapService{
		tokens:        tokens,
		masterKeyHash: hex.EncodeToString(hash[:]),
	}
}

// GetState returns the current bootstrap state
// Returns StateUnconfigured if no admin tokens exist
// Returns StateConfigured if at least one admin token exists
func (b *BootstrapService) GetState(ctx context.Context) (BootstrapState, error) {
	hasAdmin, err := b.tokens.HasAnyAdminToken(ctx)
	if err != nil {
		return StateUnconfigured, err
	}
	if hasAdmin {
		return StateConfigured, nil
	}
	return StateUnconfigured, nil
}

// IsMasterKey checks if the provided key matches the bunny.net API key
//
// SECURITY: This function MUST use constant-time comparison to prevent timing
// side-channel attacks. While SHA-256 hashing before comparison significantly
// mitigates practical timing attacks (the hash computation dominates timing),
// defense-in-depth requires using subtle.ConstantTimeCompare for the final
// hash comparison.
//
// DO NOT refactor this to use == or strings.EqualFold - such changes would
// introduce a timing side-channel vulnerability. The test
// TestIsMasterKey_UsesConstantTimeComparison enforces this requirement.
func (b *BootstrapService) IsMasterKey(key string) bool {
	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])
	// SECURITY-CRITICAL: Must use constant-time comparison (see function comment)
	return subtle.ConstantTimeCompare([]byte(keyHash), []byte(b.masterKeyHash)) == 1
}

// CanUseMasterKey returns true only during UNCONFIGURED state
// Once an admin token exists, master key is locked out
func (b *BootstrapService) CanUseMasterKey(ctx context.Context) (bool, error) {
	state, err := b.GetState(ctx)
	if err != nil {
		return false, err
	}
	return state == StateUnconfigured, nil
}

// ValidateMasterKey checks if the key is valid AND can be used
// Returns true only if: key matches AND system is UNCONFIGURED
func (b *BootstrapService) ValidateMasterKey(ctx context.Context, key string) (bool, error) {
	if !b.IsMasterKey(key) {
		return false, nil
	}
	return b.CanUseMasterKey(ctx)
}
