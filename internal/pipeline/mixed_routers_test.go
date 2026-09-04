package pipeline_test

import (
	"testing"

	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/pipeline"
)

// A service is free to register routes on more than one framework. Detection
// used to pick a single winner by priority, so the routes belonging to every
// other framework in the project disappeared without a word. These tests pin
// the union.

func TestPipeline_MixedRouters_FindsEveryFramework(t *testing.T) {
	dir := testdataDir("mixed-routers")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	want := []struct {
		method string
		path   string
		router string
	}{
		{"GET", "/api/v1/users", "chi"},
		{"GET", "/api/v1/users/{id}", "chi"},
		{"GET", "/api/v2/products", "gin"},
		{"POST", "/api/v2/products", "gin"},
		{"GET", "/healthz", "stdlib"},
	}

	if len(eps) != len(want) {
		t.Fatalf("expected %d endpoints, got %d: %v", len(want), len(eps), summarize(eps))
	}

	for _, w := range want {
		ep := findEndpoint(eps, w.method, w.path)
		if ep == nil {
			t.Errorf("missing endpoint %s %s (found: %v)", w.method, w.path, summarize(eps))
			continue
		}
		if ep.Router != w.router {
			t.Errorf("%s %s: Router = %q, want %q", w.method, w.path, ep.Router, w.router)
		}
	}
}

func TestPipeline_MixedRouters_ContractsResolveAcrossFrameworks(t *testing.T) {
	dir := testdataDir("mixed-routers")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// The chi route's path param must still resolve when gin and stdlib
	// extractors have also run over the same packages.
	getUser := findEndpoint(eps, "GET", "/api/v1/users/{id}")
	if getUser == nil {
		t.Fatal("GET /api/v1/users/{id} not found")
	}
	if len(getUser.Request.PathParams) != 1 || getUser.Request.PathParams[0].Name != "id" {
		t.Errorf("chi path params = %+v, want one param named id", getUser.Request.PathParams)
	}

	// The gin route's bound body must resolve.
	create := findEndpoint(eps, "POST", "/api/v2/products")
	if create == nil {
		t.Fatal("POST /api/v2/products not found")
	}
	if create.Request.Body == nil {
		t.Fatal("POST /api/v2/products has no request body")
	}
	if create.Request.Body.Name != "Product" {
		t.Errorf("gin body type = %q, want Product", create.Request.Body.Name)
	}
}

// summarize renders endpoints compactly for failure messages.
func summarize(eps []model.EndpointDef) []string {
	out := make([]string, 0, len(eps))
	for _, ep := range eps {
		out = append(out, ep.Method+" "+ep.Path+" ["+ep.Router+"]")
	}
	return out
}
