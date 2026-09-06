package openapi

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/pipeline"
)

func fixtureDocument(t *testing.T, name string) *Document {
	t.Helper()
	eps, err := pipeline.RunPipeline(filepath.Join("..", "..", "testdata", name), "./...", nil)
	if err != nil {
		t.Fatal(err)
	}
	return Generate(eps, Config{})
}

func captureResponses(t *testing.T, name string) map[string]any {
	t.Helper()
	path := filepath.Join(t.TempDir(), "responses.json")
	cmd := exec.Command("go", "test", "-count=1", "-run", "^TestCaptureResponses$", ".")
	cmd.Dir = filepath.Join("..", "..", "testdata", name)
	cmd.Env = append(os.Environ(), "GODOCLIVE_RESPONSE_CAPTURE="+path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("capture %s: %v\n%s", name, err, out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var responses map[string]any
	if err := json.Unmarshal(data, &responses); err != nil {
		t.Fatal(err)
	}
	return responses
}

func compileResponseSchema(t *testing.T, doc *Document, schema *Schema) *jsonschema.Schema {
	t.Helper()
	// Retain the document root for the component references in the selected schema.
	resource := map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema", "components": doc.Components, "$ref": "#/$defs/response", "$defs": map[string]any{"response": schema}}
	data, err := json.Marshal(resource)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("https://godoclive.test/response.json", value); err != nil {
		t.Fatal(err)
	}
	result, err := c.Compile("https://godoclive.test/response.json")
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestExistingArrayResponseSchemas(t *testing.T) {
	for _, name := range []string{"echo-basic", "fiber-basic", "stdlib-basic"} {
		t.Run(name, func(t *testing.T) {
			doc := fixtureDocument(t, name)
			schema := doc.Paths["/users"].Get.Responses["200"].Content["application/json"].Schema
			if schema.Type != "array" || schema.Items == nil {
				t.Fatalf("list response = %+v", schema)
			}
			if schema.Items.Ref == "" {
				t.Fatalf("missing item fields: %+v", schema.Items)
			}
			compiled := compileResponseSchema(t, doc, schema)
			if err := compiled.Validate([]any{}); err != nil {
				t.Fatal(err)
			}
			if err := compiled.Validate(map[string]any{}); err == nil {
				t.Fatal("list schema accepted an object")
			}
			if name == "stdlib-basic" {
				if err := compiled.Validate(captureResponses(t, name)["users"]); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestActualArticleResponsesValidate(t *testing.T) {
	doc := fixtureDocument(t, "validation")
	responses := captureResponses(t, "validation")
	schema := doc.Paths["/articles/{id}"].Get.Responses["200"].Content["application/json"].Schema
	compiled := compileResponseSchema(t, doc, schema)
	for name, value := range responses {
		if err := compiled.Validate(value); err != nil {
			t.Errorf("%s response: %v", name, err)
		}
	}
	// Validate the actual marshaled fee directly too, so an alternative cannot hide a wrong fee type.
	fee := compileResponseSchema(t, doc, doc.Components.Schemas["Article"].Properties["fee"])
	if err := fee.Validate(responses["full"].(map[string]any)["fee"]); err != nil {
		t.Fatal(err)
	}
	if err := compiled.Validate(map[string]any{"id": float64(12)}); err == nil {
		t.Fatal("invalid ID type accepted")
	}
}

func TestOperationIDsAreUniqueAndStable(t *testing.T) {
	eps, err := pipeline.RunPipeline(filepath.Join("..", "..", "testdata", "gin-realworld"), "./...", nil)
	if err != nil {
		t.Fatal(err)
	}
	ids := func(doc *Document) map[string]string {
		t.Helper()
		seen := make(map[string]bool)
		result := make(map[string]string)
		for path, item := range doc.Paths {
			for i, op := range pathOperations(item) {
				if op == nil {
					continue
				}
				if op.OperationID == "" || seen[op.OperationID] {
					t.Errorf("duplicate or empty operation ID: %q", op.OperationID)
				}
				seen[op.OperationID] = true
				result[path+string(rune('a'+i))] = op.OperationID
			}
		}
		return result
	}
	forward := ids(Generate(eps, Config{}))
	if len(forward) != 27 {
		t.Fatalf("operations = %d", len(forward))
	}
	slices.Reverse(eps)
	if reverse := ids(Generate(eps, Config{})); !reflect.DeepEqual(forward, reverse) {
		t.Fatal("IDs depend on extraction order")
	}
}

func TestResponseUnionsKeepDifferentAnonymousShapes(t *testing.T) {
	body := func(field string) *model.TypeDef {
		return &model.TypeDef{Kind: model.KindStruct, Fields: []model.FieldDef{{JSONName: field, Type: model.TypeDef{Kind: model.KindPrimitive, Name: "string"}}}}
	}
	doc := Generate([]model.EndpointDef{{Method: "GET", Path: "/items", Responses: []model.ResponseDef{{StatusCode: 200, Body: body("item")}, {StatusCode: 200, Body: body("summary")}}}}, Config{})
	schema := doc.Paths["/items"].Get.Responses["200"].Content["application/json"].Schema
	if len(schema.AnyOf) != 2 {
		t.Fatalf("anonymous shapes collapsed: %+v", schema)
	}
}
