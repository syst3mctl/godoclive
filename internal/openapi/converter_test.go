package openapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dicki/godoclive/internal/model"
)

func TestBasicEndpointConversion(t *testing.T) {
	endpoints := []model.EndpointDef{
		{
			Method:  "GET",
			Path:    "/users/{id}",
			Summary: "Get User By ID",
			Tags:    []string{"users"},
			Request: model.RequestDef{
				PathParams: []model.ParamDef{
					{Name: "id", In: "path", Type: "uuid", Required: true},
				},
				QueryParams: []model.ParamDef{
					{Name: "fields", In: "query", Type: "string", Required: false},
				},
			},
			Responses: []model.ResponseDef{
				{StatusCode: 200, Description: "OK", ContentType: "application/json"},
				{StatusCode: 404, Description: "Not found"},
			},
		},
	}

	doc := Generate(endpoints, Config{Title: "Test API", Version: "1.0.0"})

	if doc.OpenAPI != "3.1.0" {
		t.Errorf("expected openapi 3.1.0, got %s", doc.OpenAPI)
	}
	if doc.Info.Title != "Test API" {
		t.Errorf("expected title 'Test API', got %s", doc.Info.Title)
	}

	pathItem, ok := doc.Paths["/users/{id}"]
	if !ok {
		t.Fatal("expected path /users/{id}")
	}
	if pathItem.Get == nil {
		t.Fatal("expected GET operation")
	}

	op := pathItem.Get
	if op.OperationID != "getUsersId" {
		t.Errorf("expected operationId 'getUsersId', got %s", op.OperationID)
	}
	if op.Summary != "Get User By ID" {
		t.Errorf("expected summary 'Get User By ID', got %s", op.Summary)
	}
	if len(op.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(op.Parameters))
	}

	// Path param.
	p0 := op.Parameters[0]
	if p0.Name != "id" || p0.In != "path" || !p0.Required {
		t.Errorf("unexpected path param: %+v", p0)
	}
	if p0.Schema.Format != "uuid" {
		t.Errorf("expected uuid format, got %s", p0.Schema.Format)
	}

	// Query param.
	p1 := op.Parameters[1]
	if p1.Name != "fields" || p1.In != "query" || p1.Required {
		t.Errorf("unexpected query param: %+v", p1)
	}

	// Responses.
	if len(op.Responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(op.Responses))
	}
	if _, ok := op.Responses["200"]; !ok {
		t.Error("expected 200 response")
	}
	if _, ok := op.Responses["404"]; !ok {
		t.Error("expected 404 response")
	}
}

func TestRequestBodyWithStruct(t *testing.T) {
	endpoints := []model.EndpointDef{
		{
			Method: "POST",
			Path:   "/users",
			Request: model.RequestDef{
				Body: &model.TypeDef{
					Name: "CreateUserRequest",
					Kind: model.KindStruct,
					Fields: []model.FieldDef{
						{Name: "Name", JSONName: "name", Type: model.TypeDef{Name: "string", Kind: model.KindPrimitive}, Required: true},
						{Name: "Email", JSONName: "email", Type: model.TypeDef{Name: "string", Kind: model.KindPrimitive}, Required: true},
						{Name: "Age", JSONName: "age", Type: model.TypeDef{Name: "int", Kind: model.KindPrimitive}},
					},
				},
				ContentType: "application/json",
			},
			Responses: []model.ResponseDef{
				{StatusCode: 201, Description: "Created"},
			},
		},
	}

	doc := Generate(endpoints, Config{})

	op := doc.Paths["/users"].Post
	if op == nil {
		t.Fatal("expected POST operation")
	}
	if op.RequestBody == nil {
		t.Fatal("expected request body")
	}

	mt, ok := op.RequestBody.Content["application/json"]
	if !ok {
		t.Fatal("expected application/json content type")
	}
	if mt.Schema == nil || mt.Schema.Ref == "" {
		t.Fatal("expected $ref schema for request body")
	}
	if !strings.Contains(mt.Schema.Ref, "CreateUserRequest") {
		t.Errorf("expected ref to CreateUserRequest, got %s", mt.Schema.Ref)
	}

	// Check components/schemas.
	if doc.Components == nil || doc.Components.Schemas == nil {
		t.Fatal("expected components/schemas")
	}
	schema, ok := doc.Components.Schemas["CreateUserRequest"]
	if !ok {
		t.Fatal("expected CreateUserRequest in schemas")
	}
	if len(schema.Properties) != 3 {
		t.Errorf("expected 3 properties, got %d", len(schema.Properties))
	}
	if len(schema.Required) != 2 {
		t.Errorf("expected 2 required fields, got %d", len(schema.Required))
	}
}

