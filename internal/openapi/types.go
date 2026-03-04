// Package openapi provides OpenAPI 3.1.0 spec generation from analyzed endpoints.
package openapi

// Document is the root OpenAPI 3.1.0 document.
type Document struct {
	OpenAPI    string                `json:"openapi" yaml:"openapi"`
	Info       Info                  `json:"info" yaml:"info"`
	Servers    []Server              `json:"servers,omitempty" yaml:"servers,omitempty"`
	Paths      map[string]*PathItem  `json:"paths" yaml:"paths"`
	Components *Components           `json:"components,omitempty" yaml:"components,omitempty"`
	Tags       []Tag                 `json:"tags,omitempty" yaml:"tags,omitempty"`
	Security   []SecurityRequirement `json:"security,omitempty" yaml:"security,omitempty"`
}

// Info provides metadata about the API.
type Info struct {
	Title       string   `json:"title" yaml:"title"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string   `json:"version" yaml:"version"`
	Contact     *Contact `json:"contact,omitempty" yaml:"contact,omitempty"`
	License     *License `json:"license,omitempty" yaml:"license,omitempty"`
}

// Contact information for the API.
type Contact struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	URL   string `json:"url,omitempty" yaml:"url,omitempty"`
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
}

// License information for the API.
type License struct {
	Name string `json:"name" yaml:"name"`
	URL  string `json:"url,omitempty" yaml:"url,omitempty"`
}

// Server represents an API server.
type Server struct {
	URL         string `json:"url" yaml:"url"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// Tag adds metadata to a single tag.
type Tag struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// PathItem describes the operations available on a single path.
type PathItem struct {
	Get     *Operation `json:"get,omitempty" yaml:"get,omitempty"`
	Post    *Operation `json:"post,omitempty" yaml:"post,omitempty"`
	Put     *Operation `json:"put,omitempty" yaml:"put,omitempty"`
	Delete  *Operation `json:"delete,omitempty" yaml:"delete,omitempty"`
	Patch   *Operation `json:"patch,omitempty" yaml:"patch,omitempty"`
	Head    *Operation `json:"head,omitempty" yaml:"head,omitempty"`
	Options *Operation `json:"options,omitempty" yaml:"options,omitempty"`
	Trace   *Operation `json:"trace,omitempty" yaml:"trace,omitempty"`
}

// SetOperation sets the operation for the given HTTP method.
func (p *PathItem) SetOperation(method string, op *Operation) {
	switch method {
	case "GET":
		p.Get = op
	case "POST":
		p.Post = op
	case "PUT":
		p.Put = op
	case "DELETE":
		p.Delete = op
	case "PATCH":
		p.Patch = op
	case "HEAD":
		p.Head = op
	case "OPTIONS":
		p.Options = op
	case "TRACE":
		p.Trace = op
	}
}

// Operation describes a single API operation on a path.
type Operation struct {
	OperationID string                `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Summary     string                `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string                `json:"description,omitempty" yaml:"description,omitempty"`
	Tags        []string              `json:"tags,omitempty" yaml:"tags,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
	Responses   map[string]*Response  `json:"responses" yaml:"responses"`
	Security    []SecurityRequirement `json:"security,omitempty" yaml:"security,omitempty"`
	Deprecated  bool                  `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
}

// Parameter describes a single operation parameter.
type Parameter struct {
	Name        string  `json:"name" yaml:"name"`
	In          string  `json:"in" yaml:"in"`
	Description string  `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool    `json:"required,omitempty" yaml:"required,omitempty"`
	Schema      *Schema `json:"schema,omitempty" yaml:"schema,omitempty"`
	Example     string  `json:"example,omitempty" yaml:"example,omitempty"`
}

// RequestBody describes a single request body.
type RequestBody struct {
	Description string               `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool                  `json:"required,omitempty" yaml:"required,omitempty"`
	Content     map[string]*MediaType `json:"content" yaml:"content"`
}

// MediaType describes a media type with optional schema.
type MediaType struct {
	Schema *Schema `json:"schema,omitempty" yaml:"schema,omitempty"`
}

// Response describes a single response from an API operation.
type Response struct {
	Description string                `json:"description" yaml:"description"`
	Content     map[string]*MediaType `json:"content,omitempty" yaml:"content,omitempty"`
	Headers     map[string]*Header    `json:"headers,omitempty" yaml:"headers,omitempty"`
}

// Header describes a single header.
type Header struct {
	Description string  `json:"description,omitempty" yaml:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty" yaml:"schema,omitempty"`
}

// Schema represents the OpenAPI Schema Object (subset of JSON Schema).
type Schema struct {
	Ref                  string             `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Type                 string             `json:"type,omitempty" yaml:"type,omitempty"`
	Format               string             `json:"format,omitempty" yaml:"format,omitempty"`
	Description          string             `json:"description,omitempty" yaml:"description,omitempty"`
	Properties           map[string]*Schema  `json:"properties,omitempty" yaml:"properties,omitempty"`
	Required             []string            `json:"required,omitempty" yaml:"required,omitempty"`
	Items                *Schema             `json:"items,omitempty" yaml:"items,omitempty"`
	AdditionalProperties *Schema             `json:"additionalProperties,omitempty" yaml:"additionalProperties,omitempty"`
	Nullable             bool                `json:"nullable,omitempty" yaml:"nullable,omitempty"`
	Deprecated           bool                `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
	Example              interface{}         `json:"example,omitempty" yaml:"example,omitempty"`
	Enum                 []string            `json:"enum,omitempty" yaml:"enum,omitempty"`
}

// SecurityRequirement maps security scheme names to required scopes.
type SecurityRequirement map[string][]string

// SecurityScheme describes an authentication mechanism.
type SecurityScheme struct {
	Type         string `json:"type" yaml:"type"`
	Scheme       string `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty" yaml:"bearerFormat,omitempty"`
	Name         string `json:"name,omitempty" yaml:"name,omitempty"`
	In           string `json:"in,omitempty" yaml:"in,omitempty"`
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
}

// Components holds reusable objects for the spec.
type Components struct {
	Schemas         map[string]*Schema         `json:"schemas,omitempty" yaml:"schemas,omitempty"`
	SecuritySchemes map[string]*SecurityScheme `json:"securitySchemes,omitempty" yaml:"securitySchemes,omitempty"`
}
