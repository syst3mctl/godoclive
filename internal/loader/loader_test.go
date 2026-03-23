package loader_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dicki/godoclive/internal/loader"
)

func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "chi-basic")
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
