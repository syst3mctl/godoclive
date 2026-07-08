package auth_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/syst3mctl/godoclive/internal/auth"
	"github.com/syst3mctl/godoclive/internal/extractor"
	"github.com/syst3mctl/godoclive/internal/loader"
	"github.com/syst3mctl/godoclive/internal/model"
)

func testdataDir(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", name)
}

func TestDetectAuth_JWTBearer(t *testing.T) {
	dir := testdataDir("mixed-auth")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	info := pkgs[0].TypesInfo

	// Find the GET /users route — should have JWT auth.
	for _, route := range routes {
		if route.Method == "GET" && route.Path == "/users" {
			authDef := auth.DetectAuth(route.Middlewares, info, pkgs)
			if !authDef.Required {
				t.Error("GET /users should require auth")
			}
			if len(authDef.Schemes) == 0 {
				t.Fatal("expected at least one auth scheme")
			}
			if authDef.Schemes[0] != model.AuthBearer {
				t.Errorf("expected bearer, got %s", authDef.Schemes[0])
			}
			if authDef.Source != "middleware" {
				t.Errorf("expected source 'middleware', got %q", authDef.Source)
			}
			return
		}
	}
	t.Fatal("GET /users route not found")
}

func TestDetectAuth_APIKey(t *testing.T) {
	dir := testdataDir("mixed-auth")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	info := pkgs[0].TypesInfo

	// Find GET /webhooks — should have API key auth.
	for _, route := range routes {
		if route.Method == "GET" && route.Path == "/webhooks" {
			authDef := auth.DetectAuth(route.Middlewares, info, pkgs)
			if !authDef.Required {
				t.Error("GET /webhooks should require auth")
			}
			if len(authDef.Schemes) == 0 {
				t.Fatal("expected at least one auth scheme")
			}
			found := false
			for _, s := range authDef.Schemes {
				if s == model.AuthAPIKey {
					found = true
				}
			}
			if !found {
				t.Errorf("expected apikey scheme, got %v", authDef.Schemes)
			}
			return
		}
	}
	t.Fatal("GET /webhooks route not found")
}

func TestDetectAuth_Basic(t *testing.T) {
	dir := testdataDir("mixed-auth")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	info := pkgs[0].TypesInfo

	// Find GET /admin/stats — should have basic auth.
	for _, route := range routes {
		if route.Method == "GET" && route.Path == "/admin/stats" {
			authDef := auth.DetectAuth(route.Middlewares, info, pkgs)
			if !authDef.Required {
				t.Error("GET /admin/stats should require auth")
			}
			found := false
			for _, s := range authDef.Schemes {
				if s == model.AuthBasic {
					found = true
				}
			}
			if !found {
				t.Errorf("expected basic scheme, got %v", authDef.Schemes)
			}
			return
		}
	}
	t.Fatal("GET /admin/stats route not found")
}

func TestDetectAuth_Public(t *testing.T) {
	dir := testdataDir("mixed-auth")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	info := pkgs[0].TypesInfo

	// Find GET /health — should have no auth.
	for _, route := range routes {
		if route.Method == "GET" && route.Path == "/health" {
			authDef := auth.DetectAuth(route.Middlewares, info, pkgs)
			if authDef.Required {
				t.Error("GET /health should not require auth")
			}
			if len(authDef.Schemes) != 0 {
				t.Errorf("expected no schemes, got %v", authDef.Schemes)
			}
			return
		}
	}
	t.Fatal("GET /health route not found")
}

// TestDetectAuth_FactoryVarMiddleware covers the production-edge idiom where
// the middleware factory's result is held in a LOCAL VARIABLE and the route's
// middleware expression is that variable's ident:
//
//	requireAuth := RequireAuth(logger)
//	mux.Handle("GET /items", requireAuth(http.HandlerFunc(handleItems)))
//
// Detection must trace the var to its initializer (the factory call) and scan
// the factory body — which checks the Authorization header → bearer.
func TestDetectAuth_FactoryVarMiddleware(t *testing.T) {
	dir := testdataDir("stdlib-authvar")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages: %v", err)
	}

	ext := &extractor.StdlibExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	info := pkgs[0].TypesInfo

	var sawItems, sawPublic bool
	for _, route := range routes {
		switch {
		case route.Method == "GET" && route.Path == "/items":
			sawItems = true
			authDef := auth.DetectAuth(route.Middlewares, info, pkgs)
			if !authDef.Required {
				t.Error("GET /items should require auth (factory-var middleware)")
			}
			var bearer bool
			for _, s := range authDef.Schemes {
				if s == model.AuthBearer {
					bearer = true
				}
			}
			if !bearer {
				t.Errorf("expected bearer scheme, got %v", authDef.Schemes)
			}
		case route.Method == "GET" && route.Path == "/public":
			sawPublic = true
			authDef := auth.DetectAuth(route.Middlewares, info, pkgs)
			if authDef.Required {
				t.Error("GET /public should not require auth")
			}
		}
	}
	if !sawItems {
		t.Fatal("GET /items route not found")
	}
	if !sawPublic {
		t.Fatal("GET /public route not found")
	}
}
