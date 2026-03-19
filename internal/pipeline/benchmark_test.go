package pipeline_test

import (
	"os"
	"testing"

	"github.com/dicki/godoclive/internal/loader"
	"github.com/dicki/godoclive/internal/pipeline"
)

func BenchmarkLoadPackages_ChiBasic(b *testing.B) {
	dir := testdataDir("chi-basic")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pkgs, err := loader.LoadPackages(dir, "./...")
		if err != nil {
			b.Fatal(err)
		}
		if len(pkgs) == 0 {
			b.Fatal("no packages loaded")
		}
	}
}

func BenchmarkRunPipeline_ChiBasic(b *testing.B) {
	dir := testdataDir("chi-basic")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eps, err := pipeline.RunPipeline(dir, "./...", nil)
		if err != nil {
			b.Fatal(err)
		}
		b.ReportMetric(float64(len(eps)), "endpoints")
	}
}

func BenchmarkRunPipeline_GinBasic(b *testing.B) {
	dir := testdataDir("gin-basic")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eps, err := pipeline.RunPipeline(dir, "./...", nil)
		if err != nil {
			b.Fatal(err)
		}
		b.ReportMetric(float64(len(eps)), "endpoints")
	}
}

func BenchmarkRunPipeline_GorillaBasic(b *testing.B) {
	dir := testdataDir("gorilla-basic")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eps, err := pipeline.RunPipeline(dir, "./...", nil)
		if err != nil {
			b.Fatal(err)
		}
		b.ReportMetric(float64(len(eps)), "endpoints")
	}
}

// BenchmarkRunPipeline_LoadVsProcess isolates package loading from route processing
// by loading packages once, then measuring just the pipeline phases that follow.
func BenchmarkRunPipeline_LoadVsProcess(b *testing.B) {
	dir := testdataDir("chi-basic")

	b.Run("Load", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pkgs, err := loader.LoadPackages(dir, "./...")
			if err != nil {
				b.Fatal(err)
			}
			_ = pkgs
		}
	})

	b.Run("PipelineFromCache", func(b *testing.B) {
		// Warm up the module cache with one load, then benchmark the full pipeline.
		// The OS page cache and Go build cache mean subsequent loads are much cheaper.
		_, err := loader.LoadPackages(dir, "./...")
		if err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			eps, err := pipeline.RunPipeline(dir, "./...", nil)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportMetric(float64(len(eps)), "endpoints")
		}
	})
}

// BenchmarkRunPipeline_Gorilla_vs_Chi compares pipeline cost across router types.
func BenchmarkRunPipeline_Gorilla_vs_Chi(b *testing.B) {
	gorDir := testdataDir("gorilla-basic")
	chiDir := testdataDir("chi-basic")

	b.Run("Chi", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := pipeline.RunPipeline(chiDir, "./...", nil); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Gorilla", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := pipeline.RunPipeline(gorDir, "./...", nil); err != nil {
				// Gorilla testdata may not be present in all envs; skip gracefully.
				if os.IsNotExist(err) {
					b.Skip("gorilla-basic testdata not found")
				}
				b.Fatal(err)
			}
		}
	})
}
