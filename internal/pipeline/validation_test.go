package pipeline_test

import (
	"testing"

	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/pipeline"
)

// Validator rules are enforced at runtime. A schema that omits them documents
// an API more permissive than the one that actually runs.

func fieldByJSON(t *testing.T, td *model.TypeDef, jsonName string) model.FieldDef {
	t.Helper()
	if td == nil {
		t.Fatalf("no type to look up %q in", jsonName)
	}
	for _, f := range td.Fields {
		if f.JSONName == jsonName {
			return f
		}
	}
	t.Fatalf("field %q not found in %s", jsonName, td.Name)
	return model.FieldDef{}
}

func articleBody(t *testing.T) *model.TypeDef {
	t.Helper()
	eps, err := pipeline.RunPipeline(testdataDir("validation"), "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	ep := findEndpoint(eps, "POST", "/articles")
	if ep == nil {
		t.Fatal("POST /articles not found")
	}
	if ep.Request.Body == nil {
		t.Fatal("POST /articles has no request body")
	}
	return ep.Request.Body
}

func TestPipeline_ValidatorTagsBecomeConstraints(t *testing.T) {
	body := articleBody(t)

	t.Run("min and max on a string bound its length", func(t *testing.T) {
		c := fieldByJSON(t, body, "title").Constraints
		if c == nil || c.MinLength == nil || *c.MinLength != 3 {
			t.Fatalf("title MinLength = %v, want 3", c)
		}
		if c.MaxLength == nil || *c.MaxLength != 120 {
			t.Errorf("title MaxLength = %v, want 120", c.MaxLength)
		}
		if c.Minimum != nil || c.MinItems != nil {
			t.Errorf("title bound recorded as a value or item count: %+v", c)
		}
	})

	t.Run("gte and lte on an integer bound its value", func(t *testing.T) {
		c := fieldByJSON(t, body, "word_count").Constraints
		if c == nil || c.Minimum == nil || *c.Minimum != 300 {
			t.Fatalf("word_count Minimum = %v, want 300", c)
		}
		if c.Maximum == nil || *c.Maximum != 5000 {
			t.Errorf("word_count Maximum = %v, want 5000", c.Maximum)
		}
		if c.MinLength != nil {
			t.Errorf("word_count bound recorded as a length: %+v", c)
		}
	})

	t.Run("min and max on a slice bound its item count", func(t *testing.T) {
		c := fieldByJSON(t, body, "tags").Constraints
		if c == nil || c.MinItems == nil || *c.MinItems != 1 {
			t.Fatalf("tags MinItems = %v, want 1", c)
		}
		if c.MaxItems == nil || *c.MaxItems != 5 {
			t.Errorf("tags MaxItems = %v, want 5", c.MaxItems)
		}
	})

	t.Run("gt and lt are exclusive bounds", func(t *testing.T) {
		c := fieldByJSON(t, body, "score").Constraints
		if c == nil || c.ExclusiveMinimum == nil || *c.ExclusiveMinimum != 0 {
			t.Fatalf("score ExclusiveMinimum = %v, want 0", c)
		}
		if c.ExclusiveMaximum == nil || *c.ExclusiveMaximum != 10 {
			t.Errorf("score ExclusiveMaximum = %v, want 10", c.ExclusiveMaximum)
		}
		if c.Minimum != nil || c.Maximum != nil {
			t.Errorf("score exclusive bound recorded as inclusive: %+v", c)
		}
	})

	t.Run("len sets both ends", func(t *testing.T) {
		c := fieldByJSON(t, body, "country_code").Constraints
		if c == nil || c.MinLength == nil || c.MaxLength == nil {
			t.Fatalf("country_code = %+v, want both length bounds", c)
		}
		if *c.MinLength != 2 || *c.MaxLength != 2 {
			t.Errorf("country_code length = %d–%d, want 2–2", *c.MinLength, *c.MaxLength)
		}
	})

	t.Run("oneof becomes an enum", func(t *testing.T) {
		c := fieldByJSON(t, body, "visibility").Constraints
		want := []string{"public", "unlisted", "private"}
		if c == nil || len(c.Enum) != len(want) {
			t.Fatalf("visibility Enum = %v, want %v", c, want)
		}
		for i := range want {
			if c.Enum[i] != want[i] {
				t.Errorf("visibility Enum[%d] = %q, want %q", i, c.Enum[i], want[i])
			}
		}
	})

	t.Run("named rules become formats", func(t *testing.T) {
		for jsonName, want := range map[string]string{
			"author_email": "email",
			"homepage":     "uri",
			"reviewer_id":  "uuid",
		} {
			c := fieldByJSON(t, body, jsonName).Constraints
			if c == nil || c.Format != want {
				t.Errorf("%s Format = %v, want %q", jsonName, c, want)
			}
		}
	})

	t.Run("character-class rules become patterns", func(t *testing.T) {
		c := fieldByJSON(t, body, "slug").Constraints
		if c == nil || c.Pattern == "" {
			t.Errorf("slug Pattern = %v, want a regular expression", c)
		}
	})

	t.Run("a field with no rules has no constraints", func(t *testing.T) {
		if c := fieldByJSON(t, body, "status").Constraints; c != nil && c.Format != "" {
			t.Errorf("status carries a format it never declared: %+v", c)
		}
	})
}

// A named type over a string or an integer with a const block is how Go spells
// an enumeration, whether or not anyone also wrote a oneof rule.
func TestPipeline_ConstBlocksBecomeEnums(t *testing.T) {
	body := articleBody(t)

	t.Run("string constants, in declaration order", func(t *testing.T) {
		c := fieldByJSON(t, body, "status").Constraints
		want := []string{"draft", "in_review", "published", "archived"}
		if c == nil || len(c.Enum) != len(want) {
			t.Fatalf("status Enum = %v, want %v", c, want)
		}
		for i := range want {
			if c.Enum[i] != want[i] {
				t.Errorf("status Enum[%d] = %q, want %q — declaration order lost", i, c.Enum[i], want[i])
			}
		}
	})

	t.Run("iota constants", func(t *testing.T) {
		c := fieldByJSON(t, body, "priority").Constraints
		want := []string{"1", "2", "3"}
		if c == nil || len(c.Enum) != len(want) {
			t.Fatalf("priority Enum = %v, want %v", c, want)
		}
		for i := range want {
			if c.Enum[i] != want[i] {
				t.Errorf("priority Enum[%d] = %q, want %q", i, c.Enum[i], want[i])
			}
		}
	})

	// An example outside the enum tells a reader to send a value the server
	// will reject.
	t.Run("the example is a member of the enum", func(t *testing.T) {
		if got := fieldByJSON(t, body, "status").Example; got != "draft" {
			t.Errorf("status Example = %v, want \"draft\"", got)
		}
		if got := fieldByJSON(t, body, "priority").Example; got != int64(1) {
			t.Errorf("priority Example = %v (%T), want int64(1)", got, got)
		}
	})
}

// A cookie is an input to the endpoint exactly as a header is, and OpenAPI
// gives it its own location.
func TestPipeline_CookiesAreExtracted(t *testing.T) {
	eps, err := pipeline.RunPipeline(testdataDir("validation"), "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	ep := findEndpoint(eps, "GET", "/articles")
	if ep == nil {
		t.Fatal("GET /articles not found")
	}

	want := []string{"session_id", "csrf_token"}
	if len(ep.Request.Cookies) != len(want) {
		t.Fatalf("Cookies = %+v, want %v", ep.Request.Cookies, want)
	}
	for i, name := range want {
		c := ep.Request.Cookies[i]
		if c.Name != name {
			t.Errorf("Cookies[%d].Name = %q, want %q", i, c.Name, name)
		}
		if c.In != "cookie" {
			t.Errorf("Cookies[%d].In = %q, want \"cookie\"", i, c.In)
		}
		if c.Type != "string" {
			t.Errorf("Cookies[%d].Type = %q, want \"string\"", i, c.Type)
		}
	}

	// A cookie must not also be reported as a header.
	for _, h := range ep.Request.Headers {
		if h.Name == "session_id" || h.Name == "csrf_token" {
			t.Errorf("cookie %q also reported as a header", h.Name)
		}
	}
}

// A handler that answers one status with more than one payload documents both.
// Keeping only whichever branch the walk reached first told a client to expect
// a shape it might never receive, with nothing to say the other existed.
func TestPipeline_MultipleShapesUnderOneStatus(t *testing.T) {
	eps, err := pipeline.RunPipeline(testdataDir("validation"), "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	ep := findEndpoint(eps, "GET", "/articles/{id}")
	if ep == nil {
		t.Fatal("GET /articles/{id} not found")
	}

	var shapes []string
	for _, r := range ep.Responses {
		if r.StatusCode != 200 || r.Body == nil {
			continue
		}
		shapes = append(shapes, r.Body.Name)
	}
	if len(shapes) != 2 {
		t.Fatalf("200 shapes = %v, want both ArticleSummary and Article", shapes)
	}
	found := map[string]bool{shapes[0]: true, shapes[1]: true}
	for _, want := range []string{"ArticleSummary", "Article"} {
		if !found[want] {
			t.Errorf("200 shapes = %v, missing %s", shapes, want)
		}
	}
}

// A status written from several branches with the same payload is one response,
// not four.
func TestPipeline_RepeatedShapeCollapses(t *testing.T) {
	eps, err := pipeline.RunPipeline(testdataDir("validation"), "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	for _, ep := range eps {
		counts := make(map[string]int)
		for _, r := range ep.Responses {
			key := r.Description
			if r.Body != nil {
				key = r.Body.Name
			}
			counts[string(rune(r.StatusCode))+"|"+key]++
		}
		for key, n := range counts {
			if n > 1 {
				t.Errorf("%s %s: %q recorded %d times", ep.Method, ep.Path, key, n)
			}
		}
	}
}
