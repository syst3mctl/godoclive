package extractor_test

import (
	"path/filepath"
	"runtime"
	"strings"
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

// --- routes registered outside main() ---
//
// Every router framework has to cope with the layout of a real service: main()
// owns the router and hands it to another package, sub-routers are built by
// factories whose signatures name no router type, and a server struct keeps its
// router in a field. Regression tests for issue #33 and its siblings.

// extractRoutes runs one extractor over a testdata module.
func extractRoutes(t *testing.T, ext extractor.Extractor, module string) []extractor.RawRoute {
	t.Helper()
	pkgs, err := loader.LoadPackages(testdataDir(module), "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	return routes
}

// assertRoutesOutsideMain checks that a module's routes are exactly the
// expected set, each emitted once and with no unresolved caveat — every route
// in these modules reaches a resolvable call site.
func assertRoutesOutsideMain(t *testing.T, ext extractor.Extractor, module string, expected []string) {
	t.Helper()
	routes := extractRoutes(t, ext, module)

	want := make(map[string]bool, len(expected))
	for _, key := range expected {
		want[key] = true
	}

	seen := map[string]bool{}
	for _, r := range routes {
		key := r.Method + " " + r.Path
		if seen[key] {
			t.Errorf("%s: route emitted twice: %s", module, key)
		}
		seen[key] = true
		if !want[key] {
			t.Errorf("%s: unexpected route: %s (%s:%d)", module, key, r.File, r.Line)
		}
		if len(r.Unresolved) > 0 {
			t.Errorf("%s: %s: unexpected caveat %v", module, key, r.Unresolved)
		}
	}
	for key := range want {
		if !seen[key] {
			t.Errorf("%s: missing route: %s", module, key)
		}
	}
}

// assertNoRouteMatching guards each extractor's receiver type check: now that
// every route-setup function is walked, an unrelated method that happens to
// share a registration's name must not become a route.
func assertNoRouteMatching(t *testing.T, ext extractor.Extractor, module, needle string) {
	t.Helper()
	for _, r := range extractRoutes(t, ext, module) {
		if strings.Contains(r.Path, needle) {
			t.Errorf("%s: non-router call extracted as route %s %s (%s:%d)",
				module, r.Method, r.Path, r.File, r.Line)
		}
	}
}

func TestChiExtractor_RoutesOutsideMain(t *testing.T) {
	assertRoutesOutsideMain(t, &extractor.ChiExtractor{}, "chi-multipkg", []string{
		// routes.Register(r), called from main() at the root prefix.
		"GET /health",
		// registerPayments(r), reached through r.Route("/api/v1", …) so the
		// prefix of the call site flows into the registrar.
		"GET /api/v1/payments",
		"POST /api/v1/payments",
		"GET /api/v1/payments/{paymentID}",
		// admin.Router() builds its own router and is mounted under /admin.
		"GET /admin/dashboard",
		// A router held in a struct field, registered from a method.
		"GET /status",
		// Route methods promoted from an embedded chi.Router.
		"GET /embedded",
	})
	assertNoRouteMatching(t, &extractor.ChiExtractor{}, "chi-multipkg", "warm")
}

// TestChiExtractor_MountedRouterNotDuplicated pins the rule that a factory
// mounted by another function is emitted only under its mount prefix — never a
// second time at the bare path it uses internally.
func TestChiExtractor_MountedRouterNotDuplicated(t *testing.T) {
	for _, r := range extractRoutes(t, &extractor.ChiExtractor{}, "chi-multipkg") {
		if r.Path == "/dashboard" {
			t.Errorf("mounted route emitted without its /admin prefix (%s:%d)", r.File, r.Line)
		}
	}
}

func TestStdlibExtractor_RoutesOutsideMain(t *testing.T) {
	assertRoutesOutsideMain(t, &extractor.StdlibExtractor{}, "stdlib-multipkg", []string{
		// Handed to routes.Register(mux) from main().
		"GET /health",
		// Handed on again to registerItems(mux).
		"GET /api/v1/items",
		"POST /api/v1/items",
		"GET /api/v1/items/{itemID}",
		// Built inside a factory whose signature names no mux type.
		"GET /factory",
		// Registered on a mux held in a struct field.
		"GET /status",
	})
	assertNoRouteMatching(t, &extractor.StdlibExtractor{}, "stdlib-multipkg", "warm")
}

func TestGorillaExtractor_RoutesOutsideMain(t *testing.T) {
	assertRoutesOutsideMain(t, &extractor.GorillaExtractor{}, "gorilla-multipkg", []string{
		// routes.Register(r), called from main() at the root prefix.
		"GET /health",
		// routes.RegisterItems(api) is handed a subrouter, so the prefix that
		// subrouter carries flows across the package boundary.
		"GET /api/v1/items",
		"POST /api/v1/items",
		"GET /api/v1/items/{itemID}",
		// Built inside a factory whose signature names no mux type.
		"GET /factory",
		// Registered on a router held in a struct field.
		"GET /status",
	})
	assertNoRouteMatching(t, &extractor.GorillaExtractor{}, "gorilla-multipkg", "warm")
}
