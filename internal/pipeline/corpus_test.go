package pipeline_test

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/openapi"
	"github.com/syst3mctl/godoclive/internal/pipeline"
)

// The RealWorld ("Conduit") gin backend is the corpus this analyzer is held to,
// reduced in testdata/gin-realworld to the shapes that matter for static
// analysis: cross-package registration helpers handed a group at the call
// site, a middleware chain accumulated through .Use(), and gin's own
// trailing-slash semantics, under which "" and "/" register two routes.
//
// Every number here is a count taken from that route table by hand, not a
// snapshot of what the analyzer happens to produce. checkCorpusGates holds
// both the fixture and — under the `corpus` build tag — the upstream
// repository itself to them.
const (
	corpusWantEndpoints    = 27
	corpusWantRequiredAuth = 16
	corpusWantOptionalAuth = 7
	corpusWantBodies       = 10
)

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

// corpusWantRequiredRoutes are the operations gated by AuthMiddleware(true).
// The rest of the table is either anonymous or behind AuthMiddleware(false),
// which reads a token when one is present but serves the request without it.
var corpusWantRequiredRoutes = []string{
	"DELETE /api/articles/{slug}",
	"DELETE /api/articles/{slug}/comments/{id}",
	"DELETE /api/articles/{slug}/favorite",
	"DELETE /api/profiles/{username}/follow",
	"GET /api/articles/feed",
	"GET /api/user",
	"GET /api/user/",
	"POST /api/articles",
	"POST /api/articles/",
	"POST /api/articles/{slug}/comments",
	"POST /api/articles/{slug}/favorite",
	"POST /api/profiles/{username}/follow",
	"PUT /api/articles/{slug}",
	"PUT /api/articles/{slug}/",
	"PUT /api/user",
	"PUT /api/user/",
}

// corpusWantOptionalRoutes are the operations behind AuthMiddleware(false).
var corpusWantOptionalRoutes = []string{
	"GET /api/articles",
	"GET /api/articles/",
	"GET /api/articles/{slug}",
	"GET /api/articles/{slug}/comments",
	"GET /api/profiles/{username}",
	"GET /api/tags",
	"GET /api/tags/",
}

// corpusWantBodyTypes maps each body-carrying operation to the validator type
// the binding chain has to arrive at. None of these is named at the bind
// itself: the shared binder takes interface{}, and the validator passes itself.
var corpusWantBodyTypes = map[string]string{
	"POST /api/users":                    "UserModelValidator",
	"POST /api/users/":                   "UserModelValidator",
	"POST /api/users/login":              "LoginValidator",
	"PUT /api/user":                      "UserModelValidator",
	"PUT /api/user/":                     "UserModelValidator",
	"POST /api/articles":                 "ArticleModelValidator",
	"POST /api/articles/":                "ArticleModelValidator",
	"PUT /api/articles/{slug}":           "ArticleModelValidator",
	"PUT /api/articles/{slug}/":          "ArticleModelValidator",
	"POST /api/articles/{slug}/comments": "CommentModelValidator",
}

// runCorpusPipeline analyzes one testdata module.
func runCorpusPipeline(t *testing.T, name string) []model.EndpointDef {
	t.Helper()
	endpoints, err := pipelineRun(testdataDir(name))
	if err != nil {
		t.Fatalf("RunPipeline(%s): %v", name, err)
	}
	return endpoints
}

