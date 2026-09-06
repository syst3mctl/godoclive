package generator_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/syst3mctl/godoclive/internal/generator"
	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/pipeline"
)

func testdataDir(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", name)
}

func loadEndpoints(b *testing.B, project string) []model.EndpointDef {
	b.Helper()
	eps, err := pipeline.RunPipeline(testdataDir(project), "./...", nil)
	if err != nil {
		b.Fatalf("pipeline(%s): %v", project, err)
	}
	return eps
}

// benchmarkGenerate measures repeated generation into an existing output directory,
// as in watch mode. Fixture loading, temporary directory setup and cleanup are excluded.
func benchmarkGenerate(b *testing.B, eps []model.EndpointDef, format string) {
	b.Helper()
	cfg := generator.GeneratorConfig{OutputPath: b.TempDir(), Format: format, Title: "Bench", Theme: "light"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := generator.Generate(eps, cfg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerate_Folder_ChiBasic(b *testing.B) {
	benchmarkGenerate(b, loadEndpoints(b, "chi-basic"), "folder")
}

func BenchmarkGenerate_Single_ChiBasic(b *testing.B) {
	benchmarkGenerate(b, loadEndpoints(b, "chi-basic"), "single")
}

func BenchmarkGenerate_Folder_GinBasic(b *testing.B) {
	benchmarkGenerate(b, loadEndpoints(b, "gin-basic"), "folder")
}

func BenchmarkGenerate_Folder_vs_Single(b *testing.B) {
	eps := loadEndpoints(b, "chi-basic")
	b.Run("Folder", func(b *testing.B) { benchmarkGenerate(b, eps, "folder") })
	b.Run("Single", func(b *testing.B) { benchmarkGenerate(b, eps, "single") })
}
