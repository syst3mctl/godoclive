package mapper

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/syst3mctl/godoclive/internal/model"
)

// validatorFormats maps the go-playground/validator rules that name a concrete
// string format onto their OpenAPI equivalent.
var validatorFormats = map[string]string{
	"email":            "email",
	"uuid":             "uuid",
	"uuid3":            "uuid",
	"uuid4":            "uuid",
	"uuid5":            "uuid",
	"url":              "uri",
	"uri":              "uri",
	"hostname":         "hostname",
	"hostname_rfc1123": "hostname",
	"ip":               "ip",
	"ipv4":             "ipv4",
	"ipv6":             "ipv6",
	"datetime":         "date-time",
}

// validatorPatterns maps the character-class rules onto the regular expression
// that expresses the same restriction in a schema.
var validatorPatterns = map[string]string{
	"alpha":        "^[a-zA-Z]+$",
	"alphanum":     "^[a-zA-Z0-9]+$",
	"alphaunicode": `^[\p{L}]+$`,
	"number":       `^[0-9]+$`,
	"numeric":      `^[-+]?[0-9]*\.?[0-9]+$`,
	"hexadecimal":  "^[0-9a-fA-F]+$",
	"lowercase":    "^[^A-Z]*$",
	"uppercase":    "^[^a-z]*$",
}

// parseConstraints reads the validator rules on a field into schema
// constraints. Both tags gin understands are read: `binding` is gin's own
// spelling and `validate` is go-playground/validator's, and a struct shared
// between a gin handler and a standalone validator call carries both.
//
// td is the field's mapped type: several rules mean different things depending
// on it. `min=3` bounds a number's value, a string's length and a slice's item
// count, and emitting the wrong one of those describes a different API.
func parseConstraints(tag reflect.StructTag, td model.TypeDef) *model.Constraints {
	c := &model.Constraints{}
	for _, tagName := range [...]string{"binding", "validate"} {
		applyRules(c, tag.Get(tagName), td)
	}
	if c.IsZero() {
		return nil
	}
	return c
}

// applyRules parses one validator tag value and records what it constrains.
func applyRules(c *model.Constraints, tagValue string, td model.TypeDef) {
	if tagValue == "" || tagValue == "-" {
		return
	}
	for _, rule := range splitRules(tagValue) {
		name, arg, _ := strings.Cut(rule, "=")
		name = strings.TrimSpace(name)

		if format, ok := validatorFormats[name]; ok {
			c.Format = format
			continue
		}
		if pattern, ok := validatorPatterns[name]; ok {
			c.Pattern = pattern
			continue
		}

		switch name {
		case "oneof":
			c.Enum = parseOneOf(arg)
		case "len":
			setBound(c, td, arg, boundExact)
		case "min", "gte":
			setBound(c, td, arg, boundLower)
		case "max", "lte":
			setBound(c, td, arg, boundUpper)
		case "gt":
			if n, ok := parseNumber(arg); ok {
				c.ExclusiveMinimum = &n
			}
		case "lt":
			if n, ok := parseNumber(arg); ok {
				c.ExclusiveMaximum = &n
			}
		}
	}
}

// bound says which end of a range a rule sets.
type bound int

const (
	boundLower bound = iota
	boundUpper
	boundExact // len= sets both ends
)

// setBound records a numeric argument against whichever facet the field's type
// makes it mean: a value range for numbers, a length for strings, an item
// count for slices and maps.
func setBound(c *model.Constraints, td model.TypeDef, arg string, b bound) {
	n, ok := parseNumber(arg)
	if !ok {
		return
	}
	switch {
	case isNumericType(td):
		lo, hi := &c.Minimum, &c.Maximum
		if b == boundLower || b == boundExact {
			*lo = &n
		}
		if b == boundUpper || b == boundExact {
			*hi = &n
		}
	case td.Kind == model.KindSlice || td.Kind == model.KindMap:
		i := int(n)
		if b == boundLower || b == boundExact {
			c.MinItems = &i
		}
		if b == boundUpper || b == boundExact {
			c.MaxItems = &i
		}
	default: // strings, and anything whose length is the only sensible reading
		i := int(n)
		if b == boundLower || b == boundExact {
			c.MinLength = &i
		}
		if b == boundUpper || b == boundExact {
			c.MaxLength = &i
		}
	}
}

// splitRules breaks a tag value on commas, honouring the escape validator uses
// for a literal comma inside a rule argument.
func splitRules(tagValue string) []string {
	var rules []string
	var cur strings.Builder
	for i := 0; i < len(tagValue); i++ {
		switch {
		case tagValue[i] == '\\' && i+1 < len(tagValue):
			i++
			cur.WriteByte(tagValue[i])
		case tagValue[i] == ',':
			rules = append(rules, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(tagValue[i])
		}
	}
	rules = append(rules, cur.String())
	return rules
}

// parseOneOf splits the space-separated values of a oneof rule. A value
// containing spaces is written in single quotes.
func parseOneOf(arg string) []string {
	var values []string
	var cur strings.Builder
	inQuote := false
	flush := func() {
		if cur.Len() > 0 {
			values = append(values, cur.String())
			cur.Reset()
		}
	}
	for i := 0; i < len(arg); i++ {
		switch c := arg[i]; {
		case c == '\'':
			inQuote = !inQuote
			if !inQuote {
				flush()
			}
		case c == ' ' && !inQuote:
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return values
}

// parseNumber parses a rule argument as a number.
func parseNumber(arg string) (float64, bool) {
	n, err := strconv.ParseFloat(strings.TrimSpace(arg), 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// isNumericType reports whether a mapped type is one whose value a bound
// restricts, rather than its length.
func isNumericType(td model.TypeDef) bool {
	if td.Kind != model.KindPrimitive {
		return false
	}
	switch td.Name {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64":
		return true
	}
	return false
}
