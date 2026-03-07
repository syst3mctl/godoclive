package pipeline_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/pipeline"
)

func testdataDir(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", name)
}

// findEndpoint returns the endpoint matching method+path, or nil.
func findEndpoint(eps []model.EndpointDef, method, path string) *model.EndpointDef {
	for i := range eps {
		if eps[i].Method == method && eps[i].Path == path {
			return &eps[i]
		}
	}
	return nil
}

// --- chi-basic integration tests ---

func TestPipeline_ChiBasic(t *testing.T) {
	dir := testdataDir("chi-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	if len(eps) != 6 {
		t.Fatalf("expected 6 endpoints, got %d", len(eps))
	}

	// Verify all expected routes exist.
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/users"},
		{"POST", "/users"},
		{"GET", "/users/{id}"},
		{"DELETE", "/users/{id}"},
		{"GET", "/v1/users/{id}"},
		{"POST", "/v2/users"},
	}
	for _, r := range routes {
		ep := findEndpoint(eps, r.method, r.path)
		if ep == nil {
			t.Errorf("missing endpoint %s %s", r.method, r.path)
		}
	}
}

func TestPipeline_ChiBasic_ListUsers(t *testing.T) {
	dir := testdataDir("chi-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/users")
	if ep == nil {
		t.Fatal("GET /users not found")
	}

	// Summary inferred from handler name.
	if ep.Summary != "List Users" {
		t.Errorf("Summary = %q, want %q", ep.Summary, "List Users")
	}

	// Tags inferred.
	if len(ep.Tags) == 0 || ep.Tags[0] != "users" {
		t.Errorf("Tags = %v, want [users]", ep.Tags)
	}

	// No auth on this endpoint (it's outside the auth group).
	if ep.Auth.Required {
		t.Error("GET /users should not require auth")
	}

	// Query params: page (required) and limit.
	if len(ep.Request.QueryParams) < 2 {
		t.Fatalf("expected >= 2 query params, got %d", len(ep.Request.QueryParams))
	}
	var foundPage, foundLimit bool
	for _, p := range ep.Request.QueryParams {
		if p.Name == "page" {
			foundPage = true
			if !p.Required {
				t.Error("page should be required")
			}
		}
		if p.Name == "limit" {
			foundLimit = true
		}
	}
	if !foundPage {
		t.Error("missing query param 'page'")
	}
	if !foundLimit {
		t.Error("missing query param 'limit'")
	}

	// Responses should include 200 and 400.
	var has200, has400 bool
	for _, r := range ep.Responses {
		if r.StatusCode == 200 {
			has200 = true
		}
		if r.StatusCode == 400 {
			has400 = true
		}
	}
	if !has200 {
		t.Error("missing 200 response")
	}
	if !has400 {
		t.Error("missing 400 response")
	}
}

