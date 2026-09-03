package loader_test

import (
	"go/types"
	"golang.org/x/tools/go/packages"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/syst3mctl/godoclive/internal/loader"
)

func testdataDir() string {
	return moduleDir("chi-basic")
}

// moduleDir locates one testdata module.
func moduleDir(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
}

func TestLoadPackages_ChiBasic(t *testing.T) {
	dir := testdataDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatalf("testdata dir does not exist: %s", dir)
	}

	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	if len(pkgs) == 0 {
		t.Fatal("expected at least one package, got none")
	}

	pkg := pkgs[0]

	// Verify package name.
	if pkg.Name != "main" {
		t.Errorf("expected package name 'main', got %q", pkg.Name)
	}

	// Verify AST is loaded.
	if len(pkg.Syntax) == 0 {
		t.Error("expected non-empty Syntax (AST), got none")
	}

	// Verify TypesInfo is populated.
	if pkg.TypesInfo == nil {
		t.Error("expected TypesInfo to be populated, got nil")
	}

	if len(pkg.TypesInfo.Types) == 0 {
		t.Error("expected TypesInfo.Types to have entries")
	}

	// Verify types package is loaded.
	if pkg.Types == nil {
		t.Error("expected Types (go/types.Package) to be populated, got nil")
	}
}

func TestLoadPackages_InvalidPattern(t *testing.T) {
	_, err := loader.LoadPackages("/nonexistent/path/that/does/not/exist", "./...")
	if err == nil {
		t.Error("expected error for invalid pattern, got nil")
	}
}

// TestLoadPackages_DoesNotParseDependencies locks in the load mode.
//
// Adding packages.NeedDeps back would have go/packages parse and type-check
// every transitive dependency from source. For a service on gin that is a few
// hundred packages and most of the wall clock, spent building syntax trees
// this analyzer never reads — dependencies are only ever consulted through
// types.Type, which export data provides. This test fails if that regresses.
func TestLoadPackages_DoesNotParseDependencies(t *testing.T) {
	pkgs, err := loader.LoadPackages(moduleDir("gin-realworld"), "./...")
	if err != nil {
		t.Fatalf("LoadPackages: %v", err)
	}

	roots := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		roots[p.PkgPath] = true
		if len(p.Syntax) == 0 {
			t.Errorf("analyzed package %s has no syntax: it cannot be walked", p.PkgPath)
		}
		if p.TypesInfo == nil {
			t.Errorf("analyzed package %s has no type information", p.PkgPath)
		}
	}

	var parsedDeps []string
	packages.Visit(pkgs, func(p *packages.Package) bool {
		if !roots[p.PkgPath] && len(p.Syntax) > 0 {
			parsedDeps = append(parsedDeps, p.PkgPath)
		}
		return true
	}, nil)

	if len(parsedDeps) > 0 {
		sort.Strings(parsedDeps)
		shown := parsedDeps
		if len(shown) > 8 {
			shown = shown[:8]
		}
		t.Errorf("%d dependencies were parsed from source; expected none. First few: %v",
			len(parsedDeps), shown)
	}
}

// TestLoadPackages_DependencyTypesStillResolve is the other half of the
// contract: export data has to give complete type information for a
// dependency's types, since the analyzer maps request and response schemas
// through them.
func TestLoadPackages_DependencyTypesStillResolve(t *testing.T) {
	pkgs, err := loader.LoadPackages(moduleDir("gin-realworld"), "./...")
	if err != nil {
		t.Fatalf("LoadPackages: %v", err)
	}

	for _, p := range pkgs {
		if p.Types == nil {
			continue
		}
		for _, imp := range p.Types.Imports() {
			if imp.Path() != "github.com/gin-gonic/gin" {
				continue
			}
			obj := imp.Scope().Lookup("Context")
			if obj == nil {
				t.Fatal("gin.Context not found in the dependency's scope")
			}
			if _, ok := obj.Type().Underlying().(*types.Struct); !ok {
				t.Fatalf("gin.Context resolved to %T, want a struct: export data is incomplete",
					obj.Type().Underlying())
			}
			return
		}
	}
	t.Skip("gin not among the analyzed packages' imports")
}
