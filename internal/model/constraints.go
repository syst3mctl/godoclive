package model

// Constraints are the value restrictions declared on a field or parameter.
//
// Go services state these in validator struct tags — `binding:"..."` for gin,
// `validate:"..."` for go-playground/validator, which gin's binding wraps. The
// rules are already enforced at runtime, so a request that violates one is
// rejected; a schema that omits them documents an API more permissive than the
// one that actually runs.
//
// A nil pointer means the bound was not declared, which is distinct from a
// bound of zero.
type Constraints struct {
	Enum             []string // oneof=draft published archived
	Format           string   // email, uuid, uri, date-time, ipv4…
	Pattern          string   // regular expression the value must match
	Minimum          *float64 // min / gte on a numeric field
	Maximum          *float64 // max / lte on a numeric field
	ExclusiveMinimum *float64 // gt
	ExclusiveMaximum *float64 // lt
	MinLength        *int     // min / len on a string
	MaxLength        *int     // max / len on a string
	MinItems         *int     // min / len on a slice or map
	MaxItems         *int     // max / len on a slice or map
}

// IsZero reports whether no constraint was declared.
func (c *Constraints) IsZero() bool {
	if c == nil {
		return true
	}
	return len(c.Enum) == 0 &&
		c.Format == "" &&
		c.Pattern == "" &&
		c.Minimum == nil &&
		c.Maximum == nil &&
		c.ExclusiveMinimum == nil &&
		c.ExclusiveMaximum == nil &&
		c.MinLength == nil &&
		c.MaxLength == nil &&
		c.MinItems == nil &&
		c.MaxItems == nil
}

// GetEnum returns the declared enum values, tolerating a nil receiver.
func (c *Constraints) GetEnum() []string {
	if c == nil {
		return nil
	}
	return c.Enum
}
