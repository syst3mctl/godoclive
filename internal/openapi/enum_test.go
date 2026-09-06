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

// OpenAPI has one response object per status, so alternative payloads have to
// be merged into it rather than overwriting one another.
func TestResponsesMergeAlternativesIntoAnyOf(t *testing.T) {
	article := &model.TypeDef{Kind: model.KindStruct, Name: "Article", Package: "example.com/api",
		Fields: []model.FieldDef{{Name: "ID", JSONName: "id", Type: model.TypeDef{Kind: model.KindPrimitive, Name: "string"}}}}
	summary := &model.TypeDef{Kind: model.KindStruct, Name: "ArticleSummary", Package: "example.com/api",
		Fields: []model.FieldDef{{Name: "ID", JSONName: "id", Type: model.TypeDef{Kind: model.KindPrimitive, Name: "string"}}}}

	doc := Generate([]model.EndpointDef{{
		Method: "GET", Path: "/articles/{id}",
		Responses: []model.ResponseDef{
			{StatusCode: 200, ContentType: "application/json", Body: summary},
			{StatusCode: 200, ContentType: "application/json", Body: article},
		},
	}}, Config{Title: "t", Version: "1"})

	op := doc.Paths["/articles/{id}"].Get
	media := op.Responses["200"].Content["application/json"]
	if media == nil || media.Schema == nil {
		t.Fatalf("no schema: %+v", op.Responses["200"])
	}
	if len(media.Schema.AnyOf) != 2 {
		t.Fatalf("schema = %+v, want an anyOf of two", media.Schema)
	}
	if media.Schema.Ref != "" {
		t.Errorf("a merged schema must not also be a $ref: %q", media.Schema.Ref)
	}
	for i, alt := range media.Schema.AnyOf {
		if alt.Ref == "" {
			t.Errorf("anyOf[%d] = %+v, want a $ref", i, alt)
		}
	}
}

// A status with one payload keeps the plain schema; an anyOf of one would be
// noise in every spec that has no alternatives.
func TestResponsesKeepASingleShapePlain(t *testing.T) {
	article := &model.TypeDef{Kind: model.KindStruct, Name: "Article", Package: "example.com/api",
		Fields: []model.FieldDef{{Name: "ID", JSONName: "id", Type: model.TypeDef{Kind: model.KindPrimitive, Name: "string"}}}}

	doc := Generate([]model.EndpointDef{{
		Method: "GET", Path: "/articles/{id}",
		Responses: []model.ResponseDef{
			{StatusCode: 200, ContentType: "application/json", Body: article},
		},
	}}, Config{Title: "t", Version: "1"})

	media := doc.Paths["/articles/{id}"].Get.Responses["200"].Content["application/json"]
	if media == nil || media.Schema == nil {
		t.Fatal("no schema")
	}
	if len(media.Schema.AnyOf) != 0 {
		t.Errorf("single shape wrapped in anyOf: %+v", media.Schema)
	}
	if media.Schema.Ref == "" {
		t.Errorf("schema = %+v, want a $ref", media.Schema)
	}
}
