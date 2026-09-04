package pipeline_test

import (
	"testing"

	"github.com/syst3mctl/godoclive/internal/pipeline"
)

// A team that wraps its router behind a house type — holding the real router in
// a field and exposing its own Handle, GET and POST — used to get nothing at
// all. The registration call names no router type and no literal path, so every
// route in the service was invisible. These fixtures pin the fix for each
// supported framework.

func TestPipeline_HouseRouterWrapper(t *testing.T) {
	cases := []struct {
		fixture string
		routes  [][2]string // method, path
	}{
		{"chi-wrapped", [][2]string{
			{"GET", "/widgets"},
			{"GET", "/widgets/{id}"},
			{"POST", "/widgets"},
		}},
		{"gin-wrapped", [][2]string{
			{"GET", "/widgets"},
			{"POST", "/widgets"},
		}},
		{"echo-wrapped", [][2]string{
			{"GET", "/widgets"},
			{"POST", "/widgets"},
		}},
		{"fiber-wrapped", [][2]string{
			{"GET", "/widgets"},
			{"POST", "/widgets"},
		}},
		{"gorilla-wrapped", [][2]string{
			{"GET", "/widgets"},
			{"POST", "/widgets"},
		}},
		{"wrapped-router", [][2]string{
			{"GET", "/users"},
			{"GET", "/users/{id}"},
			{"POST", "/users"},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			eps, err := pipeline.RunPipeline(testdataDir(tc.fixture), "./...", nil)
			if err != nil {
				t.Fatalf("RunPipeline: %v", err)
			}
			if len(eps) != len(tc.routes) {
				t.Fatalf("got %d endpoints, want %d: %v", len(eps), len(tc.routes), summarize(eps))
			}
			for _, want := range tc.routes {
				ep := findEndpoint(eps, want[0], want[1])
				if ep == nil {
					t.Errorf("missing %s %s (found: %v)", want[0], want[1], summarize(eps))
					continue
				}
				// A route whose path came from a wrapper parameter but whose
				// handler did not resolve is worse than none: it documents an
				// endpoint with no contract.
				if len(ep.Unresolved) > 0 {
					t.Errorf("%s %s unresolved: %v", want[0], want[1], ep.Unresolved)
				}
				if ep.HandlerName == "" || ep.HandlerName == "anonymous" {
					t.Errorf("%s %s: handler did not resolve (%q)", want[0], want[1], ep.HandlerName)
				}
			}
		})
	}
}

// The wrapper, the handlers and the wiring each live in their own package. The
// handler expression type-checks only in the package that named it, so the
// route has to carry that package's type information out of the wrapper's body
// for anything about the endpoint to resolve.
func TestPipeline_HouseRouterWrapper_AcrossPackages(t *testing.T) {
	eps, err := pipeline.RunPipeline(testdataDir("wrapped-router"), "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	get := findEndpoint(eps, "GET", "/users/{id}")
	if get == nil {
		t.Fatal("GET /users/{id} not found")
	}
	const wantPkg = "github.com/syst3mctl/godoclive/testdata/wrapped-router/handlers"
	if get.Package != wantPkg {
		t.Errorf("Package = %q, want %q", get.Package, wantPkg)
	}
	if len(get.Request.PathParams) != 1 || get.Request.PathParams[0].Name != "id" {
		t.Errorf("PathParams = %+v, want one param named id", get.Request.PathParams)
	}

	// The body type is declared in the handlers package and reached only
	// through the handler expression the call site supplied.
	post := findEndpoint(eps, "POST", "/users")
	if post == nil {
		t.Fatal("POST /users not found")
	}
	if post.Request.Body == nil {
		t.Fatal("POST /users has no request body")
	}
	if post.Request.Body.Name != "User" {
		t.Errorf("body type = %q, want User", post.Request.Body.Name)
	}
	var sawEmail bool
	for _, f := range post.Request.Body.Fields {
		if f.JSONName != "email" {
			continue
		}
		sawEmail = true
		if f.Constraints == nil || f.Constraints.Format != "email" {
			t.Errorf("email constraints = %+v, want format email", f.Constraints)
		}
	}
	if !sawEmail {
		t.Error("body has no email field")
	}
}

// A wrapper nothing calls registers nothing, and must not be reported as a
// route whose path is a parameter's name.
func TestPipeline_UncalledWrapperEmitsNothing(t *testing.T) {
	eps, err := pipeline.RunPipeline(testdataDir("chi-wrapped"), "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	for _, ep := range eps {
		if ep.Path == "" || ep.Path == "path" || ep.Path == "/path" {
			t.Errorf("wrapper body emitted as a route: %s %q", ep.Method, ep.Path)
		}
	}
}