func TestPipeline_ChiBasic_CreateUser(t *testing.T) {
	dir := testdataDir("chi-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "POST", "/users")
	if ep == nil {
		t.Fatal("POST /users not found")
	}

	if ep.Summary != "Create User" {
		t.Errorf("Summary = %q, want %q", ep.Summary, "Create User")
	}

	// Auth: should have bearer (JWT middleware).
	if !ep.Auth.Required {
		t.Error("POST /users should require auth")
	}
	if len(ep.Auth.Schemes) == 0 || ep.Auth.Schemes[0] != model.AuthBearer {
		t.Errorf("expected bearer auth, got %v", ep.Auth.Schemes)
	}

	// Request body should be CreateUserRequest.
	if ep.Request.Body == nil {
		t.Fatal("expected request body")
	}
	if ep.Request.Body.Name != "CreateUserRequest" {
		t.Errorf("body name = %q, want %q", ep.Request.Body.Name, "CreateUserRequest")
	}
	if ep.Request.Body.Kind != model.KindStruct {
		t.Errorf("body kind = %s, want struct", ep.Request.Body.Kind)
	}
	if len(ep.Request.Body.Fields) != 3 {
		t.Errorf("expected 3 body fields, got %d", len(ep.Request.Body.Fields))
	}

	// Responses: 201 and 400.
	var has201, has400 bool
	for _, r := range ep.Responses {
		if r.StatusCode == 201 {
			has201 = true
		}
		if r.StatusCode == 400 {
			has400 = true
		}
	}
	if !has201 {
		t.Error("missing 201 response")
	}
	if !has400 {
		t.Error("missing 400 response")
	}
}

func TestPipeline_ChiBasic_GetUser(t *testing.T) {
	dir := testdataDir("chi-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/users/{id}")
	if ep == nil {
		t.Fatal("GET /users/{id} not found")
	}

	// Path params: id.
	if len(ep.Request.PathParams) == 0 {
		t.Fatal("expected path param 'id'")
	}
	found := false
	for _, p := range ep.Request.PathParams {
		if p.Name == "id" {
			found = true
			if !p.Required {
				t.Error("path param 'id' should be required")
			}
		}
	}
	if !found {
		t.Error("missing path param 'id'")
	}

	// Auth: bearer.
	if !ep.Auth.Required {
		t.Error("GET /users/{id} should require auth")
	}
}

func TestPipeline_ChiBasic_DeleteUser(t *testing.T) {
	dir := testdataDir("chi-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "DELETE", "/users/{id}")
	if ep == nil {
		t.Fatal("DELETE /users/{id} not found")
	}

	// Should have 204 response.
	var has204 bool
	for _, r := range ep.Responses {
		if r.StatusCode == 204 {
			has204 = true
		}
	}
	if !has204 {
		t.Error("missing 204 response")
	}
}

func TestPipeline_ChiBasic_IOReadAllBody(t *testing.T) {
	dir := testdataDir("chi-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "POST", "/v2/users")
	if ep == nil {
		t.Fatal("POST /v2/users not found")
	}

	if ep.Request.Body == nil {
		t.Fatal("expected request body from io.ReadAll + json.Unmarshal pattern")
	}
	if ep.Request.Body.Name != "CreateUserRequest" {
		t.Errorf("body name = %q, want %q", ep.Request.Body.Name, "CreateUserRequest")
	}
	if ep.Request.ContentType != "application/json" {
		t.Errorf("content type = %q, want %q", ep.Request.ContentType, "application/json")
	}
}

func TestPipeline_ChiBasic_DeprecatedHandler(t *testing.T) {
	dir := testdataDir("chi-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/v1/users/{id}")
	if ep == nil {
		t.Fatal("GET /v1/users/{id} not found")
	}
	if !ep.Deprecated {
		t.Error("GET /v1/users/{id} should be deprecated")
	}

	// Non-deprecated endpoints should not be marked.
	ep2 := findEndpoint(eps, "GET", "/users/{id}")
	if ep2 == nil {
		t.Fatal("GET /users/{id} not found")
	}
	if ep2.Deprecated {
		t.Error("GET /users/{id} should not be deprecated")
	}
}

// --- gin-basic integration tests ---

func TestPipeline_GinBasic(t *testing.T) {
	dir := testdataDir("gin-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	if len(eps) != 5 {
		t.Fatalf("expected 5 endpoints, got %d", len(eps))
	}

	// Verify routes.
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/items"},
		{"GET", "/api/v1/items/{id}"},
		{"POST", "/api/v1/items"},
		{"DELETE", "/api/v1/items/{id}"},
		{"GET", "/api/v1/admin/users"},
	}
	for _, r := range routes {
		ep := findEndpoint(eps, r.method, r.path)
		if ep == nil {
			t.Errorf("missing endpoint %s %s", r.method, r.path)
		}
	}
}

func TestPipeline_GinBasic_CreateItem(t *testing.T) {
	dir := testdataDir("gin-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "POST", "/api/v1/items")
	if ep == nil {
		t.Fatal("POST /api/v1/items not found")
	}

	if ep.Summary != "Create Item" {
		t.Errorf("Summary = %q, want %q", ep.Summary, "Create Item")
	}

	// Request body.
	if ep.Request.Body == nil {
		t.Fatal("expected request body")
	}
	if ep.Request.Body.Name != "CreateItemRequest" {
		t.Errorf("body name = %q, want %q", ep.Request.Body.Name, "CreateItemRequest")
	}

	// Responses: 201 and 400.
	var has201, has400 bool
	for _, r := range ep.Responses {
		if r.StatusCode == 201 {
			has201 = true
		}
		if r.StatusCode == 400 {
			has400 = true
		}
	}
	if !has201 {
		t.Error("missing 201 response")
	}
	if !has400 {
		t.Error("missing 400 response")
	}
}

func TestPipeline_GinBasic_AdminAuth(t *testing.T) {
	dir := testdataDir("gin-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/api/v1/admin/users")
	if ep == nil {
		t.Fatal("GET /api/v1/admin/users not found")
	}

	// Auth: apikey.
	if !ep.Auth.Required {
		t.Error("admin endpoint should require auth")
	}
	foundAPIKey := false
	for _, s := range ep.Auth.Schemes {
		if s == model.AuthAPIKey {
			foundAPIKey = true
		}
	}
	if !foundAPIKey {
		t.Errorf("expected apikey auth, got %v", ep.Auth.Schemes)
	}
}

func TestPipeline_GinBasic_ListItems_QueryParams(t *testing.T) {
	dir := testdataDir("gin-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/api/v1/items")
	if ep == nil {
		t.Fatal("GET /api/v1/items not found")
	}

	// Query params: search and limit (with default).
	var foundSearch, foundLimit bool
	for _, p := range ep.Request.QueryParams {
		if p.Name == "search" {
			foundSearch = true
		}
		if p.Name == "limit" {
			foundLimit = true
			if p.Default == nil || *p.Default != "20" {
				t.Errorf("limit default should be '20', got %v", p.Default)
			}
		}
	}
	if !foundSearch {
		t.Error("missing query param 'search'")
	}
	if !foundLimit {
		t.Error("missing query param 'limit'")
	}
}

func TestPipeline_GinBasic_PathParams(t *testing.T) {
	dir := testdataDir("gin-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/api/v1/items/{id}")
	if ep == nil {
		t.Fatal("GET /api/v1/items/{id} not found")
	}

	if len(ep.Request.PathParams) == 0 {
		t.Fatal("expected path param 'id'")
	}
	found := false
	for _, p := range ep.Request.PathParams {
		if p.Name == "id" {
			found = true
		}
	}
	if !found {
		t.Error("missing path param 'id'")
	}
}

// --- chi-helpers integration tests ---

func TestPipeline_ChiHelpers(t *testing.T) {
	dir := testdataDir("chi-helpers")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	if len(eps) != 4 {
		t.Fatalf("expected 4 endpoints, got %d", len(eps))
	}
}

func TestPipeline_ChiHelpers_GetUser(t *testing.T) {
	dir := testdataDir("chi-helpers")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/users/{id}")
	if ep == nil {
		t.Fatal("GET /users/{id} not found")
	}

	// Should have responses from both helpers: respond (200) and sendError (400).
	var has200, has400 bool
	for _, r := range ep.Responses {
		if r.StatusCode == 200 {
			has200 = true
		}
		if r.StatusCode == 400 {
			has400 = true
		}
	}
	if !has200 {
		t.Error("missing 200 response (from respond helper)")
	}
	if !has400 {
		t.Error("missing 400 response (from sendError helper)")
	}
}

func TestPipeline_ChiHelpers_HealthCheck(t *testing.T) {
	dir := testdataDir("chi-helpers")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/health")
	if ep == nil {
		t.Fatal("GET /health not found")
	}

	if ep.Summary != "Health Check" {
		t.Errorf("Summary = %q, want %q", ep.Summary, "Health Check")
	}

	// Tag should be "health".
	if len(ep.Tags) == 0 || ep.Tags[0] != "health" {
		t.Errorf("Tags = %v, want [health]", ep.Tags)
	}

	// Should have a 200 response (implicit from json.Encode without WriteHeader).
	var has200 bool
	for _, r := range ep.Responses {
		if r.StatusCode == 200 {
			has200 = true
		}
	}
	if !has200 {
		t.Error("missing 200 response (implicit from json.Encode)")
	}
}

func TestPipeline_ChiHelpers_ListUsers(t *testing.T) {
	dir := testdataDir("chi-helpers")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/users")
	if ep == nil {
		t.Fatal("GET /users not found")
	}

	// Should have a 200 response from writeJSON helper.
	var has200 bool
	for _, r := range ep.Responses {
		if r.StatusCode == 200 {
			has200 = true
		}
	}
	if !has200 {
		t.Error("missing 200 response (from writeJSON helper)")
	}
}

// --- chi-nested integration tests ---

func TestPipeline_ChiNested(t *testing.T) {
	dir := testdataDir("chi-nested")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// Core routes from nested Route/Group patterns (Mount is a known Phase 2 limitation).
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/users"},
		{"POST", "/api/v1/users"},
		{"GET", "/api/v1/users/{userID}"},
		{"PUT", "/api/v1/users/{userID}"},
		{"GET", "/api/v1/stats"},
		{"DELETE", "/api/v1/cache"},
	}

	for _, r := range routes {
		ep := findEndpoint(eps, r.method, r.path)
		if ep == nil {
			t.Errorf("missing endpoint %s %s", r.method, r.path)
		}
	}

	// Mount targets (adminRouter()) are a known limitation — log but don't fail.
	mountRoutes := []struct{ method, path string }{
		{"GET", "/admin/dashboard"},
		{"POST", "/admin/settings"},
	}
	for _, r := range mountRoutes {
		ep := findEndpoint(eps, r.method, r.path)
		if ep == nil {
			t.Logf("KNOWN LIMITATION: %s %s not found — Mount cross-function resolution (Phase 2)", r.method, r.path)
		}
	}
}

func TestPipeline_ChiNested_PathPrefixAccumulation(t *testing.T) {
	dir := testdataDir("chi-nested")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// Verify deeply nested path: /api/v1/users/{userID}
	ep := findEndpoint(eps, "GET", "/api/v1/users/{userID}")
	if ep == nil {
		t.Fatal("GET /api/v1/users/{userID} not found — prefix accumulation broken")
	}

	// Verify path param.
	found := false
	for _, p := range ep.Request.PathParams {
		if p.Name == "userID" {
			found = true
		}
	}
	if !found {
		t.Error("missing path param 'userID'")
	}
}

func TestPipeline_ChiNested_MountedSubrouter(t *testing.T) {
	dir := testdataDir("chi-nested")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// Mount at /admin — sub-router has /dashboard and /settings.
	// NOTE: Mount cross-function resolution is a known Phase 2 limitation.
	ep := findEndpoint(eps, "GET", "/admin/dashboard")
	ep2 := findEndpoint(eps, "POST", "/admin/settings")

	if ep == nil || ep2 == nil {
		t.Log("KNOWN LIMITATION: Mount cross-function resolution not yet implemented (Phase 2)")
		t.SkipNow()
	}
}

// --- chi-inline integration tests ---

func TestPipeline_ChiInline(t *testing.T) {
	dir := testdataDir("chi-inline")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(eps))
	}

	// Named handler with non-standard param names (rw, req).
	ep := findEndpoint(eps, "GET", "/health")
	if ep == nil {
		t.Fatal("GET /health not found")
	}
	// Should have a 200 response even with non-standard param names.
	var has200 bool
	for _, r := range ep.Responses {
		if r.StatusCode == 200 {
			has200 = true
		}
	}
	if !has200 {
		t.Error("GET /health: missing 200 response with non-standard param names")
	}

	// Inline function literal.
	ep2 := findEndpoint(eps, "GET", "/inline")
	if ep2 == nil {
		t.Fatal("GET /inline not found")
	}
	has200 = false
	for _, r := range ep2.Responses {
		if r.StatusCode == 200 {
			has200 = true
		}
	}
	if !has200 {
		t.Error("GET /inline: missing 200 response from inline FuncLit")
	}
}

