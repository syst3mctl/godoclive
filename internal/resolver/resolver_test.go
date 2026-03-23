package resolver_test

import (
	"go/ast"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dicki/godoclive/internal/extractor"
	"github.com/dicki/godoclive/internal/loader"
	"github.com/dicki/godoclive/internal/resolver"
)

func testdataDir(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
}

// --- Handler resolution tests ---

func TestResolveHandler_ChiBasic(t *testing.T) {
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

	if len(routes) != 6 {
		t.Fatalf("expected 6 routes, got %d", len(routes))
	}

	info := pkgs[0].TypesInfo

	expectedHandlers := map[string]string{
		"GET /users":          "ListUsers",
		"POST /users":         "CreateUser",
		"GET /users/{id}":     "GetUser",
		"DELETE /users/{id}":  "DeleteUser",
		"GET /v1/users/{id}":  "GetUserV1",
		"POST /v2/users":      "CreateUserV2",
	}

	for _, route := range routes {
		key := route.Method + " " + route.Path
		expectedName, ok := expectedHandlers[key]
		if !ok {
			t.Errorf("unexpected route: %s", key)
			continue
		}

		fd, fl, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Errorf("ResolveHandler(%s) failed: %v", key, err)
			continue
		}

		if fl != nil {
			t.Errorf("ResolveHandler(%s) returned FuncLit, expected FuncDecl", key)
			continue
		}

		if fd == nil {
			t.Errorf("ResolveHandler(%s) returned nil FuncDecl", key)
			continue
		}

		if fd.Name.Name != expectedName {
			t.Errorf("ResolveHandler(%s): got func name %q, want %q", key, fd.Name.Name, expectedName)
		}

		// Verify the resolved function has a body.
		if fd.Body == nil {
			t.Errorf("ResolveHandler(%s): resolved FuncDecl has nil body", key)
		}
	}
}

func TestResolveHandler_GinBasic(t *testing.T) {
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

	if len(routes) != 5 {
		t.Fatalf("expected 5 routes, got %d", len(routes))
	}

	info := pkgs[0].TypesInfo

	expectedHandlers := map[string]string{
		"GET /api/v1/items":          "ListItems",
		"GET /api/v1/items/{id}":     "GetItem",
		"POST /api/v1/items":         "CreateItem",
		"DELETE /api/v1/items/{id}":  "DeleteItem",
		"GET /api/v1/admin/users":    "ListUsers",
	}

	for _, route := range routes {
		key := route.Method + " " + route.Path
		expectedName, ok := expectedHandlers[key]
		if !ok {
			t.Errorf("unexpected route: %s", key)
			continue
		}

		fd, fl, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Errorf("ResolveHandler(%s) failed: %v", key, err)
			continue
		}

		if fl != nil {
			t.Errorf("ResolveHandler(%s) returned FuncLit, expected FuncDecl", key)
			continue
		}

		if fd == nil {
			t.Errorf("ResolveHandler(%s) returned nil FuncDecl", key)
			continue
		}

		if fd.Name.Name != expectedName {
			t.Errorf("ResolveHandler(%s): got func name %q, want %q", key, fd.Name.Name, expectedName)
		}
	}
}

func TestResolveHandler_InlineFuncLit(t *testing.T) {
	dir := testdataDir("chi-inline")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}

	info := pkgs[0].TypesInfo

	for _, route := range routes {
		key := route.Method + " " + route.Path

		fd, fl, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Errorf("ResolveHandler(%s) failed: %v", key, err)
			continue
		}

		switch route.Path {
		case "/health":
			// Direct function reference.
			if fd == nil {
				t.Errorf("ResolveHandler(%s): expected FuncDecl, got nil", key)
				continue
			}
			if fd.Name.Name != "HealthCheck" {
				t.Errorf("ResolveHandler(%s): got func name %q, want HealthCheck", key, fd.Name.Name)
			}
		case "/inline":
			// Inline function literal.
			if fl == nil {
				t.Errorf("ResolveHandler(%s): expected FuncLit, got nil", key)
				continue
			}
			if fl.Body == nil {
				t.Errorf("ResolveHandler(%s): FuncLit has nil body", key)
			}
			// Should not return a FuncDecl.
			if fd != nil {
				t.Errorf("ResolveHandler(%s): expected nil FuncDecl for inline, got %q", key, fd.Name.Name)
			}
		}
	}
}

// --- Parameter name resolution tests ---

func TestResolveHandlerParams_ChiBasic(t *testing.T) {
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

	info := pkgs[0].TypesInfo

	for _, route := range routes {
		key := route.Method + " " + route.Path
		fd, _, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler(%s) failed: %v", key, err)
		}
		if fd == nil {
			continue
		}

		params := resolver.ResolveHandlerParams(fd.Type, info)

		// chi-basic handlers all use (w http.ResponseWriter, r *http.Request)
		if params.Writer != "w" {
			t.Errorf("handler %s: expected Writer='w', got %q", fd.Name.Name, params.Writer)
		}
		if params.Request != "r" {
			t.Errorf("handler %s: expected Request='r', got %q", fd.Name.Name, params.Request)
		}
		if params.GinCtx != "" {
			t.Errorf("handler %s: expected GinCtx='', got %q", fd.Name.Name, params.GinCtx)
		}
	}
}

func TestResolveHandlerParams_NonStandardNames(t *testing.T) {
	dir := testdataDir("chi-inline")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info := pkgs[0].TypesInfo

	for _, route := range routes {
		key := route.Method + " " + route.Path
		fd, fl, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler(%s) failed: %v", key, err)
		}

		var fnType = getFuncType(fd, fl)
		params := resolver.ResolveHandlerParams(fnType, info)

		// Both HealthCheck and the inline handler use rw and req.
		if params.Writer != "rw" {
			t.Errorf("handler at %s: expected Writer='rw', got %q", key, params.Writer)
		}
		if params.Request != "req" {
			t.Errorf("handler at %s: expected Request='req', got %q", key, params.Request)
		}
	}
}

func TestResolveHandlerParams_GinContext(t *testing.T) {
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

	info := pkgs[0].TypesInfo

	for _, route := range routes {
		key := route.Method + " " + route.Path
		fd, _, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler(%s) failed: %v", key, err)
		}
		if fd == nil {
			continue
		}

		params := resolver.ResolveHandlerParams(fd.Type, info)

		// gin-basic handlers all use (c *gin.Context)
		if params.GinCtx != "c" {
			t.Errorf("handler %s: expected GinCtx='c', got %q", fd.Name.Name, params.GinCtx)
		}
		// gin handlers don't have separate Writer/Request params.
		if params.Writer != "" {
			t.Errorf("handler %s: expected Writer='', got %q", fd.Name.Name, params.Writer)
		}
		if params.Request != "" {
			t.Errorf("handler %s: expected Request='', got %q", fd.Name.Name, params.Request)
		}
	}
}

func TestResolveHandler_NilExpr(t *testing.T) {
	_, _, err := resolver.ResolveHandler(nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil expression, got nil")
	}
}

// getFuncType extracts the *ast.FuncType from either a FuncDecl or FuncLit.
func getFuncType(fd *ast.FuncDecl, fl *ast.FuncLit) *ast.FuncType {
	if fd != nil {
		return fd.Type
	}
	if fl != nil {
		return fl.Type
	}
	return nil
}
