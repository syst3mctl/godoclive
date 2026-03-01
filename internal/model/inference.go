package model

import (
	"strings"
	"unicode"
)

// stripPrefixes are meaningless prefixes removed from handler names.
var stripPrefixes = []string{"Handle", "Http", "HTTP", "API", "Api", "Do"}

// irregularPlurals maps nouns that don't pluralize by simply adding "s".
var irregularPlurals = map[string]string{
	"person":  "people",
	"child":   "children",
	"status":  "statuses",
	"address": "addresses",
	"health":  "health",
	"auth":    "auth",
	"info":    "info",
	"data":    "data",
	"media":   "media",
}

// InferSummary converts a handler function name into a human-readable summary.
// GetUserByID → "Get User By ID", CreateUser → "Create User".
func InferSummary(handlerName string) string {
	words := splitCamelCase(handlerName)
	if len(words) == 0 {
		return handlerName
	}

	// Strip meaningless prefixes.
	words = stripPrefix(words)

	// Capitalize each word.
	for i, w := range words {
		words[i] = capitalizeWord(w)
	}

	return strings.Join(words, " ")
}

// knownVerbs are HTTP/CRUD verbs that should be skipped when inferring the tag.
var knownVerbs = map[string]bool{
	"get": true, "list": true, "create": true, "update": true,
	"delete": true, "remove": true, "put": true, "patch": true,
	"post": true, "fetch": true, "find": true, "search": true,
	"upload": true, "download": true, "refresh": true, "reset": true,
	"send": true, "set": true, "add": true, "check": true,
	"verify": true, "validate": true,
}

// InferTag converts a handler function name into a grouping tag.
// CreateUser → "users", GetOrderItem → "orders", UploadAvatar → "avatars".
func InferTag(handlerName string) string {
	words := splitCamelCase(handlerName)
	if len(words) == 0 {
		return ""
	}

	// Strip meaningless prefixes.
	words = stripPrefix(words)
	if len(words) == 0 {
		return ""
	}

	// Only skip the first word if it's a known verb.
	// For non-verb starts like "HealthCheck", the first word IS the noun.
	noun := strings.ToLower(words[0])
	if knownVerbs[noun] && len(words) > 1 {
		noun = strings.ToLower(words[1])
	}

	return pluralize(noun)
}

// splitCamelCase splits a camelCase or PascalCase string into words.
// It also handles underscores as word boundaries.
// GetUserByID → ["Get", "User", "By", "ID"]
func splitCamelCase(s string) []string {
	// First split on underscores.
	parts := strings.Split(s, "_")
	var words []string
	for _, part := range parts {
		words = append(words, splitCamelPart(part)...)
	}
	return words
}

// splitCamelPart splits a single camelCase/PascalCase token into words.
func splitCamelPart(s string) []string {
	if s == "" {
		return nil
	}

	var words []string
	runes := []rune(s)
	start := 0

	for i := 1; i < len(runes); i++ {
		// Split on lowercase→uppercase boundary.
		if unicode.IsLower(runes[i-1]) && unicode.IsUpper(runes[i]) {
			words = append(words, string(runes[start:i]))
			start = i
			continue
		}
		// Split on uppercase run followed by uppercase+lowercase (e.g., "ID" in "ByID" or "APIKey").
		if i+1 < len(runes) && unicode.IsUpper(runes[i-1]) && unicode.IsUpper(runes[i]) && unicode.IsLower(runes[i+1]) {
			words = append(words, string(runes[start:i]))
			start = i
			continue
		}
	}
	words = append(words, string(runes[start:]))
	return words
}

// stripPrefix removes meaningless leading prefixes from the word list.
func stripPrefix(words []string) []string {
	if len(words) == 0 {
		return words
	}
	for _, prefix := range stripPrefixes {
		if strings.EqualFold(words[0], prefix) && len(words) > 1 {
			return words[1:]
		}
	}
	return words
}

// capitalizeWord uppercases the first letter of a word.
// Keeps fully uppercase words (like "ID", "API") as-is.
func capitalizeWord(w string) string {
	if len(w) == 0 {
		return w
	}
	// If all uppercase already (abbreviation), keep it.
	if len(w) > 1 && w == strings.ToUpper(w) {
		return w
	}
	runes := []rune(w)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// pluralize adds an "s" to a noun, handling common irregular cases.
func pluralize(noun string) string {
	if noun == "" {
		return ""
	}

	// Check irregular plurals.
	if plural, ok := irregularPlurals[noun]; ok {
		return plural
	}

	// Already plural.
	if strings.HasSuffix(noun, "s") && !strings.HasSuffix(noun, "ss") {
		return noun
	}

	// Words ending in s, sh, ch, x, z → add "es".
	if strings.HasSuffix(noun, "ss") || strings.HasSuffix(noun, "sh") ||
		strings.HasSuffix(noun, "ch") || strings.HasSuffix(noun, "x") ||
		strings.HasSuffix(noun, "z") {
		return noun + "es"
	}

	// Words ending in y preceded by a consonant → replace y with ies.
	if strings.HasSuffix(noun, "y") && len(noun) > 1 {
		prev := rune(noun[len(noun)-2])
		if !isVowel(prev) {
			return noun[:len(noun)-1] + "ies"
		}
	}

	return noun + "s"
}

func isVowel(r rune) bool {
	switch unicode.ToLower(r) {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}