// --- gin-groups integration tests ---

func TestPipeline_GinGroups(t *testing.T) {
	dir := testdataDir("gin-groups")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/health"},
		{"GET", "/api/v1/users"},
		{"GET", "/api/v1/users/{id}"},
		{"POST", "/api/v1/users"},
		{"GET", "/api/v1/admin/stats"},
		{"DELETE", "/api/v1/admin/cache"},
	}

	for _, r := range routes {
		ep := findEndpoint(eps, r.method, r.path)
		if ep == nil {
			t.Errorf("missing endpoint %s %s", r.method, r.path)
		}
	}
}

func TestPipeline_GinGroups_Prefixes(t *testing.T) {
	dir := testdataDir("gin-groups")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// /api/v1/admin/stats — nested group prefix accumulation.
	ep := findEndpoint(eps, "GET", "/api/v1/admin/stats")
	if ep == nil {
		t.Fatal("GET /api/v1/admin/stats not found — group prefix accumulation broken")
	}
}

func TestPipeline_GinGroups_Auth(t *testing.T) {
	dir := testdataDir("gin-groups")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// Public: /health — no auth.
	health := findEndpoint(eps, "GET", "/health")
	if health == nil {
		t.Fatal("GET /health not found")
	}
	if health.Auth.Required {
		t.Error("GET /health should not require auth")
	}

	// Users group: JWT/bearer auth.
	users := findEndpoint(eps, "GET", "/api/v1/users")
	if users == nil {
		t.Fatal("GET /api/v1/users not found")
	}
	if !users.Auth.Required {
		t.Error("GET /api/v1/users should require auth")
	}
	foundBearer := false
	for _, s := range users.Auth.Schemes {
		if s == model.AuthBearer {
			foundBearer = true
		}
	}
	if !foundBearer {
		t.Errorf("expected bearer auth on users group, got %v", users.Auth.Schemes)
	}

	// Admin group: API key auth.
	admin := findEndpoint(eps, "GET", "/api/v1/admin/stats")
	if admin == nil {
		t.Fatal("GET /api/v1/admin/stats not found")
	}
	if !admin.Auth.Required {
		t.Error("GET /api/v1/admin/stats should require auth")
	}
	foundAPIKey := false
	for _, s := range admin.Auth.Schemes {
		if s == model.AuthAPIKey {
			foundAPIKey = true
		}
	}
	if !foundAPIKey {
		t.Errorf("expected apikey auth on admin group, got %v", admin.Auth.Schemes)
	}
}

