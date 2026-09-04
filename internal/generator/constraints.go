package generator

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/syst3mctl/godoclive/internal/model"
)

// maxEnumChipValues caps how many enum members are spelled out before the chip
// says how many more there are. A status enum of four reads well inline; a
// currency enum of a hundred and eighty would push every other column off the
// card.
const maxEnumChipValues = 6

// describeConstraints renders a field's declared constraints as short chips for
// the docs table.
//
// The chips say what a client must send, in the units the reader is thinking
// in: a string's bound is a character count, a slice's is an item count, and a
// number's is the value itself. Spelling all three as a bare "min 3" would make
// the reader guess which.
func describeConstraints(c *model.Constraints) []string {
	if c.IsZero() {
		return nil
	}
	var chips []string

	if len(c.Enum) > 0 {
		chips = append(chips, enumChip(c.Enum))
	}
	if c.Format != "" {
		chips = append(chips, "format: "+c.Format)
	}
	if chip := rangeChip(c.MinLength, c.MaxLength, "char"); chip != "" {
		chips = append(chips, chip)
	}
	if chip := rangeChip(c.MinItems, c.MaxItems, "item"); chip != "" {
		chips = append(chips, chip)
	}
	if chip := numericChip(c); chip != "" {
		chips = append(chips, chip)
	}
	if c.Pattern != "" {
		chips = append(chips, "pattern: "+c.Pattern)
	}
	return chips
}

// enumChip lists the permitted values, naming the count of any it elides.
func enumChip(values []string) string {
	if len(values) <= maxEnumChipValues {
		return "one of: " + strings.Join(values, " | ")
	}
	shown := strings.Join(values[:maxEnumChipValues], " | ")
	return fmt.Sprintf("one of: %s | +%d more", shown, len(values)-maxEnumChipValues)
}

// rangeChip renders a count bound — characters or items — as one phrase.
func rangeChip(min, max *int, unit string) string {
	plural := func(n int) string {
		if n == 1 {
			return unit
		}
		return unit + "s"
	}
	switch {
	case min != nil && max != nil && *min == *max:
		return fmt.Sprintf("exactly %d %s", *min, plural(*min))
	case min != nil && max != nil:
		return fmt.Sprintf("%d–%d %s", *min, *max, plural(*max))
	case min != nil:
		return fmt.Sprintf("≥ %d %s", *min, plural(*min))
	case max != nil:
		return fmt.Sprintf("≤ %d %s", *max, plural(*max))
	}
	return ""
}

// numericChip renders the bounds on a number's value. Inclusive and exclusive
// bounds are distinct constraints and may both be present.
func numericChip(c *model.Constraints) string {
	var parts []string
	if c.Minimum != nil && c.Maximum != nil && *c.Minimum == *c.Maximum {
		return "exactly " + formatNumber(*c.Minimum)
	}
	if c.Minimum != nil {
		parts = append(parts, "≥ "+formatNumber(*c.Minimum))
	}
	if c.ExclusiveMinimum != nil {
		parts = append(parts, "> "+formatNumber(*c.ExclusiveMinimum))
	}
	if c.Maximum != nil {
		parts = append(parts, "≤ "+formatNumber(*c.Maximum))
	}
	if c.ExclusiveMaximum != nil {
		parts = append(parts, "< "+formatNumber(*c.ExclusiveMaximum))
	}
	return strings.Join(parts, ", ")
}

// formatNumber prints a bound without a trailing ".0" on whole values.
func formatNumber(n float64) string {
	return strconv.FormatFloat(n, 'f', -1, 64)
}
