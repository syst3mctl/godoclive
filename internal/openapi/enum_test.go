package openapi

import (
	"encoding/json"
	"testing"

	"github.com/syst3mctl/godoclive/internal/model"
)

// An enum is carried through the analyzer as strings — a constant's value and
// a oneof argument are both source text. Emitting those strings under an
// integer schema produces a document nothing can satisfy.
func TestSchemaEnumMatchesTheSchemaType(t *testing.T) {
	tests := []struct {
		name       string
		schemaType string
		enum       []string
		want       string
	}{
		{
			name:       "an integer schema gets numbers",
			schemaType: "integer",
			enum:       []string{"1", "2", "3"},
			want:       `[1,2,3]`,
		},
		{
			name:       "a number schema gets numbers",
			schemaType: "number",
			enum:       []string{"0.5", "1.5"},
			want:       `[0.5,1.5]`,
		},
		{
			name:       "a string schema keeps strings",
			schemaType: "string",
			enum:       []string{"draft", "published"},
			want:       `["draft","published"]`,
		},
		{
			name:       "an unparseable member under a numeric schema stays a string",
			schemaType: "integer",
			enum:       []string{"1", "unbounded"},
			want:       `[1,"unbounded"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Schema{Type: tt.schemaType}
			s.applyConstraints(&model.Constraints{Enum: tt.enum})

			got, err := json.Marshal(s.Enum)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("enum = %s, want %s", got, tt.want)
			}
		})
	}
}

// A nil or empty constraint set must leave the schema untouched so that
// nothing spurious appears in the document.
func TestApplyConstraintsIgnoresEmpty(t *testing.T) {
	s := &Schema{Type: "string", Format: "date-time"}
	s.applyConstraints(nil)
	s.applyConstraints(&model.Constraints{})

	if s.Format != "date-time" {
		t.Errorf("Format = %q, want it left alone", s.Format)
	}
	if s.Enum != nil || s.Minimum != nil || s.MaxLength != nil {
		t.Errorf("empty constraints wrote to the schema: %+v", s)
	}
}
