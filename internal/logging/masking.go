// Package logging provides utilities for secure logging with data masking.
package logging

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MaskHeader redacts sensitive header values based on header name.
// Returns the redacted value suitable for logging.
//
// Rules:
// - Password/secret headers: "[REDACTED]" (no partial reveal)
// - Token/API key headers: "****" + last4chars (e.g., "****ab3f")
// - Other headers: returned unchanged
func MaskHeader(name, value string) string {
	lowerName := strings.ToLower(name)

	// Password/secret headers - full redaction
	if strings.Contains(lowerName, "password") ||
		strings.Contains(lowerName, "secret") ||
		strings.Contains(lowerName, "private-key") {
		return "[REDACTED]"
	}

	// Token/API key headers - show last 4 chars
	if lowerName == "authorization" ||
		lowerName == "accesskey" ||
		lowerName == "x-api-key" ||
		lowerName == "x-access-key" {
		if len(value) < 4 {
			return "****"
		}
		return "****" + value[len(value)-4:]
	}

	// All other headers - return unchanged
	return value
}

// MaskJSONBody redacts non-allowlisted fields in a JSON body.
// Uses an allowlist approach for security.
//
// If allowlist is nil, returns the body unchanged (everything allowed).
// If allowlist is non-nil, only fields in the allowlist are preserved.
// All other fields are replaced with "[REDACTED]".
//
// Returns the masked JSON as bytes, or the original if parsing fails.
func MaskJSONBody(body []byte, allowlist []string) []byte {
	// If allowlist is nil, return body unchanged
	if allowlist == nil {
		return body
	}

	// Empty body - return as is
	if len(body) == 0 {
		return body
	}

	// Parse JSON
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		// Parsing failed - return original
		return body
	}

	// Create allowlist map for O(1) lookup
	allowlistMap := make(map[string]bool)
	for _, field := range allowlist {
		allowlistMap[field] = true
	}

	// Process the data recursively
	masked := maskJSONValue(data, allowlistMap)

	// Re-serialize to JSON
	result, err := json.Marshal(masked)
	if err != nil {
		// Serialization failed - return original
		return body
	}

	return result
}

// maskJSONValue recursively masks JSON values based on allowlist
func maskJSONValue(value interface{}, allowlist map[string]bool) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		// Process object - always recurse, apply allowlist to each field
		result := make(map[string]interface{})
		for key, val := range v {
			if allowlist[key] {
				// Field is in allowlist - keep it, and recursively process its value
				result[key] = maskJSONValue(val, allowlist)
			} else {
				// Field not in allowlist
				switch val.(type) {
				case map[string]interface{}, []interface{}:
					// For objects/arrays: still recurse to process nested fields
					result[key] = maskJSONValue(val, allowlist)
				default:
					// For primitives: redact them
					result[key] = "[REDACTED]"
				}
			}
		}
		return result
	case []interface{}:
		// Process array - keep structure, redact non-allowlisted fields
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = maskJSONValue(item, allowlist)
		}
		return result
	default:
		// Primitive values - return unchanged
		return value
	}
}

// FormatBinaryData formats binary data for logging.
// Returns a human-readable size indicator.
func FormatBinaryData(data []byte) string {
	size := len(data)
	return fmt.Sprintf("[BINARY: %d bytes]", size)
}
