package generator

import (
	"reflect"
	"testing"

	"github.com/syst3mctl/godoclive/internal/model"
)

func f64(v float64) *float64 { return &v }
func iptr(v int) *int        { return &v }

func TestDescribeConstraints(t *testing.T) {
	tests := []struct {
		name string
		in   *model.Constraints
		want []string
	}{
		{
			name: "nil constraints produce no chips",
			in:   nil,
		},
		{
			name: "a string length range reads in characters",
			in:   &model.Constraints{MinLength: iptr(3), MaxLength: iptr(120)},
			want: []string{"3–120 chars"},
		},
		{
			name: "equal bounds collapse to an exact length",
			in:   &model.Constraints{MinLength: iptr(2), MaxLength: iptr(2)},
			want: []string{"exactly 2 chars"},
		},
		{
			name: "an exact length of one is singular",
			in:   &model.Constraints{MinLength: iptr(1), MaxLength: iptr(1)},
			want: []string{"exactly 1 char"},
		},
		{
			name: "a slice bound reads in items",
			in:   &model.Constraints{MinItems: iptr(1), MaxItems: iptr(5)},
			want: []string{"1–5 items"},
		},
		{
			name: "inclusive numeric bounds",
			in:   &model.Constraints{Minimum: f64(300), Maximum: f64(5000)},
			want: []string{"≥ 300, ≤ 5000"},
		},
		{
			name: "exclusive numeric bounds are distinct from inclusive ones",
			in:   &model.Constraints{ExclusiveMinimum: f64(0), ExclusiveMaximum: f64(10)},
			want: []string{"> 0, < 10"},
		},
		{
			name: "a fractional bound keeps its decimals and a whole one loses none",
			in:   &model.Constraints{Minimum: f64(0.5), Maximum: f64(10)},
			want: []string{"≥ 0.5, ≤ 10"},
		},
		{
			name: "an enum lists its values",
			in:   &model.Constraints{Enum: []string{"draft", "published"}},
			want: []string{"one of: draft | published"},
		},
		{
			name: "a long enum names how many it elides",
			in:   &model.Constraints{Enum: []string{"a", "b", "c", "d", "e", "f", "g", "h"}},
			want: []string{"one of: a | b | c | d | e | f | +2 more"},
		},
		{
			name: "format and pattern each get a chip",
			in:   &model.Constraints{Format: "email", Pattern: "^[a-z]+$"},
			want: []string{"format: email", "pattern: ^[a-z]+$"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := describeConstraints(tt.in)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("describeConstraints() = %v, want %v", got, tt.want)
			}
		})
	}
}
