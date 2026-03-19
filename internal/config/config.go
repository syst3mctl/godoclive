package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dicki/godoclive/internal/model"
	"gopkg.in/yaml.v3"
)

// Config represents the optional .godoclive.yaml configuration file.
type Config struct {
	Title           string        `yaml:"title"`
	Version         string        `yaml:"version"`
	BaseURL         string        `yaml:"base_url"`
	Theme           string        `yaml:"theme"`
	Overrides       []Override    `yaml:"overrides"`
	Exclude         []string      `yaml:"exclude"`
	Auth            AuthConfig    `yaml:"auth"`
	ResponseHelpers []string      `yaml:"response_helpers"`
	OpenAPI         OpenAPIConfig `yaml:"openapi"`
}

// OpenAPIConfig holds OpenAPI-specific settings from the config file.
type OpenAPIConfig struct {
	Description string         `yaml:"description"`
	Contact     ContactConfig  `yaml:"contact"`
	License     LicenseConfig  `yaml:"license"`
	Servers     []ServerConfig `yaml:"servers"`
}

// ContactConfig holds contact information for the OpenAPI spec.
type ContactConfig struct {
	Name  string `yaml:"name"`
	URL   string `yaml:"url"`
	Email string `yaml:"email"`
}

// LicenseConfig holds license information for the OpenAPI spec.
type LicenseConfig struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// ServerConfig holds a server entry for the OpenAPI spec.
type ServerConfig struct {
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
}

// Override allows users to supplement or override static analysis results.
type Override struct {
	Path      string             `yaml:"path"`      // "POST /users"
	Summary   string             `yaml:"summary"`   // Override inferred summary
	Tags      []string           `yaml:"tags"`      // Override inferred tags
	Responses []ResponseOverride `yaml:"responses"` // Additional responses
}

// ResponseOverride adds a response that static analysis may have missed.
type ResponseOverride struct {
	Status      int    `yaml:"status"`
	Description string `yaml:"description"`
}

// AuthConfig overrides auth detection settings.
type AuthConfig struct {
	Header string `yaml:"header"` // Default: "Authorization"
	Scheme string `yaml:"scheme"` // "bearer", "apikey", "basic"
}

// LoadConfig loads a .godoclive.yaml file from the given directory.
// Returns nil config (no error) if the file does not exist.
func LoadConfig(dir string) (*Config, error) {
	path := filepath.Join(dir, ".godoclive.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ApplyExclusions removes endpoints matching any of the given glob patterns.
// Patterns are matched against "METHOD /path" (e.g., "GET /internal/*").
func ApplyExclusions(endpoints []model.EndpointDef, patterns []string) []model.EndpointDef {
	if len(patterns) == 0 {
		return endpoints
	}

	var result []model.EndpointDef
	for _, ep := range endpoints {
		key := ep.Method + " " + ep.Path
		excluded := false
		for _, pattern := range patterns {
			if matched, _ := filepath.Match(pattern, key); matched {
				excluded = true
				break
			}
		}
		if !excluded {
			result = append(result, ep)
		}
	}
	return result
}

// ApplyOverrides merges user-defined overrides into the endpoint list.
// Overrides match on "METHOD /path" and can set Summary, Tags, and add Responses.
func ApplyOverrides(endpoints []model.EndpointDef, overrides []Override) []model.EndpointDef {
	if len(overrides) == 0 {
		return endpoints
	}

	// Build a map for quick lookup.
	overrideMap := make(map[string]*Override)
	for i := range overrides {
		overrideMap[strings.ToUpper(overrides[i].Path)] = &overrides[i]
	}

	for i := range endpoints {
		key := endpoints[i].Method + " " + endpoints[i].Path
		ov, ok := overrideMap[strings.ToUpper(key)]
		if !ok {
			continue
		}

		if ov.Summary != "" {
			endpoints[i].Summary = ov.Summary
		}
		if len(ov.Tags) > 0 {
			endpoints[i].Tags = ov.Tags
		}
		for _, r := range ov.Responses {
			endpoints[i].Responses = append(endpoints[i].Responses, model.ResponseDef{
				StatusCode:  r.Status,
				Description: r.Description,
				Source:      "override",
			})
		}
	}

	return endpoints
}