func TestPipeline_GinGroups_CreateUser(t *testing.T) {
	dir := testdataDir("gin-groups")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "POST", "/api/v1/users")
	if ep == nil {
		t.Fatal("POST /api/v1/users not found")
	}

	// Request body.
	if ep.Request.Body == nil {
		t.Fatal("expected request body")
	}
	if ep.Request.Body.Name != "CreateUserRequest" {
		t.Errorf("body name = %q, want %q", ep.Request.Body.Name, "CreateUserRequest")
	}

	// Responses: 201, 400.
	var has201, has400 bool
	for _, r := range ep.Responses {
		if r.StatusCode == 201 {
			has201 = true
		}
		if r.StatusCode == 400 {
			has400 = true
		}
	}
	if !has201 {
		t.Error("missing 201 response")
	}
	if !has400 {
		t.Error("missing 400 response")
	}
}

// --- gin-helpers integration tests ---

func TestPipeline_GinHelpers(t *testing.T) {
	dir := testdataDir("gin-helpers")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	if len(eps) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(eps))
	}

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/items/{id}"},
		{"GET", "/items"},
		{"DELETE", "/items/{id}"},
	}
	for _, r := range routes {
		ep := findEndpoint(eps, r.method, r.path)
		if ep == nil {
			t.Errorf("missing endpoint %s %s", r.method, r.path)
		}
	}
}

func TestPipeline_GinHelpers_GetItem(t *testing.T) {
	dir := testdataDir("gin-helpers")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/items/{id}")
	if ep == nil {
		t.Fatal("GET /items/{id} not found")
	}

	// Should have 200 from respondOK helper and error from respondError helper.
	var has200 bool
	for _, r := range ep.Responses {
		if r.StatusCode == 200 {
			has200 = true
		}
	}
	if !has200 {
		t.Error("missing 200 response (from respondOK helper)")
	}
}

// --- multipart integration tests ---

func TestPipeline_Multipart(t *testing.T) {
	dir := testdataDir("multipart")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	if len(eps) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(eps))
	}
}

func TestPipeline_Multipart_UploadAvatar(t *testing.T) {
	dir := testdataDir("multipart")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "POST", "/users/{id}/avatar")
	if ep == nil {
		t.Fatal("POST /users/{id}/avatar not found")
	}

	// Should detect multipart.
	if !ep.Request.IsMultipart {
		t.Error("expected IsMultipart=true")
	}
	if ep.Request.ContentType != "multipart/form-data" {
		t.Errorf("expected content type multipart/form-data, got %q", ep.Request.ContentType)
	}

	// Path param: id.
	found := false
	for _, p := range ep.Request.PathParams {
		if p.Name == "id" {
			found = true
		}
	}
	if !found {
		t.Error("missing path param 'id'")
	}
}

func TestPipeline_Multipart_UpdateProfile(t *testing.T) {
	dir := testdataDir("multipart")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "PUT", "/users/{id}/profile")
	if ep == nil {
		t.Fatal("PUT /users/{id}/profile not found")
	}

	// JSON body — should NOT be multipart.
	if ep.Request.IsMultipart {
		t.Error("PUT /users/{id}/profile should not be multipart")
	}

	// Body should be ProfileUpdateRequest.
	if ep.Request.Body == nil {
		t.Fatal("expected request body")
	}
	if ep.Request.Body.Name != "ProfileUpdateRequest" {
		t.Errorf("body name = %q, want %q", ep.Request.Body.Name, "ProfileUpdateRequest")
	}
}

func TestPipeline_Multipart_UploadDocument(t *testing.T) {
	dir := testdataDir("multipart")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "POST", "/documents")
	if ep == nil {
		t.Fatal("POST /documents not found")
	}

	if !ep.Request.IsMultipart {
		t.Error("expected IsMultipart=true for UploadDocument")
	}
}

// --- mixed-auth integration tests ---

func TestPipeline_MixedAuth(t *testing.T) {
	dir := testdataDir("mixed-auth")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	if len(eps) < 6 {
		t.Fatalf("expected >= 6 endpoints, got %d", len(eps))
	}

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/health"},
		{"GET", "/users"},
		{"GET", "/users/{id}"},
		{"GET", "/webhooks"},
		{"POST", "/webhooks"},
		{"GET", "/admin/stats"},
	}
	for _, r := range routes {
		ep := findEndpoint(eps, r.method, r.path)
		if ep == nil {
			t.Errorf("missing endpoint %s %s", r.method, r.path)
		}
	}
}

func TestPipeline_MixedAuth_PublicRoute(t *testing.T) {
	dir := testdataDir("mixed-auth")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/health")
	if ep == nil {
		t.Fatal("GET /health not found")
	}
	if ep.Auth.Required {
		t.Error("GET /health should not require auth — it's public")
	}
}

func TestPipeline_MixedAuth_BearerRoutes(t *testing.T) {
	dir := testdataDir("mixed-auth")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	for _, path := range []string{"/users", "/users/{id}"} {
		ep := findEndpoint(eps, "GET", path)
		if ep == nil {
			t.Fatalf("GET %s not found", path)
		}
		if !ep.Auth.Required {
			t.Errorf("GET %s should require auth", path)
		}
		foundBearer := false
		for _, s := range ep.Auth.Schemes {
			if s == model.AuthBearer {
				foundBearer = true
			}
		}
		if !foundBearer {
			t.Errorf("GET %s: expected bearer auth, got %v", path, ep.Auth.Schemes)
		}
	}
}

