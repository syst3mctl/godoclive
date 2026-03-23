package generator_test

import (
	"os"
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

func BenchmarkGenerate_Folder_ChiBasic(b *testing.B) {
	eps := loadEndpoints(b, "chi-basic")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		outDir := b.TempDir()
		cfg := generator.GeneratorConfig{
			OutputPath: outDir,
			Format:     "folder",
			Title:      "Bench",
			Theme:      "light",
		}
		if err := generator.Generate(eps, cfg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerate_Single_ChiBasic(b *testing.B) {
	eps := loadEndpoints(b, "chi-basic")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		outDir := b.TempDir()
		cfg := generator.GeneratorConfig{
			OutputPath: filepath.Join(outDir, "docs.html"),
			Format:     "single",
			Title:      "Bench",
			Theme:      "light",
		}
		if err := generator.Generate(eps, cfg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerate_Folder_GinBasic(b *testing.B) {
	eps := loadEndpoints(b, "gin-basic")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		outDir := b.TempDir()
		cfg := generator.GeneratorConfig{
			OutputPath: outDir,
			Format:     "folder",
			Title:      "Bench",
			Theme:      "light",
		}
		if err := generator.Generate(eps, cfg); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGenerate_Folder_vs_Single compares folder vs single-file output cost,
// measuring the base64 encoding overhead of the single format.
func BenchmarkGenerate_Folder_vs_Single(b *testing.B) {
	eps := loadEndpoints(b, "chi-basic")

	b.Run("Folder", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			outDir, err := os.MkdirTemp("", "bench-folder-*")
			if err != nil {
				b.Fatal(err)
			}
			cfg := generator.GeneratorConfig{
				OutputPath: outDir,
				Format:     "folder",
				Title:      "Bench",
				Theme:      "light",
			}
			genErr := generator.Generate(eps, cfg)
			_ = os.RemoveAll(outDir)
			if genErr != nil {
				b.Fatal(genErr)
			}
		}
	})

	b.Run("Single", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			outDir, err := os.MkdirTemp("", "bench-single-*")
			if err != nil {
				b.Fatal(err)
			}
			cfg := generator.GeneratorConfig{
				OutputPath: filepath.Join(outDir, "docs.html"),
				Format:     "single",
				Title:      "Bench",
				Theme:      "light",
			}
			genErr := generator.Generate(eps, cfg)
			_ = os.RemoveAll(outDir)
			if genErr != nil {
				b.Fatal(genErr)
			}
		}
	})
}
