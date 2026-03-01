package detector_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/syst3mctl/godoclive/internal/detector"
	"github.com/syst3mctl/godoclive/internal/loader"
)

func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "chi-basic")
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

func TestDetectRouter_NilPackages(t *testing.T) {
	kind := detector.DetectRouter(nil)
	if kind != detector.RouterKindUnknown {
		t.Errorf("expected RouterKindUnknown for nil input, got %q", kind)
	}
}
