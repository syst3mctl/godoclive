package pipeline_test

import (
	"os"
	"testing"

	"github.com/syst3mctl/godoclive/internal/loader"
	"github.com/syst3mctl/godoclive/internal/pipeline"
)

// perfDirs are the corpora worth measuring: the reduction we ship and, when
// available, the real application it was reduced from.
func perfDirs() map[string]string {
	dirs := map[string]string{
		"fixture-realworld": testdataDir("gin-realworld"),
	}
	if d := os.Getenv("GODOCLIVE_CORPUS_DIR"); d != "" {
		dirs["upstream-realworld"] = d
	}
	return dirs
}

// BenchmarkPerf_LoadVsAnalyze compares package loading with the full pipeline.
// Each iteration loads packages; neither sub-benchmark isolates route processing.
func BenchmarkPerf_LoadVsAnalyze(b *testing.B) {
	for name, dir := range perfDirs() {
		b.Run(name+"/Load", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := loader.LoadPackages(dir, "./..."); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(name+"/LoadAndAnalyze", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				eps, err := pipeline.RunPipeline(dir, "./...", nil)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(len(eps)), "endpoints")
			}
		})
	}
}
