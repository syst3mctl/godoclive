package pipeline_test

import (
	"testing"

	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/openapi"
	"github.com/syst3mctl/godoclive/internal/pipeline"
)

// time.Time marshals to an RFC 3339 string. Expanding its fields documented
// wall, ext and loc — Go runtime internals — and dragged Location, zone and
// zoneTrans into the spec alongside them.
func TestPipeline_TimeIsAStringNotAStruct(t *testing.T) {
	eps, err := pipeline.RunPipeline(testdataDir("validation"), "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	var article *model.TypeDef
	for _, ep := range eps {
		for _, r := range ep.Responses {
			if r.Body != nil && r.Body.Name == "Article" {
				article = r.Body
			}
		}
	}
	if article == nil {
		t.Fatal("no Article response body found")
	}

	var found bool
	for _, f := range article.Fields {
		if f.JSONName != "created_at" {
			continue
		}
		found = true
		if f.Type.Kind != model.KindPrimitive {
			t.Errorf("created_at Kind = %q, want primitive", f.Type.Kind)
		}
		if f.Type.Name != "string" {
			t.Errorf("created_at Name = %q, want string", f.Type.Name)
		}
		if f.Type.Format != "date-time" {
			t.Errorf("created_at Format = %q, want date-time", f.Type.Format)
		}
		if len(f.Type.Fields) != 0 {
			t.Errorf("created_at expanded into %d fields", len(f.Type.Fields))
		}
	}
	if !found {
		t.Fatal("Article has no created_at field")
	}
}

// The internals time.Time drags along must not reach components/schemas.
func TestOpenAPI_NoRuntimeInternalsInComponents(t *testing.T) {
	eps, err := pipeline.RunPipeline(testdataDir("validation"), "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	doc := openapi.Generate(eps, openapi.Config{Title: "t", Version: "1"})

	for _, leaked := range []string{"Time", "Location", "zone", "zoneTrans"} {
		if _, ok := doc.Components.Schemas[leaked]; ok {
			t.Errorf("component schema %q leaked from time.Time internals", leaked)
		}
	}

	// And the field itself carries the format.
	schema := doc.Components.Schemas["Article"]
	if schema == nil {
		t.Fatal("no Article schema")
	}
	createdAt := schema.Properties["created_at"]
	if createdAt == nil {
		t.Fatal("Article has no created_at property")
	}
	if createdAt.Type != "string" || createdAt.Format != "date-time" {
		t.Errorf("created_at = {type: %q, format: %q}, want string/date-time", createdAt.Type, createdAt.Format)
	}
}

// A type with its own MarshalJSON does not marshal as its fields, and what it
// does marshal as is only knowable by reading that method's body. Publishing
// the fields would document the one shape known to be wrong.
func TestPipeline_CustomMarshalerIsNotExpanded(t *testing.T) {
	eps, err := pipeline.RunPipeline(testdataDir("validation"), "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	var article *model.TypeDef
	for _, ep := range eps {
		for _, r := range ep.Responses {
			if r.Body != nil && r.Body.Name == "Article" {
				article = r.Body
			}
		}
	}
	if article == nil {
		t.Fatal("no Article response body found")
	}

	var found bool
	for _, f := range article.Fields {
		if f.JSONName != "fee" {
			continue
		}
		found = true
		if len(f.Type.Fields) != 0 {
			t.Errorf("fee expanded into %d fields despite its MarshalJSON", len(f.Type.Fields))
		}
		if f.Type.Kind == model.KindStruct {
			t.Errorf("fee Kind = %q, want something other than struct", f.Type.Kind)
		}
	}
	if !found {
		t.Fatal("Article has no fee field")
	}

	doc := openapi.Generate(eps, openapi.Config{Title: "t", Version: "1"})
	if _, ok := doc.Components.Schemas["Money"]; ok {
		t.Error("Money reached components/schemas with its internals")
	}
}