func TestAuthSecuritySchemes(t *testing.T) {
	endpoints := []model.EndpointDef{
		{
			Method: "GET",
			Path:   "/protected",
			Auth: model.AuthDef{
				Required: true,
				Schemes:  []model.AuthScheme{model.AuthBearer},
			},
			Responses: []model.ResponseDef{{StatusCode: 200}},
		},
		{
			Method: "GET",
			Path:   "/api-key",
			Auth: model.AuthDef{
				Required: true,
				Schemes:  []model.AuthScheme{model.AuthAPIKey},
			},
			Responses: []model.ResponseDef{{StatusCode: 200}},
		},
	}

	doc := Generate(endpoints, Config{})

	if doc.Components == nil || doc.Components.SecuritySchemes == nil {
		t.Fatal("expected security schemes in components")
	}

	bearer, ok := doc.Components.SecuritySchemes["bearerAuth"]
	if !ok {
		t.Fatal("expected bearerAuth scheme")
	}
	if bearer.Type != "http" || bearer.Scheme != "bearer" {
		t.Errorf("unexpected bearer scheme: %+v", bearer)
	}

	apiKey, ok := doc.Components.SecuritySchemes["apiKeyAuth"]
	if !ok {
		t.Fatal("expected apiKeyAuth scheme")
	}
	if apiKey.Type != "apiKey" || apiKey.In != "header" {
		t.Errorf("unexpected apiKey scheme: %+v", apiKey)
	}

	// Check operation-level security.
	op := doc.Paths["/protected"].Get
	if len(op.Security) != 1 {
		t.Fatalf("expected 1 security requirement, got %d", len(op.Security))
	}
	if _, ok := op.Security[0]["bearerAuth"]; !ok {
		t.Error("expected bearerAuth in operation security")
	}
}

func TestSliceResponseBody(t *testing.T) {
	endpoints := []model.EndpointDef{
		{
			Method: "GET",
			Path:   "/users",
			Responses: []model.ResponseDef{
				{
					StatusCode:  200,
					ContentType: "application/json",
					Body: &model.TypeDef{
						Name: "[]User",
						Kind: model.KindSlice,
						Elem: &model.TypeDef{
							Name: "User",
							Kind: model.KindStruct,
							Fields: []model.FieldDef{
								{Name: "ID", JSONName: "id", Type: model.TypeDef{Name: "int", Kind: model.KindPrimitive}},
								{Name: "Name", JSONName: "name", Type: model.TypeDef{Name: "string", Kind: model.KindPrimitive}},
							},
						},
					},
				},
			},
		},
	}

	doc := Generate(endpoints, Config{})

	resp := doc.Paths["/users"].Get.Responses["200"]
	if resp == nil {
		t.Fatal("expected 200 response")
	}
	mt := resp.Content["application/json"]
	if mt == nil || mt.Schema == nil {
		t.Fatal("expected schema in response content")
	}
	if mt.Schema.Type != "array" {
		t.Errorf("expected array type, got %s", mt.Schema.Type)
	}
	if mt.Schema.Items == nil || mt.Schema.Items.Ref == "" {
		t.Error("expected $ref items in array schema")
	}
	if !strings.Contains(mt.Schema.Items.Ref, "User") {
		t.Errorf("expected User ref, got %s", mt.Schema.Items.Ref)
	}
}

func TestANYMethodExpansion(t *testing.T) {
	endpoints := []model.EndpointDef{
		{
			Method:    "ANY",
			Path:      "/health",
			Responses: []model.ResponseDef{{StatusCode: 200}},
		},
	}

	doc := Generate(endpoints, Config{})

	pathItem := doc.Paths["/health"]
	if pathItem == nil {
		t.Fatal("expected /health path")
	}

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, m := range methods {
		var op *Operation
		switch m {
		case "GET":
			op = pathItem.Get
		case "POST":
			op = pathItem.Post
		case "PUT":
			op = pathItem.Put
		case "DELETE":
			op = pathItem.Delete
		case "PATCH":
			op = pathItem.Patch
		case "HEAD":
			op = pathItem.Head
		case "OPTIONS":
			op = pathItem.Options
		}
		if op == nil {
			t.Errorf("expected %s operation on /health", m)
		}
	}
}

func TestJSONOutputValidity(t *testing.T) {
	endpoints := []model.EndpointDef{
		{
			Method:    "GET",
			Path:      "/test",
			Summary:   "Test endpoint",
			Tags:      []string{"test"},
			Responses: []model.ResponseDef{{StatusCode: 200, Description: "OK"}},
		},
	}

	doc := Generate(endpoints, Config{Title: "Test", Version: "1.0.0"})

	data, err := Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Verify it's valid JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	// Check key fields exist.
	if parsed["openapi"] != "3.1.0" {
		t.Errorf("expected openapi 3.1.0 in JSON")
	}
}

