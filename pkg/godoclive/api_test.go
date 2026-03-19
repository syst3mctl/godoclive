package godoclive_test

import (
	"encoding/json"
	"testing"

	"github.com/dicki/godoclive/internal/model"
	godoclive "github.com/dicki/godoclive/pkg/godoclive"
)

func TestGenerateOpenAPI_WithDescription(t *testing.T) {
	endpoints := []model.EndpointDef{
		{
			Method:    "GET",
			Path:      "/health",
			Summary:   "Health check",
			Responses: []model.ResponseDef{{StatusCode: 200, Description: "OK"}},
		},
	}

	data, err := godoclive.GenerateOpenAPI(endpoints,
		godoclive.WithTitle("Test API"),
		godoclive.WithDescription("A detailed description of the Test API."),
		godoclive.WithVersion("1.2.3"),
	)
	if err != nil {
		t.Fatalf("GenerateOpenAPI: %v", err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	info, ok := doc["info"].(map[string]interface{})
	if !ok {
		t.Fatal("expected info object in OpenAPI document")
	}

	if got := info["description"]; got != "A detailed description of the Test API." {
		t.Errorf("expected description, got %v", got)
	}
	if got := info["title"]; got != "Test API" {
		t.Errorf("expected title Test API, got %v", got)
	}
	if got := info["version"]; got != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %v", got)
	}
}

func TestGenerateOpenAPI_WithoutDescription(t *testing.T) {
	endpoints := []model.EndpointDef{
		{
			Method:    "GET",
			Path:      "/ping",
			Responses: []model.ResponseDef{{StatusCode: 200}},
		},
	}

	data, err := godoclive.GenerateOpenAPI(endpoints,
		godoclive.WithTitle("Minimal API"),
	)
	if err != nil {
		t.Fatalf("GenerateOpenAPI: %v", err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	info, ok := doc["info"].(map[string]interface{})
	if !ok {
		t.Fatal("expected info object in OpenAPI document")
	}

	// Description should be omitted when not set.
	if _, present := info["description"]; present {
		t.Error("expected description to be absent when not set")
	}
}