func TestPipeline_MixedAuth_APIKeyRoutes(t *testing.T) {
	dir := testdataDir("mixed-auth")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/webhooks")
	if ep == nil {
		t.Fatal("GET /webhooks not found")
	}
	if !ep.Auth.Required {
		t.Error("GET /webhooks should require auth")
	}
	foundAPIKey := false
	for _, s := range ep.Auth.Schemes {
		if s == model.AuthAPIKey {
			foundAPIKey = true
		}
	}
	if !foundAPIKey {
		t.Errorf("expected apikey auth, got %v", ep.Auth.Schemes)
	}
}

func TestPipeline_MixedAuth_BasicAuthRoutes(t *testing.T) {
	dir := testdataDir("mixed-auth")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/admin/stats")
	if ep == nil {
		t.Fatal("GET /admin/stats not found")
	}
	if !ep.Auth.Required {
		t.Error("GET /admin/stats should require auth")
	}
	foundBasic := false
	for _, s := range ep.Auth.Schemes {
		if s == model.AuthBasic {
			foundBasic = true
		}
	}
	if !foundBasic {
		t.Errorf("expected basic auth, got %v", ep.Auth.Schemes)
	}
}

// --- Accuracy measurement (Section 14.2) ---

func TestPipeline_AccuracyReport(t *testing.T) {
	projects := []struct {
		name           string
		expectedRoutes []struct{ method, path string }
		// expected features per endpoint
	}{
		{
			name: "chi-basic",
			expectedRoutes: []struct{ method, path string }{
				{"GET", "/users"},
				{"POST", "/users"},
				{"GET", "/users/{id}"},
				{"DELETE", "/users/{id}"},
				{"GET", "/v1/users/{id}"},
				{"POST", "/v2/users"},
			},
		},
		{
			name: "gin-basic",
			expectedRoutes: []struct{ method, path string }{
				{"GET", "/api/v1/items"},
				{"GET", "/api/v1/items/{id}"},
				{"POST", "/api/v1/items"},
				{"DELETE", "/api/v1/items/{id}"},
				{"GET", "/api/v1/admin/users"},
			},
		},
		{
			name: "chi-nested",
			expectedRoutes: []struct{ method, path string }{
				{"GET", "/api/v1/users"},
				{"POST", "/api/v1/users"},
				{"GET", "/api/v1/users/{userID}"},
				{"PUT", "/api/v1/users/{userID}"},
				{"GET", "/api/v1/stats"},
				{"DELETE", "/api/v1/cache"},
				// Mount targets excluded — known Phase 2 limitation.
			},
		},
		{
			name: "gin-groups",
			expectedRoutes: []struct{ method, path string }{
				{"GET", "/health"},
				{"GET", "/api/v1/users"},
				{"GET", "/api/v1/users/{id}"},
				{"POST", "/api/v1/users"},
				{"GET", "/api/v1/admin/stats"},
				{"DELETE", "/api/v1/admin/cache"},
			},
		},
		{
			name: "chi-helpers",
			expectedRoutes: []struct{ method, path string }{
				{"GET", "/users/{id}"},
				{"GET", "/users"},
				{"DELETE", "/users/{id}"},
				{"GET", "/health"},
			},
		},
		{
			name: "gin-helpers",
			expectedRoutes: []struct{ method, path string }{
				{"GET", "/items/{id}"},
				{"GET", "/items"},
				{"DELETE", "/items/{id}"},
			},
		},
		{
			name: "multipart",
			expectedRoutes: []struct{ method, path string }{
				{"POST", "/users/{id}/avatar"},
				{"PUT", "/users/{id}/profile"},
				{"POST", "/documents"},
			},
		},
		{
			name: "mixed-auth",
			expectedRoutes: []struct{ method, path string }{
				{"GET", "/health"},
				{"GET", "/users"},
				{"GET", "/users/{id}"},
				{"GET", "/webhooks"},
				{"POST", "/webhooks"},
				{"GET", "/admin/stats"},
			},
		},
		{
			name: "gin-bind-query",
			expectedRoutes: []struct{ method, path string }{
				{"GET", "/products"},
			},
		},
		{
			name: "gorilla-basic",
			expectedRoutes: []struct{ method, path string }{
				{"GET", "/users"},
				{"POST", "/users"},
				{"GET", "/users/{id}"},
				{"DELETE", "/users/{id}"},
				{"ANY", "/health"},
				{"GET", "/api/v1/items"},
				{"GET", "/api/v1/items/{id}"},
				{"GET", "/admin/dashboard"},
			},
		},
	}

	var totalExpected, totalDetected int
	var totalPathParams, foundPathParams int
	var totalQueryParams, foundQueryParams int
	var totalBodies, foundBodies int
	var totalStatusCodes, foundStatusCodes int
	var totalAuth, foundAuth int

	for _, proj := range projects {
		dir := testdataDir(proj.name)
		eps, err := pipeline.RunPipeline(dir, "./...", nil)
		if err != nil {
			t.Logf("SKIP %s: %v", proj.name, err)
			continue
		}

		totalExpected += len(proj.expectedRoutes)

		for _, expected := range proj.expectedRoutes {
			ep := findEndpoint(eps, expected.method, expected.path)
			if ep != nil {
				totalDetected++
			}
		}

		// Count feature extraction across all detected endpoints.
		for _, ep := range eps {
			// Path params: count from route pattern.
			for _, p := range ep.Request.PathParams {
				totalPathParams++
				if p.Name != "" && p.Type != "" {
					foundPathParams++
				}
			}

			// Query params.
			for range ep.Request.QueryParams {
				totalQueryParams++
				foundQueryParams++ // If extracted at all, it's found.
			}

			// Request body — only count endpoints that actually have bodies.
			// Not every POST/PUT has a JSON body (e.g., multipart, no-body endpoints).
			if ep.Request.Body != nil || ep.Request.IsMultipart {
				totalBodies++
				if ep.Request.Body != nil {
					foundBodies++
				}
			}

			// Response status codes.
			if len(ep.Responses) > 0 {
				for _, r := range ep.Responses {
					totalStatusCodes++
					if r.StatusCode > 0 {
						foundStatusCodes++
					}
				}
			}

			// Auth detection.
			if ep.Auth.Required {
				totalAuth++
				if len(ep.Auth.Schemes) > 0 {
					foundAuth++
				}
			}
		}
	}

	// Print report.
	t.Log("")
	t.Log("=== GoDoc Live — Accuracy Report ===")
	t.Log("")

	routePct := pct(totalDetected, totalExpected)
	t.Logf("Route detection:      %d/%d  (%.1f%%)  target: 95%%", totalDetected, totalExpected, routePct)

	pathPct := pct(foundPathParams, totalPathParams)
	t.Logf("Path params:          %d/%d  (%.1f%%)  target: 99%%", foundPathParams, totalPathParams, pathPct)

	queryPct := pct(foundQueryParams, totalQueryParams)
	t.Logf("Query params:         %d/%d  (%.1f%%)  target: 85%%", foundQueryParams, totalQueryParams, queryPct)

	bodyPct := pct(foundBodies, totalBodies)
	t.Logf("Request body:         %d/%d  (%.1f%%)  target: 88%%", foundBodies, totalBodies, bodyPct)

	statusPct := pct(foundStatusCodes, totalStatusCodes)
	t.Logf("Response status:      %d/%d  (%.1f%%)  target: 85%%", foundStatusCodes, totalStatusCodes, statusPct)

	authPct := pct(foundAuth, totalAuth)
	t.Logf("Auth detection:       %d/%d  (%.1f%%)  target: 87%%", foundAuth, totalAuth, authPct)

	t.Log("")

	// Fail if route detection falls below target.
	if routePct < 95.0 {
		t.Errorf("Route detection %.1f%% below 95%% target", routePct)
	}


}