func TestMapType(t *testing.T) {
	endpoints := []model.EndpointDef{
		{
			Method: "GET",
			Path:   "/config",
			Responses: []model.ResponseDef{
				{
					StatusCode:  200,
					ContentType: "application/json",
					Body: &model.TypeDef{
						Name: "map[string]interface{}",
						Kind: model.KindMap,
						Elem: &model.TypeDef{
							Name: "interface{}",
							Kind: model.KindInterface,
						},
					},
				},
			},
		},
	}

	doc := Generate(endpoints, Config{})

	resp := doc.Paths["/config"].Get.Responses["200"]
	mt := resp.Content["application/json"]
	if mt.Schema.Type != "object" {
		t.Errorf("expected object type for map, got %s", mt.Schema.Type)
	}
	if mt.Schema.AdditionalProperties == nil {
		t.Error("expected additionalProperties for map type")
	}
}

func TestDefaultResponseWhenNone(t *testing.T) {
	endpoints := []model.EndpointDef{
		{
			Method: "DELETE",
			Path:   "/items/{id}",
		},
	}

	doc := Generate(endpoints, Config{})

	op := doc.Paths["/items/{id}"].Delete
	if op == nil {
		t.Fatal("expected DELETE operation")
	}
	resp, ok := op.Responses["200"]
	if !ok {
		t.Fatal("expected default 200 response")
	}
	if resp.Description != "Successful response" {
		t.Errorf("expected default description, got %s", resp.Description)
	}
}

func TestDeprecatedEndpoint(t *testing.T) {
	endpoints := []model.EndpointDef{
		{
			Method:     "GET",
			Path:       "/old",
			Deprecated: true,
			Responses:  []model.ResponseDef{{StatusCode: 200}},
		},
	}

	doc := Generate(endpoints, Config{})
	op := doc.Paths["/old"].Get
	if !op.Deprecated {
		t.Error("expected deprecated flag on operation")
	}
}

func TestNullableAndDeprecatedFields(t *testing.T) {
	endpoints := []model.EndpointDef{
		{
			Method: "POST",
			Path:   "/items",
			Request: model.RequestDef{
				Body: &model.TypeDef{
					Name: "Item",
					Kind: model.KindStruct,
					Fields: []model.FieldDef{
						{Name: "Name", JSONName: "name", Type: model.TypeDef{Name: "string", Kind: model.KindPrimitive}},
						{Name: "Notes", JSONName: "notes", Type: model.TypeDef{Name: "string", Kind: model.KindPrimitive}, Nullable: true},
						{Name: "OldField", JSONName: "old_field", Type: model.TypeDef{Name: "string", Kind: model.KindPrimitive}, Deprecated: true},
					},
				},
			},
			Responses: []model.ResponseDef{{StatusCode: 201}},
		},
	}

	doc := Generate(endpoints, Config{})

	schema := doc.Components.Schemas["Item"]
	if schema == nil {
		t.Fatal("expected Item schema")
	}

	notes := schema.Properties["notes"]
	if notes == nil || !notes.Nullable {
		t.Error("expected nullable notes field")
	}

	oldField := schema.Properties["old_field"]
	if oldField == nil || !oldField.Deprecated {
		t.Error("expected deprecated old_field")
	}
}

func TestConfigPassthrough(t *testing.T) {
	doc := Generate(nil, Config{
		Title:       "My API",
		Description: "Test description",
		Version:     "2.0.0",
		Servers:     []Server{{URL: "https://api.example.com"}},
		Contact:     &Contact{Name: "Dev", Email: "dev@example.com"},
		License:     &License{Name: "MIT"},
	})

	if doc.Info.Title != "My API" {
		t.Errorf("expected title 'My API', got %s", doc.Info.Title)
	}
	if doc.Info.Description != "Test description" {
		t.Errorf("expected description")
	}
	if doc.Info.Version != "2.0.0" {
		t.Errorf("expected version 2.0.0")
	}
	if doc.Info.Contact == nil || doc.Info.Contact.Email != "dev@example.com" {
		t.Error("expected contact info")
	}
	if doc.Info.License == nil || doc.Info.License.Name != "MIT" {
		t.Error("expected license info")
	}
	if len(doc.Servers) != 1 || doc.Servers[0].URL != "https://api.example.com" {
		t.Error("expected server")
	}
}
