package detector_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/syst3mctl/godoclive/internal/detector"
	"github.com/syst3mctl/godoclive/internal/loader"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
}

func testdataDir() string {
	return testdataPath("chi-basic")
}

func TestDetectRouter_ChiBasic(t *testing.T) {
	dir := testdataDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatalf("testdata dir does not exist: %s", dir)
	}

	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	kind := detector.DetectRouter(pkgs)
	if kind != detector.RouterKindChi {
		t.Errorf("expected RouterKindChi, got %q", kind)
	}
}

func TestDetectRouter_StdlibBasic(t *testing.T) {
	dir := testdataPath("stdlib-basic")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skipf("testdata dir does not exist: %s", dir)
	}

	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	kind := detector.DetectRouter(pkgs)
	if kind != detector.RouterKindStdlib {
		t.Errorf("expected RouterKindStdlib, got %q", kind)
	}
}

func TestDetectRouter_GorillaBasic(t *testing.T) {
	dir := testdataPath("gorilla-basic")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skipf("testdata dir does not exist: %s", dir)
	}

	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	kind := detector.DetectRouter(pkgs)
	if kind != detector.RouterKindGorilla {
		t.Errorf("expected RouterKindGorilla, got %q", kind)
	}
}

func TestDetectRouter_NilPackages(t *testing.T) {
	kind := detector.DetectRouter(nil)
	if kind != detector.RouterKindUnknown {
		t.Errorf("expected RouterKindUnknown for nil input, got %q", kind)
	}
}

func TestDetectRouters_Mixed(t *testing.T) {
	dir := testdataPath("mixed-routers")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skipf("testdata dir does not exist: %s", dir)
	}

	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	kinds := detector.DetectRouters(pkgs)
	want := []detector.RouterKind{
		detector.RouterKindChi,
		detector.RouterKindGin,
		detector.RouterKindStdlib,
	}
	if len(kinds) != len(want) {
		t.Fatalf("DetectRouters = %v, want %v", kinds, want)
	}
	for i, w := range want {
		if kinds[i] != w {
			t.Errorf("DetectRouters[%d] = %q, want %q", i, kinds[i], w)
		}
	}
}

// A chi, gin or gorilla service also imports net/http and calls Handle on its
// own router. Method-name matching alone reported stdlib for all of them.
func TestDetectRouters_ChiIsNotAlsoStdlib(t *testing.T) {
	dir := testdataPath("chi-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	for _, kind := range detector.DetectRouters(pkgs) {
		if kind == detector.RouterKindStdlib {
			t.Errorf("chi-basic reported as using the stdlib router: %v", detector.DetectRouters(pkgs))
		}
	}
}
