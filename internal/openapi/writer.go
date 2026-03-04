package openapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// OutputFormat specifies the serialization format.
type OutputFormat string

const (
	FormatJSON OutputFormat = "json"
	FormatYAML OutputFormat = "yaml"
)

// WriteConfig configures how the document is written.
type WriteConfig struct {
	OutputPath string
	Format     OutputFormat
	Indent     bool
}

// Write serializes the document and writes it to a file. If Format is empty,
// it is inferred from the file extension.
func Write(doc *Document, cfg WriteConfig) error {
	format := cfg.Format
	if format == "" {
		format = inferFormat(cfg.OutputPath)
	}

	var data []byte
	var err error

	switch format {
	case FormatYAML:
		data, err = MarshalYAML(doc)
	default:
		if cfg.Indent {
			data, err = json.MarshalIndent(doc, "", "  ")
		} else {
			data, err = Marshal(doc)
		}
	}
	if err != nil {
		return fmt.Errorf("marshaling OpenAPI spec: %w", err)
	}

	// Ensure parent directory exists.
	dir := filepath.Dir(cfg.OutputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
	}

	if err := os.WriteFile(cfg.OutputPath, data, 0o644); err != nil {
		return fmt.Errorf("writing OpenAPI spec: %w", err)
	}

	return nil
}

// Marshal serializes the document to indented JSON bytes.
func Marshal(doc *Document) ([]byte, error) {
	return json.MarshalIndent(doc, "", "  ")
}

// MarshalYAML serializes the document to YAML bytes.
func MarshalYAML(doc *Document) ([]byte, error) {
	return yaml.Marshal(doc)
}

// inferFormat determines the output format from the file extension.
func inferFormat(path string) OutputFormat {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return FormatYAML
	default:
		return FormatJSON
	}
}
