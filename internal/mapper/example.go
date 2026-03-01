package mapper

import (
	"go/types"
	"strings"
)

// nameHints maps common JSON field names to realistic example values.
var nameHints = map[string]interface{}{
	"id":           "f47ac10b-58cc-4372-a567-0e02b2c3d479",
	"uuid":         "f47ac10b-58cc-4372-a567-0e02b2c3d479",
	"email":        "user@example.com",
	"name":         "John Doe",
	"first_name":   "John",
	"last_name":    "Doe",
	"username":     "johndoe",
	"phone":        "+1-555-0100",
	"url":          "https://example.com",
	"website":      "https://example.com",
	"avatar":       "https://example.com/avatar.png",
	"image":        "https://example.com/image.png",
	"token":        "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
	"access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
	"password":     "••••••••",
	"created_at":   "2024-01-15T10:30:00Z",
	"updated_at":   "2024-01-15T10:30:00Z",
	"deleted_at":   nil,
	"description":  "A brief description",
	"title":        "Example Title",
	"slug":         "example-title",
	"status":       "active",
	"role":         "user",
	"count":        42,
	"total":        100,
	"page":         1,
	"limit":        20,
	"offset":       0,
}

// suffixHints maps field name suffixes to example values.
var suffixHints = map[string]interface{}{
	"_id":  "f47ac10b-58cc-4372-a567-0e02b2c3d479",
	"_at":  "2024-01-15T10:30:00Z",
	"_url": "https://example.com",
}

// generateExample produces a realistic example value for a field based on
// its JSON name (via heuristics) and then falls back to the Go type.
func generateExample(t types.Type, jsonName string) interface{} {
	lower := strings.ToLower(jsonName)

	// Direct name match.
	if v, ok := nameHints[lower]; ok {
		return v
	}

	// Suffix-based match.
	for suffix, val := range suffixHints {
		if strings.HasSuffix(lower, suffix) {
			return val
		}
	}

	// Type fallback.
	switch u := t.Underlying().(type) {
	case *types.Basic:
		switch {
		case u.Info()&types.IsString != 0:
			return "string"
		case u.Info()&types.IsInteger != 0:
			return 0
		case u.Info()&types.IsFloat != 0:
			return 0.0
		case u.Info()&types.IsBoolean != 0:
			return false
		}
	case *types.Slice:
		return []interface{}{}
	case *types.Map:
		return map[string]interface{}{}
	case *types.Pointer:
		return generateExample(u.Elem(), jsonName)
	}
	return nil
}
