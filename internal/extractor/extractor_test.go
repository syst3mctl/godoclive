package extractor_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/syst3mctl/godoclive/internal/extractor"
	"github.com/syst3mctl/godoclive/internal/loader"
)

func testdataDir(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
}

// --- Chi extractor tests ---

func TestChiExtractor_Basic(t *testing.T) {
	dir := testdataDir("chi-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// chi-basic has: GET /users, POST /users, GET /users/{id}, DELETE /users/{id},
	// GET /v1/users/{id} (deprecated), POST /v2/users (io.ReadAll pattern).
	expected := []struct {
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

	if len(routes) != len(expected) {
		t.Fatalf("expected %d routes, got %d", len(expected), len(routes))
		for _, r := range routes {
			t.Logf("  %s %s", r.Method, r.Path)
		}
	}

	routeMap := make(map[string]extractor.RawRoute)
	for _, r := range routes {
		key := r.Method + " " + r.Path
		routeMap[key] = r
	}

	for _, exp := range expected {
		key := exp.method + " " + exp.path
		r, ok := routeMap[key]
		if !ok {
			t.Errorf("missing route: %s", key)
			continue
		}
		if r.HandlerExpr == nil {
			t.Errorf("route %s has nil HandlerExpr", key)
		}
		if r.File == "" {
			t.Errorf("route %s has empty File", key)
		}
		if r.Line == 0 {
			t.Errorf("route %s has zero Line", key)
		}
	}

	// Verify middleware: POST /users, GET /users/{id}, DELETE /users/{id} should have JWTAuth middleware.
	// GET /users (ListUsers) is outside the auth group so should only have scope middleware from .Use(middleware.Logger).
	for _, r := range routes {
		key := r.Method + " " + r.Path
		if key == "GET /users" {
			// Outside the auth group — should not have JWTAuth.
			for _, mw := range r.Middlewares {
				t.Logf("GET /users middleware: %T", mw)
			}
		}
	}
}

func TestChiExtractor_Nested(t *testing.T) {
	dir := testdataDir("chi-nested")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// chi-nested main() has inline routes:
	// r.Route("/api/v1", func) containing:
	//   r.Route("/users", func) containing:
	//     GET  /  → /api/v1/users
	//     POST /  → /api/v1/users
	//     r.Route("/{userID}", func) containing:
	//       GET /  → /api/v1/users/{userID}
	//       PUT /  → /api/v1/users/{userID}
	//   r.Group(func) containing:
	//     .Use(AdminOnly)
	//     GET  /stats  → /api/v1/stats
	//     DELETE /cache → /api/v1/cache
	// r.Mount("/admin", adminRouter()) — adminRouter() is a separate factory
	// function, followed through the mount so its routes carry the /admin
	// prefix:
	//   GET  /dashboard → /admin/dashboard
	//   POST /settings  → /admin/settings
	expected := map[string]bool{
		"GET /api/v1/users":          true,
		"POST /api/v1/users":         true,
		"GET /api/v1/users/{userID}": true,
		"PUT /api/v1/users/{userID}": true,
		"GET /api/v1/stats":          true,
		"DELETE /api/v1/cache":       true,
		"GET /admin/dashboard":       true,
		"POST /admin/settings":       true,
	}

	if len(routes) != len(expected) {
		t.Errorf("expected %d routes, got %d", len(expected), len(routes))
		for _, r := range routes {
			t.Logf("  found: %s %s (line %d)", r.Method, r.Path, r.Line)
		}
	}

	for _, r := range routes {
		key := r.Method + " " + r.Path
		if !expected[key] {
			t.Errorf("unexpected route: %s", key)
		}
		delete(expected, key)
	}

	for key := range expected {
		t.Errorf("missing route: %s", key)
	}

	// Verify AdminOnly middleware on /stats and /cache.
	for _, r := range routes {
		key := r.Method + " " + r.Path
		if key == "GET /api/v1/stats" || key == "DELETE /api/v1/cache" {
			if len(r.Middlewares) == 0 {
				t.Errorf("route %s should have AdminOnly middleware", key)
			}
		}
	}
}

func TestChiExtractor_PathPrefixAccumulation(t *testing.T) {
	dir := testdataDir("chi-nested")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Verify deeply nested path: /api/v1/users/{userID}
	found := false
	for _, r := range routes {
		if r.Method == "GET" && r.Path == "/api/v1/users/{userID}" {
			found = true
			break
		}
	}
	if !found {
		t.Error("deeply nested path /api/v1/users/{userID} not found — prefix accumulation broken")
	}
}

// --- Gin extractor tests ---

func TestGinExtractor_Basic(t *testing.T) {
	dir := testdataDir("gin-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.GinExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// gin-basic has:
	// v1 := r.Group("/api/v1")
	//   v1.GET("/items", ...)         → /api/v1/items
	//   v1.GET("/items/:id", ...)     → /api/v1/items/{id}
	//   v1.POST("/items", ...)        → /api/v1/items
	//   v1.DELETE("/items/:id", ...)  → /api/v1/items/{id}
	//   admin := v1.Group("/admin")
	//     admin.GET("/users", ...)    → /api/v1/admin/users
	expected := map[string]bool{
		"GET /api/v1/items":         true,
		"GET /api/v1/items/{id}":    true,
		"POST /api/v1/items":        true,
		"DELETE /api/v1/items/{id}": true,
		"GET /api/v1/admin/users":   true,
	}

	if len(routes) != len(expected) {
		t.Errorf("expected %d routes, got %d", len(expected), len(routes))
		for _, r := range routes {
			t.Logf("  found: %s %s (line %d)", r.Method, r.Path, r.Line)
		}
	}

	for _, r := range routes {
		key := r.Method + " " + r.Path
		if !expected[key] {
			t.Errorf("unexpected route: %s", key)
		}
		delete(expected, key)
	}

	for key := range expected {
		t.Errorf("missing route: %s", key)
	}
}

func TestGinExtractor_PathNormalization(t *testing.T) {
	dir := testdataDir("gin-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.GinExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Verify :id was normalized to {id}.
	for _, r := range routes {
		if r.Path == "/api/v1/items/:id" {
			t.Errorf("gin path not normalized: got %s, want /api/v1/items/{id}", r.Path)
		}
	}

	// Verify {id} format exists.
	found := false
	for _, r := range routes {
		if r.Method == "GET" && r.Path == "/api/v1/items/{id}" {
			found = true
			break
		}
	}
	if !found {
		t.Error("normalized path /api/v1/items/{id} not found")
	}
}

func TestGinExtractor_Middleware(t *testing.T) {
	dir := testdataDir("gin-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.GinExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// admin routes should have AuthRequired middleware.
	for _, r := range routes {
		if r.Path == "/api/v1/admin/users" {
			if len(r.Middlewares) == 0 {
				t.Error("admin route should have middleware (AuthRequired)")
			}
		}
	}
}

// --- Stdlib extractor tests ---

func TestStdlibExtractor_Basic(t *testing.T) {
	dir := testdataDir("stdlib-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.StdlibExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// stdlib-basic has:
	// GET /users, POST /users, GET /users/{id}, DELETE /users/{id},
	// /health (ANY), GET /products/{id} (http.Handler)
	expected := map[string]bool{
		"GET /users":         true,
		"POST /users":        true,
		"GET /users/{id}":    true,
		"DELETE /users/{id}": true,
		"ANY /health":        true,
		"GET /products/{id}": true,
	}

	if len(routes) != len(expected) {
		t.Errorf("expected %d routes, got %d", len(expected), len(routes))
		for _, r := range routes {
			t.Logf("  found: %s %s (line %d)", r.Method, r.Path, r.Line)
		}
	}

	for _, r := range routes {
		key := r.Method + " " + r.Path
		if !expected[key] {
			t.Errorf("unexpected route: %s", key)
		}
		delete(expected, key)
	}

	for key := range expected {
		t.Errorf("missing route: %s", key)
	}

	// Verify all routes have handler expressions and file/line info.
	for _, r := range routes {
		if r.HandlerExpr == nil {
			t.Errorf("route %s %s has nil HandlerExpr", r.Method, r.Path)
		}
		if r.File == "" {
			t.Errorf("route %s %s has empty File", r.Method, r.Path)
		}
		if r.Line == 0 {
			t.Errorf("route %s %s has zero Line", r.Method, r.Path)
		}
	}
}

func TestStdlibExtractor_Conditional(t *testing.T) {
	dir := testdataDir("stdlib-conditional")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.StdlibExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Every route below is registered inside an if/else/switch block except
	// GET /v1/auth/login (top level). GET /metrics is suppressed by a
	// //godoclive:ignore directive; GET /livez sits directly below it and must
	// still be discovered (the directive applies to a single statement only).
	expected := map[string]bool{
		"GET /v1/auth/login":            true,
		"POST /v1/sessions":             true,
		"GET /v1/sessions/{id}":         true,
		"GET /v1/sessions/{id}/receipt": true,
		"GET /v1/users/me":              true,
		"GET /v1/users/{id}":            true,
		"DELETE /v1/users/{id}":         true,
		"GET /v1/status":                true,
		"GET /livez":                    true,
	}

	if len(routes) != len(expected) {
		t.Errorf("expected %d routes, got %d", len(expected), len(routes))
		for _, r := range routes {
			t.Logf("  found: %s %s (line %d)", r.Method, r.Path, r.Line)
		}
	}

	for _, r := range routes {
		key := r.Method + " " + r.Path
		if key == "GET /metrics" {
			t.Errorf("route GET /metrics should have been ignored")
		}
		if !expected[key] {
			t.Errorf("unexpected route: %s", key)
		}
		delete(expected, key)
	}

	for key := range expected {
		t.Errorf("missing route: %s", key)
	}
}

// --- Gorilla extractor tests ---

func TestGorillaExtractor_Basic(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.GorillaExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// gorilla-basic has:
	// GET /users, POST /users, GET /users/{id}, DELETE /users/{id} (.Methods() chain)
	// ANY /health (no .Methods())
	// GET /api/v1/items, GET /api/v1/items/{id} (subrouter with PathPrefix + .Methods())
	// GET /admin/dashboard (nested subrouter)
	expected := map[string]bool{
		"GET /users":             true,
		"POST /users":            true,
		"GET /users/{id}":        true,
		"DELETE /users/{id}":     true,
		"ANY /health":            true,
		"GET /api/v1/items":      true,
		"GET /api/v1/items/{id}": true,
		"GET /admin/dashboard":   true,
	}

	if len(routes) != len(expected) {
		t.Errorf("expected %d routes, got %d", len(expected), len(routes))
		for _, r := range routes {
			t.Logf("  found: %s %s (line %d)", r.Method, r.Path, r.Line)
		}
	}

	for _, r := range routes {
		key := r.Method + " " + r.Path
		if !expected[key] {
			t.Errorf("unexpected route: %s", key)
		}
		delete(expected, key)
	}

	for key := range expected {
		t.Errorf("missing route: %s", key)
	}

	// Verify all routes have handler expressions and file/line info.
	for _, r := range routes {
		if r.HandlerExpr == nil {
			t.Errorf("route %s %s has nil HandlerExpr", r.Method, r.Path)
		}
		if r.File == "" {
			t.Errorf("route %s %s has empty File", r.Method, r.Path)
		}
		if r.Line == 0 {
			t.Errorf("route %s %s has zero Line", r.Method, r.Path)
		}
	}
}

func TestGorillaExtractor_MethodRouting(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.GorillaExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Routes with .Methods() should have specific methods; without → ANY.
	methodMap := make(map[string]string)
	for _, r := range routes {
		methodMap[r.Method+" "+r.Path] = r.Method
	}

	if _, ok := methodMap["ANY /health"]; !ok {
		t.Error("route without .Methods() should have method ANY")
	}
	if _, ok := methodMap["GET /users"]; !ok {
		t.Error("GET /users should exist from .Methods(\"GET\")")
	}
	if _, ok := methodMap["POST /users"]; !ok {
		t.Error("POST /users should exist from .Methods(\"POST\")")
	}
}

func TestGorillaExtractor_SubrouterPrefixes(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.GorillaExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Verify prefix accumulation from PathPrefix().Subrouter().
	found := map[string]bool{}
	for _, r := range routes {
		found[r.Method+" "+r.Path] = true
	}

	if !found["GET /api/v1/items"] {
		t.Error("GET /api/v1/items not found — subrouter prefix accumulation broken")
	}
	if !found["GET /admin/dashboard"] {
		t.Error("GET /admin/dashboard not found — nested subrouter prefix broken")
	}
}

func TestGorillaExtractor_RegexNormalization(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.GorillaExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// {id:[0-9]+} should be normalized to {id}.
	for _, r := range routes {
		if r.Path == "/api/v1/items/{id:[0-9]+}" {
			t.Errorf("path not normalized: got %s, want /api/v1/items/{id}", r.Path)
		}
	}

	found := false
	for _, r := range routes {
		if r.Method == "GET" && r.Path == "/api/v1/items/{id}" {
			found = true
			break
		}
	}
	if !found {
		t.Error("normalized path /api/v1/items/{id} not found")
	}
}

func TestGorillaExtractor_Middleware(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.GorillaExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Subrouter routes (/api/v1/* and /admin/*) should have authMiddleware.
	for _, r := range routes {
		key := r.Method + " " + r.Path
		if key == "GET /api/v1/items" || key == "GET /admin/dashboard" {
			if len(r.Middlewares) == 0 {
				t.Errorf("route %s should have middleware (authMiddleware)", key)
			}
		}
	}

	// Root routes (/users, /health) should not have authMiddleware
	// (only loggingMiddleware from r.Use).
	for _, r := range routes {
		if r.Path == "/health" || r.Path == "/users" {
			// They should have the root loggingMiddleware but not authMiddleware.
			// Just verify they have some middleware from root r.Use().
			if len(r.Middlewares) == 0 {
				t.Logf("route %s %s has no middlewares (loggingMiddleware expected from r.Use)", r.Method, r.Path)
			}
		}
	}
}

func TestNormalizeGorillaPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/users/{id}", "/users/{id}"},
		{"/users/{id:[0-9]+}", "/users/{id}"},
		{"/files/{path:.*}", "/files/{path}"},
		{"/items/{id:[0-9]+}/reviews/{reviewID}", "/items/{id}/reviews/{reviewID}"},
		{"/no-params", "/no-params"},
	}
	for _, tt := range tests {
		got := extractor.NormalizeGorillaPath(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeGorillaPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- Stdlib extractor tests ---

func TestStdlibExtractor_PatternParsing(t *testing.T) {
	dir := testdataDir("stdlib-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.StdlibExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// /health should have method "ANY" since no method prefix.
	for _, r := range routes {
		if r.Path == "/health" {
			if r.Method != "ANY" {
				t.Errorf("/health method = %q, want %q", r.Method, "ANY")
			}
		}
	}

	// {id} should be preserved as-is (Go 1.22+ native format).
	found := false
	for _, r := range routes {
		if r.Method == "GET" && r.Path == "/users/{id}" {
			found = true
			break
		}
	}
	if !found {
		t.Error("path parameter /users/{id} not found")
	}
}

// TestChiExtractor_MultiPackage covers routes registered outside main(), the
// layout of every real chi service: main() owns the router and hands it to
// another package, and sub-routers are built by factories whose signatures name
// no chi type. Regression test for issue #33, where none of these were found.
func TestChiExtractor_MultiPackage(t *testing.T) {
	dir := testdataDir("chi-multipkg")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	expected := map[string]bool{
		// routes.Register(r), called from main() at the root prefix.
		"GET /health": true,
		// registerPayments(r), reached through r.Route("/api/v1", …) so the
		// prefix of the call site flows into the registrar.
		"GET /api/v1/payments":             true,
		"POST /api/v1/payments":            true,
		"GET /api/v1/payments/{paymentID}": true,
		// admin.Router() builds its own router and is mounted under /admin.
		"GET /admin/dashboard": true,
		// A router held in a struct field, registered from a method.
		"GET /status": true,
		// Route methods promoted from an embedded chi.Router.
		"GET /embedded": true,
	}

	seen := map[string]bool{}
	for _, r := range routes {
		key := r.Method + " " + r.Path
		if seen[key] {
			t.Errorf("route emitted twice: %s", key)
		}
		seen[key] = true
		if !expected[key] {
			t.Errorf("unexpected route: %s (%s:%d)", key, r.File, r.Line)
		}
	}
	for key := range expected {
		if !seen[key] {
			t.Errorf("missing route: %s", key)
		}
	}

	// Every route reached a resolvable call site, so none carries an
	// unknown-origin caveat.
	for _, r := range routes {
		if len(r.Unresolved) > 0 {
			t.Errorf("%s %s: unexpected caveat %v", r.Method, r.Path, r.Unresolved)
		}
	}
}

// TestChiExtractor_MountedRouterNotDuplicated pins the rule that a factory
// mounted by another function is emitted only under its mount prefix — never a
// second time at the bare path it uses internally.
func TestChiExtractor_MountedRouterNotDuplicated(t *testing.T) {
	dir := testdataDir("chi-multipkg")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	for _, r := range routes {
		if r.Path == "/dashboard" {
			t.Errorf("mounted route emitted without its /admin prefix (%s:%d)", r.File, r.Line)
		}
	}
}

// TestChiExtractor_NonRouterMethodIgnored guards the type check on the
// receiver: walking every function means unrelated methods that happen to be
// named Get or Post are now in scope, and must not become routes.
func TestChiExtractor_NonRouterMethodIgnored(t *testing.T) {
	dir := testdataDir("chi-multipkg")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	for _, r := range routes {
		if r.Path == "warm" {
			t.Errorf("cache.Get(\"warm\", …) was extracted as a route (%s:%d)", r.File, r.Line)
		}
	}
}