// pipelineRun analyzes every package under dir. The upstream corpus run shares
// it, so both go through exactly the same entry point.
func pipelineRun(dir string) ([]model.EndpointDef, error) {
	return pipeline.RunPipeline(dir, "./...", nil)
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

// checkCorpusGates asserts every release gate against an analyzed endpoint set:
// the full route table, the operations that require authentication and the
// ones that merely accept it, the request bodies and the validator each binds
// into, no OpenAPI collisions, and — since every shape in this corpus is one
// the analyzer claims to handle — nothing left unresolved at all.
//
// The fixture and the upstream repository are held to the same function, so a
// gate can never pass on the reduction while failing on the real thing.
func checkCorpusGates(t *testing.T, endpoints []model.EndpointDef) {
	t.Helper()

	// The upstream route table has distinct slash/no-slash operations. Validate
	// the generated document too, so preserving routes cannot hide duplicate IDs.
	doc := openapi.Generate(endpoints, openapi.Config{})
	seenIDs := make(map[string]bool)
	for _, item := range doc.Paths {
		for _, op := range []*openapi.Operation{item.Get, item.Post, item.Put, item.Delete, item.Patch, item.Head, item.Options, item.Trace} {
			if op == nil {
				continue
			}
			if op.OperationID == "" || seenIDs[op.OperationID] {
				t.Errorf("duplicate or empty operation ID: %q", op.OperationID)
			}
			seenIDs[op.OperationID] = true
		}
	}

	if len(endpoints) != corpusWantEndpoints {
		t.Errorf("endpoint count = %d, want %d", len(endpoints), corpusWantEndpoints)
	}

	routes := make([]string, 0, len(endpoints))
	var required, optional, bodies []string
	for _, ep := range endpoints {
		key := routeKey(ep)
		routes = append(routes, key)
		switch {
		case ep.Auth.Required:
			required = append(required, key)
			if ep.Auth.Optional {
				t.Errorf("%s: reported as both required and optional", key)
			}
		case ep.Auth.Optional:
			optional = append(optional, key)
		}
		if ep.Request.Body != nil {
			bodies = append(bodies, key)
		}
	}

	assertSameSet(t, "routes", routes, corpusWantRoutes)
	assertSameSet(t, "required-auth operations", required, corpusWantRequiredRoutes)
	assertSameSet(t, "optional-auth operations", optional, corpusWantOptionalRoutes)

	if len(required) != corpusWantRequiredAuth {
		t.Errorf("required-auth operations = %d, want %d", len(required), corpusWantRequiredAuth)
	}
	if len(optional) != corpusWantOptionalAuth {
		t.Errorf("optional-auth operations = %d, want %d", len(optional), corpusWantOptionalAuth)
	}
	if len(bodies) != corpusWantBodies {
		t.Errorf("request bodies = %d, want %d\ngot: %v", len(bodies), corpusWantBodies, sorted(bodies))
	}

	for _, ep := range endpoints {
		key := routeKey(ep)
		want, expected := corpusWantBodyTypes[key]
		switch {
		case expected && ep.Request.Body == nil:
			t.Errorf("%s: no request body resolved, want %s", key, want)
		case expected && ep.Request.Body.Name != want:
			t.Errorf("%s: request body = %q, want %q", key, ep.Request.Body.Name, want)
		case !expected && ep.Request.Body != nil:
			t.Errorf("%s: unexpected request body %q", key, ep.Request.Body.Name)
		}
	}

	for _, ep := range endpoints {
		for _, u := range ep.Unresolved {
			t.Errorf("%s: unresolved: %s", routeKey(ep), u)
		}
	}
}

// TestCorpus_GinRealWorld is the per-PR gate: the compact RealWorld-derived
// fixture has to clear every release gate on every change.
func TestCorpus_GinRealWorld(t *testing.T) {
	checkCorpusGates(t, runCorpusPipeline(t, "gin-realworld"))
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
		if routeKey(ep) == "GET /api/items/{id}" && hasPrefixIn(ep.Unresolved, "route group origin") {
			t.Errorf("GET /api/items/{id} has a traceable group but was flagged: %v", ep.Unresolved)
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

// TestCorpus_UnreadableMiddlewareIsFlagged: a middleware held as data cannot be
// resolved to a body, and an unread middleware may be the one enforcing auth —
// so it is reported rather than passed over as "no auth".
func TestCorpus_UnreadableMiddlewareIsFlagged(t *testing.T) {
	endpoints := runCorpusPipeline(t, "gin-unresolved")

	for _, ep := range endpoints {
		if routeKey(ep) != "GET /api/admin/stats" {
			continue
		}
		if !hasPrefixIn(ep.Unresolved, "middleware") {
			t.Errorf("expected an unresolved-middleware caveat, got %v", ep.Unresolved)
		}
		return
	}
	t.Fatal("route GET /api/admin/stats not found")
}

// TestCorpus_UnresolvableBindIsFlagged: a wrapper binding into an interface
// value documents nothing, and the analyzer says so rather than reporting the
// endpoint as having no body at all.
func TestCorpus_UnresolvableBindIsFlagged(t *testing.T) {
	endpoints := runCorpusPipeline(t, "gin-unresolved")

	for _, ep := range endpoints {
		if routeKey(ep) != "POST /{id}" {
			continue
		}
		if ep.Request.Body != nil {
			t.Errorf("expected no body schema, got %q", ep.Request.Body.Name)
		}
		if !hasPrefixIn(ep.Unresolved, "request body") {
			t.Errorf("expected a request-body caveat, got %v", ep.Unresolved)
		}
		return
	}
	t.Fatal("route POST /{id} not found")
}

// TestCorpus_GinRealWorldResponseSchemas pins the response schemas that gin.H
// literals have to resolve to. Every RealWorld response is an envelope —
// gin.H{"user": serializer.Response()} — and reporting that as a bare object
// says nothing at all about what the endpoint returns.
func TestCorpus_GinRealWorldResponseSchemas(t *testing.T) {
	endpoints := runCorpusPipeline(t, "gin-realworld")

	cases := []struct {
		route     string
		status    int
		field     string
		fieldType string
		slice     bool
	}{
		{"POST /api/users", 201, "user", "UserResponse", false},
		{"GET /api/user", 200, "user", "UserResponse", false},
		{"GET /api/profiles/{username}", 200, "profile", "ProfileResponse", false},
		{"POST /api/articles", 201, "article", "ArticleResponse", false},
		{"GET /api/articles", 200, "articles", "ArticleResponse", true},
		{"GET /api/articles/{slug}/comments", 200, "comments", "CommentResponse", true},
		{"POST /api/articles/{slug}/comments", 201, "comment", "CommentResponse", false},
	}

	for _, tc := range cases {
		t.Run(tc.route, func(t *testing.T) {
			ep := corpusEndpoint(t, endpoints, tc.route)
			body := responseBody(t, ep, tc.status)
			if body.Kind != model.KindStruct {
				t.Fatalf("response %d body kind = %q, want struct", tc.status, body.Kind)
			}
			field, ok := corpusField(body, tc.field)
			if !ok {
				t.Fatalf("response %d has no field %q (fields: %v)", tc.status, tc.field, fieldNames(body))
			}
			ft := field.Type
			if tc.slice {
				if ft.Kind != model.KindSlice || ft.Elem == nil {
					t.Fatalf("field %q kind = %q, want slice", tc.field, ft.Kind)
				}
				ft = *ft.Elem
			}
			if ft.Name != tc.fieldType {
				t.Errorf("field %q type = %q, want %q", tc.field, ft.Name, tc.fieldType)
			}
			if len(ft.Fields) == 0 {
				t.Errorf("field %q resolved to %q with no fields — the schema was not expanded", tc.field, ft.Name)
			}
		})
	}
}

// TestCorpus_GinRealWorldScalarEnvelope covers the other envelope shape: a
// literal value rather than a serializer call.
func TestCorpus_GinRealWorldScalarEnvelope(t *testing.T) {
	endpoints := runCorpusPipeline(t, "gin-realworld")

	body := responseBody(t, corpusEndpoint(t, endpoints, "GET /api/ping/"), 200)
	field, ok := corpusField(body, "message")
	if !ok {
		t.Fatalf("no field %q (fields: %v)", "message", fieldNames(body))
	}
	if field.Type.Kind != model.KindPrimitive || field.Type.Name != "string" {
		t.Errorf("field message = %s/%s, want primitive/string", field.Type.Kind, field.Type.Name)
	}
}

func corpusEndpoint(t *testing.T, endpoints []model.EndpointDef, route string) model.EndpointDef {
	t.Helper()
	for _, ep := range endpoints {
		if routeKey(ep) == route {
			return ep
		}
	}
	t.Fatalf("route %s not found", route)
	return model.EndpointDef{}
}

func responseBody(t *testing.T, ep model.EndpointDef, status int) *model.TypeDef {
	t.Helper()
	for _, r := range ep.Responses {
		if r.StatusCode == status {
			if r.Body == nil {
				t.Fatalf("%s: response %d has no body", routeKey(ep), status)
			}
			return r.Body
		}
	}
	t.Fatalf("%s: no response with status %d", routeKey(ep), status)
	return nil
}

func corpusField(td *model.TypeDef, name string) (model.FieldDef, bool) {
	for _, f := range td.Fields {
		if f.JSONName == name {
			return f, true
		}
	}
	return model.FieldDef{}, false
}

func fieldNames(td *model.TypeDef) []string {
	names := make([]string, 0, len(td.Fields))
	for _, f := range td.Fields {
		names = append(names, f.JSONName)
	}
	return names
}

// corpusWantAbortStatuses pins the COMPLETE response status set of the two
// handlers that reject a request outright, together with where each rejection
// is written. The sets are read off the handler source, never captured from
// analyzer output — a gate built from what the analyzer currently says can only
// ever agree with itself:
//
//	ArticleFeed   — c.AbortWithError(401) inline, then 200 on the success path.
//	ArticleDelete — c.JSON(404) for an unknown slug, abortForbidden(c) one call
//	                away for a non-author, then 200.
//
// The second is the case that matters here: an abort reached through a helper
// is found by a different switch than an inline one, and only one of the two
// was fixed initially.
var corpusWantAbortStatuses = map[string]struct {
	statuses    []int
	abortStatus int
	abortSource string
}{
	"GET /api/articles/feed":      {statuses: []int{200, 401}, abortStatus: 401, abortSource: "explicit"},
	"DELETE /api/articles/{slug}": {statuses: []int{200, 403, 404}, abortStatus: 403, abortSource: "helper"},
}

// TestCorpus_GinRealWorldAbortResponses: an abort is a response. A handler that
// rejects a request returns that status at runtime, and documenting only the
// success and not-found paths tells a client the wrong thing about what it has
// to handle — whether the abort is written inline or in a shared helper.
func TestCorpus_GinRealWorldAbortResponses(t *testing.T) {
	endpoints := runCorpusPipeline(t, "gin-realworld")

	for route, want := range corpusWantAbortStatuses {
		t.Run(route, func(t *testing.T) {
			ep := corpusEndpoint(t, endpoints, route)

			got := make([]int, 0, len(ep.Responses))
			var abort *model.ResponseDef
			for i, r := range ep.Responses {
				got = append(got, r.StatusCode)
				if r.StatusCode == want.abortStatus {
					abort = &ep.Responses[i]
				}
			}
			sort.Ints(got)

			if !reflect.DeepEqual(got, want.statuses) {
				t.Errorf("response statuses = %v, want %v", got, want.statuses)
			}
			if abort == nil {
				t.Fatalf("no %d documented; got %v", want.abortStatus, got)
			}
			if abort.Body != nil {
				t.Errorf("%d carries a body %+v; an abort writes a status and nothing else",
					want.abortStatus, abort.Body)
			}
			if abort.ContentType != "" {
				t.Errorf("%d content type = %q, want empty for a body-less abort",
					want.abortStatus, abort.ContentType)
			}
			if abort.Source != want.abortSource {
				t.Errorf("%d source = %q, want %q — the abort was found by the wrong path",
					want.abortStatus, abort.Source, want.abortSource)
			}
			if abort.Description != descriptionFor(want.abortStatus) {
				t.Errorf("%d description = %q, want %q",
					want.abortStatus, abort.Description, descriptionFor(want.abortStatus))
			}
		})
	}
}

// descriptionFor is the expected human description for the statuses this test
// pins, spelled out rather than borrowed from the analyzer's own table.
func descriptionFor(status int) string {
	switch status {
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	}
	return ""
}
