package auth_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/tools/go/packages"

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
			authDef, _ := auth.DetectAuth(route.Middlewares, info, pkgs)
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
			authDef, _ := auth.DetectAuth(route.Middlewares, info, pkgs)
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
			authDef, _ := auth.DetectAuth(route.Middlewares, info, pkgs)
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
			authDef, _ := auth.DetectAuth(route.Middlewares, info, pkgs)
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
			authDef, _ := auth.DetectAuth(route.Middlewares, info, pkgs)
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
			authDef, _ := auth.DetectAuth(route.Middlewares, info, pkgs)
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

// TestDetectAuth_DoesNotTraceIntoDependencies pins the boundary of the
// credential scan.
//
// The scan follows one call level below a middleware body, because the
// credential read is usually in a small helper. That descent must stay inside
// the analyzed program: net/http's own (*Request).BasicAuth reads the
// Authorization header, so following a call into it reports bearer for a route
// that uses HTTP Basic.
//
// The packages here are loaded with NeedDeps on purpose, so every dependency's
// source IS available to walk. The production load mode does not request it,
// which alone hides the bug — this test removes that accident and checks the
// boundary itself, so it keeps holding if the load mode ever changes.
func TestDetectAuth_DoesNotTraceIntoDependencies(t *testing.T) {
	cfg := &packages.Config{
		Dir: testdataDir("mixed-auth"),
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps |
			packages.NeedImports,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatalf("packages.Load: %v", err)
	}

	// Confirm the premise: dependency source really is loaded here.
	depsParsed := 0
	roots := map[string]bool{}
	for _, p := range pkgs {
		roots[p.PkgPath] = true
	}
	packages.Visit(pkgs, func(p *packages.Package) bool {
		if !roots[p.PkgPath] && len(p.Syntax) > 0 {
			depsParsed++
		}
		return true
	}, nil)
	if depsParsed == 0 {
		t.Fatal("no dependency source was loaded; this test would pass for the wrong reason")
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	for _, route := range routes {
		if route.Method != "GET" || route.Path != "/admin/stats" {
			continue
		}
		authDef, _ := auth.DetectAuth(route.Middlewares, pkgs[0].TypesInfo, pkgs)

		// BasicAuth() calls r.BasicAuth(); nothing on this route reads a
		// bearer token.
		want := []model.AuthScheme{model.AuthBasic}
		if len(authDef.Schemes) != len(want) || authDef.Schemes[0] != want[0] {
			t.Errorf("schemes = %v, want %v — a scheme was inferred from a dependency's source",
				authDef.Schemes, want)
		}
		if !authDef.Required {
			t.Error("basic auth on /admin/stats should be required")
		}
		return
	}
	t.Fatal("GET /admin/stats route not found")
}
