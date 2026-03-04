// Package godoclive provides a public API for analyzing Go HTTP services
// and generating interactive API documentation.
package godoclive

import (
	"github.com/syst3mctl/godoclive/internal/config"
	"github.com/syst3mctl/godoclive/internal/generator"
	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/openapi"
	"github.com/syst3mctl/godoclive/internal/pipeline"
)

// EndpointDef is the public type alias for the analyzed endpoint contract.
type EndpointDef = model.EndpointDef

// Options configures the behavior of Analyze and Generate.
type Options struct {
	Config        *config.Config
	OutputPath    string
	Format        string // "folder" or "single"
	Title         string
	Version       string
	BaseURL       string
	Theme         string // "light" or "dark"
	OpenAPIOutput string // path for OpenAPI spec output
}

// Option is a functional option for configuring Analyze and Generate.
type Option func(*Options)

// WithConfig sets the project configuration.
func WithConfig(cfg *config.Config) Option {
	return func(o *Options) { o.Config = cfg }
}

// WithOutput sets the output directory path.
func WithOutput(path string) Option {
	return func(o *Options) { o.OutputPath = path }
}

// WithFormat sets the output format ("folder" or "single").
func WithFormat(format string) Option {
	return func(o *Options) { o.Format = format }
}

// WithTitle sets the project title displayed in the documentation.
func WithTitle(title string) Option {
	return func(o *Options) { o.Title = title }
}

// WithVersion sets the project version displayed in the documentation.
func WithVersion(version string) Option {
	return func(o *Options) { o.Version = version }
}

// WithBaseURL sets the pre-filled base URL in Try It.
func WithBaseURL(url string) Option {
	return func(o *Options) { o.BaseURL = url }
}

// WithTheme sets the default theme ("light" or "dark").
func WithTheme(theme string) Option {
	return func(o *Options) { o.Theme = theme }
}

// WithOpenAPIOutput sets the output path for an OpenAPI spec file.
func WithOpenAPIOutput(path string) Option {
	return func(o *Options) { o.OpenAPIOutput = path }
}

// Analyze runs the analysis pipeline on the given Go packages and returns
// the extracted endpoint contracts. The pattern should be a directory path
// with an optional package pattern suffix (e.g. "./..." or "./cmd/...").
func Analyze(dir, pattern string, opts ...Option) ([]EndpointDef, error) {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}

	cfg := o.Config
	if cfg == nil {
		loaded, err := config.LoadConfig(dir)
		if err != nil {
			return nil, err
		}
		cfg = loaded
	}

	return pipeline.RunPipeline(dir, pattern, cfg)
}

// Generate creates documentation output from analyzed endpoints.
func Generate(endpoints []EndpointDef, opts ...Option) error {
	o := &Options{
		OutputPath: "./docs",
		Format:     "folder",
		Theme:      "light",
	}
	for _, opt := range opts {
		opt(o)
	}

	if err := generator.Generate(endpoints, generator.GeneratorConfig{
		OutputPath: o.OutputPath,
		Format:     o.Format,
		Title:      o.Title,
		Version:    o.Version,
		BaseURL:    o.BaseURL,
		Theme:      o.Theme,
	}); err != nil {
		return err
	}

	// Also write OpenAPI spec if configured.
	if o.OpenAPIOutput != "" {
		doc := openapi.Generate(endpoints, openapi.Config{
			Title:   o.Title,
			Version: o.Version,
		})
		if err := openapi.Write(doc, openapi.WriteConfig{
			OutputPath: o.OpenAPIOutput,
			Indent:     true,
		}); err != nil {
			return err
		}
	}

	return nil
}

// GenerateOpenAPI generates an OpenAPI 3.1.0 spec from analyzed endpoints and
// returns the JSON bytes. Options Title and Version are used for the spec info.
func GenerateOpenAPI(endpoints []EndpointDef, opts ...Option) ([]byte, error) {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}

	doc := openapi.Generate(endpoints, openapi.Config{
		Title:   o.Title,
		Version: o.Version,
	})

	return openapi.Marshal(doc)
}