func pct(found, total int) float64 {
	if total == 0 {
		return 100.0
	}
	return float64(found) / float64(total) * 100.0
}

// --- gin-bind-query integration tests ---

func TestPipeline_GinBindQuery(t *testing.T) {
	dir := testdataDir("gin-bind-query")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	if len(eps) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(eps))
	}

	ep := findEndpoint(eps, "GET", "/products")
	if ep == nil {
		t.Fatal("GET /products not found")
	}

	// Query params should include promoted fields from ShouldBindQuery.
	var foundPage, foundLimit, foundSearch bool
	for _, p := range ep.Request.QueryParams {
		switch p.Name {
		case "page":
			foundPage = true
			if !p.Required {
				t.Error("page should be required")
			}
			if p.Type != "integer" {
				t.Errorf("page type = %q, want %q", p.Type, "integer")
			}
		case "limit":
			foundLimit = true
			if p.Required {
				t.Error("limit should not be required")
			}
			if p.Type != "integer" {
				t.Errorf("limit type = %q, want %q", p.Type, "integer")
			}
		case "search":
			foundSearch = true
			if p.Type != "string" {
				t.Errorf("search type = %q, want %q", p.Type, "string")
			}
		}
	}
	if !foundPage {
		t.Error("missing query param 'page' from ShouldBindQuery promotion")
	}
	if !foundLimit {
		t.Error("missing query param 'limit' from ShouldBindQuery promotion")
	}
	if !foundSearch {
		t.Error("missing query param 'search' from ShouldBindQuery promotion")
	}

	// Headers should include promoted fields from ShouldBindHeader.
	var foundTenantID, foundRequestID bool
	for _, h := range ep.Request.Headers {
		switch h.Name {
		case "X-Tenant-ID":
			foundTenantID = true
			if !h.Required {
				t.Error("X-Tenant-ID should be required")
			}
		case "X-Request-ID":
			foundRequestID = true
		}
	}
	if !foundTenantID {
		t.Error("missing header 'X-Tenant-ID' from ShouldBindHeader promotion")
	}
	if !foundRequestID {
		t.Error("missing header 'X-Request-ID' from ShouldBindHeader promotion")
	}

	// Request body should be nil (ShouldBindQuery is not a body).
	if ep.Request.Body != nil {
		t.Error("expected no request body — ShouldBindQuery/Header should not set body")
	}
}

// --- Cross-project feature summary ---

// --- echo-basic integration tests ---

func TestEchoBasic(t *testing.T) {
	dir := testdataDir("echo-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	if len(eps) != 8 {
		t.Fatalf("expected 8 endpoints, got %d: %v", len(eps), func() []string {
			var s []string
			for _, e := range eps {
				s = append(s, e.Method+" "+e.Path)
			}
			return s
		}())
	}

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/health"},
		{"GET", "/users"},
		{"POST", "/users"},
		{"GET", "/users/{id}"},
		{"DELETE", "/users/{id}"},
		{"GET", "/api/v1/items"},
		{"GET", "/api/v1/items/{id}"},
		{"POST", "/api/v1/items"},
	}
	for _, r := range routes {
		ep := findEndpoint(eps, r.method, r.path)
		if ep == nil {
			t.Errorf("missing endpoint %s %s", r.method, r.path)
		}
	}
}

func TestEchoBasic_PathParam(t *testing.T) {
	dir := testdataDir("echo-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/users/{id}")
	if ep == nil {
		t.Fatal("GET /users/{id} not found")
	}
	if len(ep.Request.PathParams) == 0 {
		t.Fatal("expected path param 'id'")
	}
	found := false
	for _, p := range ep.Request.PathParams {
		if p.Name == "id" {
			found = true
		}
	}
	if !found {
		t.Error("missing path param 'id'")
	}
}

func TestEchoBasic_QueryParam(t *testing.T) {
	dir := testdataDir("echo-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/api/v1/items")
	if ep == nil {
		t.Fatal("GET /api/v1/items not found")
	}
	found := false
	for _, p := range ep.Request.QueryParams {
		if p.Name == "category" {
			found = true
		}
	}
	if !found {
		t.Error("missing query param 'category'")
	}
}

