package pipeline_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/pipeline"
)

// The RealWorld ("Conduit") gin backend is the corpus this analyzer is held to,
// reduced in testdata/gin-realworld to the shapes that matter for static
// analysis: cross-package registration helpers handed a group at the call
// site, a middleware chain accumulated through .Use(), and gin's own
// trailing-slash semantics, under which "" and "/" register two routes.
//
// Every number here is a count taken from that route table by hand, not a
// snapshot of what the analyzer happens to produce.
const corpusWantEndpoints = 27

// corpusWantRoutes is the full route table, method and path.
var corpusWantRoutes = []string{
	"DELETE /api/articles/{slug}",
	"DELETE /api/articles/{slug}/comments/{id}",
	"DELETE /api/articles/{slug}/favorite",
	"DELETE /api/profiles/{username}/follow",
	"GET /api/articles",
	"GET /api/articles/",
	"GET /api/articles/feed",
	"GET /api/articles/{slug}",
	"GET /api/articles/{slug}/comments",
	"GET /api/ping/",
	"GET /api/profiles/{username}",
	"GET /api/tags",
	"GET /api/tags/",
	"GET /api/user",
	"GET /api/user/",
	"POST /api/articles",
	"POST /api/articles/",
	"POST /api/articles/{slug}/comments",
	"POST /api/articles/{slug}/favorite",
	"POST /api/profiles/{username}/follow",
	"POST /api/users",
	"POST /api/users/",
	"POST /api/users/login",
	"PUT /api/articles/{slug}",
	"PUT /api/articles/{slug}/",
	"PUT /api/user",
	"PUT /api/user/",
}

// runCorpusPipeline analyzes one testdata module.
func runCorpusPipeline(t *testing.T, name string) []model.EndpointDef {
	t.Helper()
	endpoints, err := pipeline.RunPipeline(testdataDir(name), "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline(%s): %v", name, err)
	}
	return endpoints
}

func routeKey(ep model.EndpointDef) string {
	return ep.Method + " " + ep.Path
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

// assertSameSet compares two route sets and reports what is missing or extra.
func assertSameSet(t *testing.T, label string, got, want []string) {
	t.Helper()
	gotSet := make(map[string]int, len(got))
	for _, g := range got {
		gotSet[g]++
	}
	wantSet := make(map[string]int, len(want))
	for _, w := range want {
		wantSet[w]++
	}

	var missing, extra []string
	for w, n := range wantSet {
		if gotSet[w] < n {
			missing = append(missing, fmt.Sprintf("%s (want %d, got %d)", w, n, gotSet[w]))
		}
	}
	for g, n := range gotSet {
		if wantSet[g] < n {
			extra = append(extra, fmt.Sprintf("%s (got %d, want %d)", g, n, wantSet[g]))
		}
	}
	if len(missing) > 0 {
		t.Errorf("%s missing:\n  %s", label, strings.Join(sorted(missing), "\n  "))
	}
	if len(extra) > 0 {
		t.Errorf("%s unexpected:\n  %s", label, strings.Join(sorted(extra), "\n  "))
	}
}

// TestCorpus_GinRealWorldRoutes is the route-table gate: every path has to come
// out with the prefix of the group its registration helper was handed, and the
// trailing-slash pairs have to stay distinct.
func TestCorpus_GinRealWorldRoutes(t *testing.T) {
	endpoints := runCorpusPipeline(t, "gin-realworld")

	if len(endpoints) != corpusWantEndpoints {
		t.Errorf("endpoint count = %d, want %d", len(endpoints), corpusWantEndpoints)
	}

	got := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		got = append(got, routeKey(ep))
	}
	assertSameSet(t, "routes", got, corpusWantRoutes)

	for _, ep := range endpoints {
		for _, u := range ep.Unresolved {
			t.Errorf("%s: unresolved: %s", routeKey(ep), u)
		}
	}
}

// TestCorpus_UntracedHelperIsFlagged pins the other half of the contract: when
// a registration helper's call site cannot be traced, its routes are still
// extracted, but they say so rather than being published with a confidently
// wrong path.
func TestCorpus_UntracedHelperIsFlagged(t *testing.T) {
	endpoints := runCorpusPipeline(t, "gin-unresolved")

	flagged := map[string]bool{}
	for _, ep := range endpoints {
		if hasPrefixIn(ep.Unresolved, "route group origin unknown") {
			flagged[routeKey(ep)] = true
		}
	}

	// Both routes OrphanRegister mounts are flagged; the one registered on a
	// group main() owns keeps its resolved path and stays clean.
	for _, want := range []string{"GET ", "POST /{id}"} {
		if !flagged[want] {
			t.Errorf("route %q was not flagged as having an unknown group origin", want)
		}
	}
	for _, ep := range endpoints {
		if routeKey(ep) == "GET /api/items/{id}" && len(ep.Unresolved) > 0 {
			t.Errorf("GET /api/items/{id} is fully traceable but reported: %v", ep.Unresolved)
		}
	}

	// A route whose path could not be resolved is called out separately from
	// its group: an empty path is its own defect.
	if !hasPrefixIn(endpointFor(endpoints, "GET ").Unresolved, "empty route path") {
		t.Error("expected the empty-path registration to be reported as such")
	}
}

// endpointFor returns the endpoint with the given "METHOD path" key.
func endpointFor(endpoints []model.EndpointDef, key string) model.EndpointDef {
	for _, ep := range endpoints {
		if routeKey(ep) == key {
			return ep
		}
	}
	return model.EndpointDef{}
}

func hasPrefixIn(notes []string, prefix string) bool {
	for _, n := range notes {
		if strings.HasPrefix(n, prefix) {
			return true
		}
	}
	return false
}
