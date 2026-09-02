package pipeline_test

import (
	"strings"
	"testing"
)

// unresolvedCategories are the analysis gaps that must reduce coverage. A
// report that counts an endpoint as resolved while any of these is true is
// worse than one that admits the gap: it is a number nobody can act on.
var unresolvedCategories = []string{
	"route group origin unknown",
	"empty route path",
	"openapi collision",
	"middleware",
	"request body",
}

// TestCorpus_UnresolvedShapesReduceCoverage runs the whole inverse fixture:
// every endpoint in it has something the analyzer cannot establish, so none of
// them may be reported as fully resolved, and each category has to appear.
func TestCorpus_UnresolvedShapesReduceCoverage(t *testing.T) {
	endpoints := runCorpusPipeline(t, "gin-unresolved")

	if len(endpoints) == 0 {
		t.Fatal("no endpoints extracted")
	}

	seen := make(map[string]bool)
	resolved := 0
	for _, ep := range endpoints {
		if len(ep.Unresolved) == 0 {
			resolved++
			t.Errorf("%s reported as fully resolved, but part of its contract cannot be established", routeKey(ep))
		}
		for _, u := range ep.Unresolved {
			for _, cat := range unresolvedCategories {
				if strings.HasPrefix(u, cat) {
					seen[cat] = true
				}
			}
		}
	}

	for _, cat := range unresolvedCategories {
		if !seen[cat] {
			t.Errorf("no endpoint reported a %q issue", cat)
		}
	}
	if resolved > 0 {
		t.Errorf("coverage reported %d/%d endpoints complete; the honest figure is 0",
			resolved, len(endpoints))
	}
}

// TestCorpus_OpenAPICollisionIsReported: OpenAPI treats two paths that differ
// only in the names of their template parameters as the same path, so one
// operation silently replaces the other in the generated document. Both sides
// of the pair have to say so.
func TestCorpus_OpenAPICollisionIsReported(t *testing.T) {
	endpoints := runCorpusPipeline(t, "gin-unresolved")

	colliding := 0
	for _, ep := range endpoints {
		if !hasPrefixIn(ep.Unresolved, "openapi collision") {
			continue
		}
		colliding++
		if !strings.Contains(strings.Join(ep.Unresolved, " "), "/api/items/") {
			t.Errorf("%s: collision note does not name the colliding path: %v", routeKey(ep), ep.Unresolved)
		}
	}
	if colliding != 2 {
		t.Errorf("endpoints reporting a collision = %d, want 2 (both sides of the pair)", colliding)
	}
}

// TestCorpus_GinRealWorldHasNoCollisions is the positive case, and the one the
// trailing-slash handling exists to protect: /api/users and /api/users/ are
// distinct paths, not a collision to be deduplicated away.
func TestCorpus_GinRealWorldHasNoCollisions(t *testing.T) {
	for _, ep := range runCorpusPipeline(t, "gin-realworld") {
		if hasPrefixIn(ep.Unresolved, "openapi collision") {
			t.Errorf("%s: unexpected collision: %v", routeKey(ep), ep.Unresolved)
		}
	}
}
