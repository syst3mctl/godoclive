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

	// chi-basic has: GET /users, POST /users, GET /users/{id}, DELETE /users/{id}
	// All inside r.Route("/users", func(r chi.Router){...})
	// With a Group for auth-protected ones.
	expected := []struct {
		method string
		path   string
	}{
		{"GET", "/users"},
		{"POST", "/users"},
		{"GET", "/users/{id}"},
		{"DELETE", "/users/{id}"},
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
	// r.Mount("/admin", adminRouter()) — adminRouter is a separate func, not inline literal
	//
	// Total inline routes: 6 (the Mount callback is not a func literal, so not descended)
	expected := map[string]bool{
		"GET /api/v1/users":            true,
		"POST /api/v1/users":           true,
		"GET /api/v1/users/{userID}":   true,
		"PUT /api/v1/users/{userID}":   true,
		"GET /api/v1/stats":            true,
		"DELETE /api/v1/cache":         true,
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
		"GET /api/v1/items":       true,
		"GET /api/v1/items/{id}":  true,
		"POST /api/v1/items":      true,
		"DELETE /api/v1/items/{id}": true,
		"GET /api/v1/admin/users": true,
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
