package pipeline_test

import (
	"testing"

	"github.com/syst3mctl/godoclive/internal/pipeline"
)

// The prose a Go developer already wrote above the handler is a better
// description than anything derived from splitting the function name.

func TestPipeline_HandlerDocBecomesSummaryAndDescription(t *testing.T) {
	eps, err := pipeline.RunPipeline(testdataDir("chi-basic"), "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/users")
	if ep == nil {
		t.Fatal("GET /users not found")
	}

	// The doc comment reads:
	//   ListUsers returns a paginated list of users.
	//   Query params: page (required), limit (optional, default 20).
	const wantSummary = "Returns a paginated list of users."
	if ep.Summary != wantSummary {
		t.Errorf("Summary = %q, want %q", ep.Summary, wantSummary)
	}
	// The description carries only what the summary does not.
	const wantDesc = "Query params: page (required), limit (optional, default 20)."
	if ep.Description != wantDesc {
		t.Errorf("Description = %q, want %q", ep.Description, wantDesc)
	}
}

func TestPipeline_HandlerWithoutDocFallsBackToItsName(t *testing.T) {
	eps, err := pipeline.RunPipeline(testdataDir("chi-inline"), "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	var checked int
	for _, ep := range eps {
		if ep.Description != "" {
			continue // has a doc comment; not the case under test
		}
		if ep.Summary == "" {
			t.Errorf("%s %s: no doc comment and no inferred summary", ep.Method, ep.Path)
		}
		checked++
	}
	if checked == 0 {
		t.Skip("no undocumented handlers in this fixture")
	}
}

// A one-sentence comment says everything in the summary. Repeating it as the
// description shows the reader the same line twice.
func TestPipeline_SingleSentenceDocHasNoDescription(t *testing.T) {
	eps, err := pipeline.RunPipeline(testdataDir("mixed-routers"), "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/healthz")
	if ep == nil {
		t.Fatal("GET /healthz not found")
	}
	if ep.Summary != "Reports process liveness." {
		t.Errorf("Summary = %q, want %q", ep.Summary, "Reports process liveness.")
	}
	if ep.Description != "" {
		t.Errorf("Description = %q, want empty", ep.Description)
	}
}