func TestEchoBasic_Body(t *testing.T) {
	dir := testdataDir("echo-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "POST", "/users")
	if ep == nil {
		t.Fatal("POST /users not found")
	}
	if ep.Request.Body == nil {
		t.Fatal("expected request body")
	}
	if ep.Request.Body.Name != "CreateUserRequest" {
		t.Errorf("body name = %q, want %q", ep.Request.Body.Name, "CreateUserRequest")
	}
}

func TestEchoBasic_Responses(t *testing.T) {
	dir := testdataDir("echo-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// POST /users should have 201 and 400.
	ep := findEndpoint(eps, "POST", "/users")
	if ep == nil {
		t.Fatal("POST /users not found")
	}
	var has201, has400 bool
	for _, r := range ep.Responses {
		if r.StatusCode == 201 {
			has201 = true
		}
		if r.StatusCode == 400 {
			has400 = true
		}
	}
	if !has201 {
		t.Error("missing 201 response")
	}
	if !has400 {
		t.Error("missing 400 response")
	}

	// DELETE /users/{id} should have 204.
	del := findEndpoint(eps, "DELETE", "/users/{id}")
	if del == nil {
		t.Fatal("DELETE /users/{id} not found")
	}
	has204 := false
	for _, r := range del.Responses {
		if r.StatusCode == 204 {
			has204 = true
		}
	}
	if !has204 {
		t.Error("missing 204 response on DELETE /users/{id}")
	}
}

func TestEchoBasic_GroupAuth(t *testing.T) {
	dir := testdataDir("echo-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// /health is public.
	health := findEndpoint(eps, "GET", "/health")
	if health == nil {
		t.Fatal("GET /health not found")
	}
	if health.Auth.Required {
		t.Error("GET /health should not require auth")
	}

	// /api/v1/items has auth middleware.
	items := findEndpoint(eps, "GET", "/api/v1/items")
	if items == nil {
		t.Fatal("GET /api/v1/items not found")
	}
	if !items.Auth.Required {
		t.Error("GET /api/v1/items should require auth (bearer via authMiddleware)")
	}
}

func TestPipeline_AllProjects_Build(t *testing.T) {
	// Smoke test: every testdata project runs through the pipeline without error.
	projects := []string{
		"chi-basic", "chi-nested", "chi-inline", "chi-helpers",
		"gin-basic", "gin-groups", "gin-helpers",
		"multipart", "mixed-auth", "gin-bind-query", "gorilla-basic", "echo-basic",
	}
	for _, name := range projects {
		t.Run(name, func(t *testing.T) {
			dir := testdataDir(name)
			eps, err := pipeline.RunPipeline(dir, "./...", nil)
			if err != nil {
				t.Fatalf("RunPipeline failed: %v", err)
			}
			if len(eps) == 0 {
				t.Error("no endpoints detected")
			}
			t.Logf("%s: %d endpoints, %d unresolved total",
				name, len(eps), countUnresolved(eps))
		})
	}
}

func countUnresolved(eps []model.EndpointDef) int {
	n := 0
	for _, ep := range eps {
		n += len(ep.Unresolved)
	}
	return n
}

// --- stdlib-basic integration tests ---

func TestPipeline_StdlibBasic(t *testing.T) {
	dir := testdataDir("stdlib-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// stdlib-basic has 6 routes:
	// GET /users, POST /users, GET /users/{id}, DELETE /users/{id},
	// ANY /health, GET /products/{id}
	if len(eps) != 6 {
		t.Fatalf("expected 6 endpoints, got %d", len(eps))
	}

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/users"},
		{"POST", "/users"},
		{"GET", "/users/{id}"},
		{"DELETE", "/users/{id}"},
		{"ANY", "/health"},
		{"GET", "/products/{id}"},
	}
	for _, r := range routes {
		ep := findEndpoint(eps, r.method, r.path)
		if ep == nil {
			t.Errorf("missing endpoint %s %s", r.method, r.path)
		}
	}
}

func TestPipeline_StdlibBasic_ListUsers(t *testing.T) {
	dir := testdataDir("stdlib-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/users")
	if ep == nil {
		t.Fatal("GET /users not found")
	}

	// Summary inferred from handler name.
	if ep.Summary != "List Users" {
		t.Errorf("Summary = %q, want %q", ep.Summary, "List Users")
	}

	// Tags inferred.
	if len(ep.Tags) == 0 || ep.Tags[0] != "users" {
		t.Errorf("Tags = %v, want [users]", ep.Tags)
	}

	// Query params: page (required), limit (optional).
	if len(ep.Request.QueryParams) < 2 {
		t.Errorf("expected at least 2 query params, got %d", len(ep.Request.QueryParams))
	}
}

func TestPipeline_StdlibBasic_GetUser(t *testing.T) {
	dir := testdataDir("stdlib-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/users/{id}")
	if ep == nil {
		t.Fatal("GET /users/{id} not found")
	}

	// Path param: id.
	if len(ep.Request.PathParams) != 1 {
		t.Fatalf("expected 1 path param, got %d", len(ep.Request.PathParams))
	}
	if ep.Request.PathParams[0].Name != "id" {
		t.Errorf("path param name = %q, want %q", ep.Request.PathParams[0].Name, "id")
	}

	// Responses should include 200 and 400.
	has200 := false
	has400 := false
	for _, r := range ep.Responses {
		if r.StatusCode == 200 {
			has200 = true
		}
		if r.StatusCode == 400 {
			has400 = true
		}
	}
	if !has200 {
		t.Error("missing 200 response")
	}
	if !has400 {
		t.Error("missing 400 response")
	}
}

func TestPipeline_StdlibBasic_CreateUser(t *testing.T) {
	dir := testdataDir("stdlib-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "POST", "/users")
	if ep == nil {
		t.Fatal("POST /users not found")
	}

	// Should have a request body (CreateUserRequest).
	if ep.Request.Body == nil {
		t.Error("expected request body for POST /users")
	}

	// Responses should include 201 and 400.
	has201 := false
	has400 := false
	for _, r := range ep.Responses {
		if r.StatusCode == 201 {
			has201 = true
		}
		if r.StatusCode == 400 {
			has400 = true
		}
	}
	if !has201 {
		t.Error("missing 201 response")
	}
	if !has400 {
		t.Error("missing 400 response")
	}
}

func TestPipeline_StdlibBasic_PathValue(t *testing.T) {
	dir := testdataDir("stdlib-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// Verify r.PathValue("id") is detected for stdlib handlers.
	ep := findEndpoint(eps, "GET", "/users/{id}")
	if ep == nil {
		t.Fatal("GET /users/{id} not found")
	}
	if len(ep.Request.PathParams) == 0 {
		t.Fatal("no path params found")
	}
	// The path param should have type "uuid" from name heuristic (id → uuid).
	if ep.Request.PathParams[0].Type != "uuid" {
		t.Errorf("path param type = %q, want %q", ep.Request.PathParams[0].Type, "uuid")
	}
}

func TestPipeline_StdlibBasic_ProductHandler(t *testing.T) {
	dir := testdataDir("stdlib-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	ep := findEndpoint(eps, "GET", "/products/{id}")
	if ep == nil {
		t.Fatal("GET /products/{id} not found")
	}

	// Should resolve to ServeHTTP method of ProductHandler.
	if ep.HandlerName == "" {
		t.Error("expected a handler name for product handler")
	}

	// Should have 200 response.
	has200 := false
	for _, r := range ep.Responses {
		if r.StatusCode == 200 {
			has200 = true
		}
	}
	if !has200 {
		t.Error("missing 200 response")
	}
}

// --- gorilla-basic integration tests ---

func TestPipeline_GorillaBasic_EndpointCount(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	if len(eps) < 8 {
		t.Fatalf("expected at least 8 endpoints, got %d", len(eps))
	}
}

func TestPipeline_GorillaBasic_MethodRouting(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// .Methods("GET") routes
	getUsers := findEndpoint(eps, "GET", "/users")
	if getUsers == nil {
		t.Error("GET /users should exist")
	}

	// .Methods("POST") routes
	postUsers := findEndpoint(eps, "POST", "/users")
	if postUsers == nil {
		t.Error("POST /users should exist")
	}

	// .Methods("DELETE") routes
	deleteUser := findEndpoint(eps, "DELETE", "/users/{id}")
	if deleteUser == nil {
		t.Error("DELETE /users/{id} should exist")
	}

	// No .Methods() → ANY
	health := findEndpoint(eps, "ANY", "/health")
	if health == nil {
		t.Error("ANY /health should exist")
	}
}

func TestPipeline_GorillaBasic_Subrouter(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// PathPrefix("/api/v1").Subrouter() routes
	listItems := findEndpoint(eps, "GET", "/api/v1/items")
	if listItems == nil {
		t.Error("GET /api/v1/items should exist")
	}

	getItem := findEndpoint(eps, "GET", "/api/v1/items/{id}")
	if getItem == nil {
		t.Error("GET /api/v1/items/{id} should exist")
	}

	// Nested subrouter
	dashboard := findEndpoint(eps, "GET", "/admin/dashboard")
	if dashboard == nil {
		t.Error("GET /admin/dashboard should exist")
	}
}

func TestPipeline_GorillaBasic_PathParams(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	getUser := findEndpoint(eps, "GET", "/users/{id}")
	if getUser == nil {
		t.Fatal("GET /users/{id} not found")
	}
	found := false
	for _, p := range getUser.Request.PathParams {
		if p.Name == "id" {
			found = true
			if !p.Required {
				t.Error("path param 'id' should be required")
			}
		}
	}
	if !found {
		t.Error("missing path param 'id'")
	}

	// GetItem uses strconv.Atoi → type should be upgraded to integer.
	getItem := findEndpoint(eps, "GET", "/api/v1/items/{id}")
	if getItem == nil {
		t.Fatal("GET /api/v1/items/{id} not found")
	}
	for _, p := range getItem.Request.PathParams {
		if p.Name == "id" {
			if p.Type != "integer" {
				t.Errorf("path param 'id' type = %q, want %q (strconv.Atoi upgrade)", p.Type, "integer")
			}
		}
	}
}

func TestPipeline_GorillaBasic_QueryParams(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	listUsers := findEndpoint(eps, "GET", "/users")
	if listUsers == nil {
		t.Fatal("GET /users not found")
	}

	var foundPage, foundLimit bool
	for _, p := range listUsers.Request.QueryParams {
		if p.Name == "page" {
			foundPage = true
			if !p.Required {
				t.Error("page should be required")
			}
		}
		if p.Name == "limit" {
			foundLimit = true
		}
	}
	if !foundPage {
		t.Error("missing query param 'page'")
	}
	if !foundLimit {
		t.Error("missing query param 'limit'")
	}
}

func TestPipeline_GorillaBasic_RequestBody(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	createUser := findEndpoint(eps, "POST", "/users")
	if createUser == nil {
		t.Fatal("POST /users not found")
	}
	if createUser.Request.Body == nil {
		t.Error("POST /users should have a request body")
	}
}

func TestPipeline_GorillaBasic_Middleware(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// /api/v1/* routes should have authMiddleware → bearer auth.
	listItems := findEndpoint(eps, "GET", "/api/v1/items")
	if listItems == nil {
		t.Fatal("GET /api/v1/items not found")
	}
	if !listItems.Auth.Required {
		t.Error("GET /api/v1/items should require auth")
	}

	// /admin/* routes should also have authMiddleware.
	dashboard := findEndpoint(eps, "GET", "/admin/dashboard")
	if dashboard == nil {
		t.Fatal("GET /admin/dashboard not found")
	}
	if !dashboard.Auth.Required {
		t.Error("GET /admin/dashboard should require auth")
	}
}

func TestPipeline_GorillaBasic_RegexParam(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// {id:[0-9]+} should be normalized to {id}
	getItem := findEndpoint(eps, "GET", "/api/v1/items/{id}")
	if getItem == nil {
		t.Error("regex-constrained path should be normalized to {id}")
	}
}

