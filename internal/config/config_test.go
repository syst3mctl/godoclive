package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dicki/godoclive/internal/config"
	"github.com/dicki/godoclive/internal/model"
)

const sampleYAML = `title: "My API"
version: "v1.2.0"
base_url: "https://api.myapp.com"
theme: "dark"

overrides:
  - path: "POST /users"
    summary: "Register a new user"
    tags: ["auth", "users"]
    responses:
      - status: 409
        description: "Email already in use"

exclude:
  - "GET /internal/*"
  - "GET /debug/*"
  - "GET /metrics"

auth:
  header: "X-Auth-Token"
  scheme: "apikey"

response_helpers:
  - "respond"
  - "writeJSON"
`

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, ".godoclive.yaml"), []byte(sampleYAML), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	if cfg.Title != "My API" {
		t.Errorf("Title = %q, want %q", cfg.Title, "My API")
	}
	if cfg.Version != "v1.2.0" {
		t.Errorf("Version = %q, want %q", cfg.Version, "v1.2.0")
	}
	if cfg.BaseURL != "https://api.myapp.com" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "https://api.myapp.com")
	}
	if cfg.Theme != "dark" {
		t.Errorf("Theme = %q, want %q", cfg.Theme, "dark")
	}
	if cfg.Auth.Header != "X-Auth-Token" {
		t.Errorf("Auth.Header = %q, want %q", cfg.Auth.Header, "X-Auth-Token")
	}
	if cfg.Auth.Scheme != "apikey" {
		t.Errorf("Auth.Scheme = %q, want %q", cfg.Auth.Scheme, "apikey")
	}
	if len(cfg.Exclude) != 3 {
		t.Errorf("Exclude len = %d, want 3", len(cfg.Exclude))
	}
	if len(cfg.Overrides) != 1 {
		t.Fatalf("Overrides len = %d, want 1", len(cfg.Overrides))
	}
	if cfg.Overrides[0].Summary != "Register a new user" {
		t.Errorf("Override summary = %q", cfg.Overrides[0].Summary)
	}
	if len(cfg.ResponseHelpers) != 2 {
		t.Errorf("ResponseHelpers len = %d, want 2", len(cfg.ResponseHelpers))
	}
}

func TestLoadConfig_NoFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.LoadConfig(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config when file doesn't exist")
	}
}

func TestApplyExclusions(t *testing.T) {
	endpoints := []model.EndpointDef{
		{Method: "GET", Path: "/users"},
		{Method: "GET", Path: "/internal/health"},
		{Method: "GET", Path: "/debug/pprof"},
		{Method: "GET", Path: "/metrics"},
		{Method: "POST", Path: "/users"},
	}

	patterns := []string{
		"GET /internal/*",
		"GET /debug/*",
		"GET /metrics",
	}

	result := config.ApplyExclusions(endpoints, patterns)

	if len(result) != 2 {
		t.Fatalf("expected 2 endpoints after exclusion, got %d", len(result))
	}
	if result[0].Path != "/users" || result[0].Method != "GET" {
		t.Errorf("expected GET /users, got %s %s", result[0].Method, result[0].Path)
	}
	if result[1].Path != "/users" || result[1].Method != "POST" {
		t.Errorf("expected POST /users, got %s %s", result[1].Method, result[1].Path)
	}
}

func TestApplyOverrides(t *testing.T) {
	endpoints := []model.EndpointDef{
		{
			Method:  "POST",
			Path:    "/users",
			Summary: "Create User",
			Tags:    []string{"users"},
		},
		{
			Method:  "GET",
			Path:    "/users",
			Summary: "List Users",
		},
	}

	overrides := []config.Override{
		{
			Path:    "POST /users",
			Summary: "Register a new user",
			Tags:    []string{"auth", "users"},
			Responses: []config.ResponseOverride{
				{Status: 409, Description: "Email already in use"},
			},
		},
	}

	result := config.ApplyOverrides(endpoints, overrides)

	// POST /users should be overridden.
	post := result[0]
	if post.Summary != "Register a new user" {
		t.Errorf("Summary = %q, want %q", post.Summary, "Register a new user")
	}
	if len(post.Tags) != 2 || post.Tags[0] != "auth" {
		t.Errorf("Tags = %v, want [auth users]", post.Tags)
	}
	if len(post.Responses) != 1 {
		t.Fatalf("Responses len = %d, want 1", len(post.Responses))
	}
	if post.Responses[0].StatusCode != 409 {
		t.Errorf("Response status = %d, want 409", post.Responses[0].StatusCode)
	}
	if post.Responses[0].Source != "override" {
		t.Errorf("Response source = %q, want 'override'", post.Responses[0].Source)
	}

	// GET /users should be unchanged.
	get := result[1]
	if get.Summary != "List Users" {
		t.Errorf("GET /users summary should be unchanged, got %q", get.Summary)
	}
}
