//go:build corpus

// The upstream corpus test is tagged so it never runs in the ordinary `go
// test ./...` — it clones a real repository and downloads that repository's
// module graph. It runs nightly (and on demand) against a pinned commit:
//
//	go test -tags corpus -run TestCorpus_UpstreamRealWorld ./internal/pipeline/
//
// Set GODOCLIVE_CORPUS_DIR to an existing checkout to skip the clone.
package pipeline_test

import (
	"os"
	"os/exec"
	"testing"
)

const (
	upstreamRepo = "https://github.com/gothinkster/golang-gin-realworld-example-app.git"
	// Pinned so a gate failure means our analysis changed, never that the
	// corpus moved underneath us. Bump deliberately, with the gate counts
	// re-derived from the new route table.
	upstreamCommit = "626c372d259472148d93303f74aa9b9a1cdcef24"
)

// TestCorpus_UpstreamRealWorld holds the analyzer to the same release gates on
// the real RealWorld backend that TestCorpus_GinRealWorld holds it to on the
// compact fixture. The fixture keeps every PR honest; this keeps the fixture
// honest.
func TestCorpus_UpstreamRealWorld(t *testing.T) {
	dir := os.Getenv("GODOCLIVE_CORPUS_DIR")
	if dir == "" {
		dir = cloneUpstream(t)
	}

	endpoints, err := pipelineRun(dir)
	if err != nil {
		t.Fatalf("RunPipeline(%s): %v", dir, err)
	}
	checkCorpusGates(t, endpoints)
}

// cloneUpstream fetches exactly the pinned commit into a temporary directory.
func cloneUpstream(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "--quiet")
	run("remote", "add", "origin", upstreamRepo)
	run("fetch", "--quiet", "--depth", "1", "origin", upstreamCommit)
	run("checkout", "--quiet", "FETCH_HEAD")

	return dir
}
