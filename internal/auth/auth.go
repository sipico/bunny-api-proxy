// Package auth provides API key validation and permission checking.
package auth

// Validator validates API keys and checks permissions.
type Validator struct {
	// TODO: Add fields for storage access, etc.
}

// New creates a new Validator instance.
func New() *Validator {
	return &Validator{}
}
