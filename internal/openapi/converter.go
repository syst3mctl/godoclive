package openapi

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dicki/godoclive/internal/model"
)

// Config holds settings for OpenAPI document generation.
type Config struct {
	Title       string
	Description string
	Version     string
	Servers     []Server
	Contact     *Contact
	License     *License
}

// converter holds state during the conversion process.
type converter struct {
	cfg     Config
	schemas map[string]*Schema
	// track in-progress schemas to break cycles
	inProgress map[string]bool
	// track name collisions: short name → package path
	namePackages map[string]string
}

// Generate converts analyzed endpoints into an OpenAPI 3.1.0 document.
func Generate(endpoints []model.EndpointDef, cfg Config) *Document {
	c := &converter{
		cfg:          cfg,
		schemas:      make(map[string]*Schema),
		inProgress:   make(map[string]bool),
		namePackages: make(map[string]string),
	}

	doc := &Document{
		OpenAPI: "3.1.0",
		Info: Info{
			Title:       coalesce(cfg.Title, "API Documentation"),
			Description: cfg.Description,
			Version:     coalesce(cfg.Version, "0.1.0"),
			Contact:     cfg.Contact,
			License:     cfg.License,
		},
		Servers: cfg.Servers,
		Paths:   make(map[string]*PathItem),
	}

	// Collect unique tags and security schemes.
	tagSet := make(map[string]bool)
	secSchemes := make(map[string]*SecurityScheme)

	for _, ep := range endpoints {
		path := ep.Path
		if doc.Paths[path] == nil {
			doc.Paths[path] = &PathItem{}
		}

		op := c.convertEndpoint(ep)

		for _, t := range ep.Tags {
			tagSet[t] = true
		}

		// Handle auth → security schemes.
		if ep.Auth.Required && len(ep.Auth.Schemes) > 0 {
			for _, scheme := range ep.Auth.Schemes {
				name, secScheme := authSchemeToSecurity(scheme)
				secSchemes[name] = secScheme
			}
			var secReqs []SecurityRequirement
			for _, scheme := range ep.Auth.Schemes {
				name, _ := authSchemeToSecurity(scheme)
				secReqs = append(secReqs, SecurityRequirement{name: {}})
			}
			op.Security = secReqs
		}

		// Expand ANY method to all standard HTTP methods.
		if ep.Method == "ANY" {
			for _, m := range []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"} {
				opCopy := *op
				opCopy.OperationID = toOperationID(m, ep.Path)
				doc.Paths[path].SetOperation(m, &opCopy)
			}
		} else {
			doc.Paths[path].SetOperation(ep.Method, op)
		}
	}

	// Build sorted tags.
	var tags []Tag
	for t := range tagSet {
		tags = append(tags, Tag{Name: t})
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Name < tags[j].Name })
	doc.Tags = tags

	// Build components.
	if len(c.schemas) > 0 || len(secSchemes) > 0 {
		doc.Components = &Components{}
		if len(c.schemas) > 0 {
			doc.Components.Schemas = c.schemas
		}
		if len(secSchemes) > 0 {
			doc.Components.SecuritySchemes = secSchemes
		}
	}

	return doc
}

// convertEndpoint converts a single EndpointDef to an Operation.
func (c *converter) convertEndpoint(ep model.EndpointDef) *Operation {
	op := &Operation{
		OperationID: toOperationID(ep.Method, ep.Path),
		Summary:     ep.Summary,
		Tags:        ep.Tags,
		Deprecated:  ep.Deprecated,
		Responses:   make(map[string]*Response),
	}

	// Parameters: path + query + headers.
	for _, p := range ep.Request.PathParams {
		op.Parameters = append(op.Parameters, convertParam(p))
	}
	for _, p := range ep.Request.QueryParams {
		op.Parameters = append(op.Parameters, convertParam(p))
	}
	for _, p := range ep.Request.Headers {
		op.Parameters = append(op.Parameters, convertParam(p))
	}

	// Request body.
	if ep.Request.Body != nil {
		op.RequestBody = c.convertRequestBody(ep.Request)
	}

	// Responses.
	if len(ep.Responses) == 0 {
		op.Responses["200"] = &Response{Description: "Successful response"}
	} else {
		for _, r := range ep.Responses {
			resp := c.convertResponse(r)
			code := fmt.Sprintf("%d", r.StatusCode)
			op.Responses[code] = resp
		}
	}

	return op
}

// convertParam converts a ParamDef to an OpenAPI Parameter.
func convertParam(p model.ParamDef) Parameter {
	param := Parameter{
		Name:        p.Name,
		In:          p.In,
		Required:    p.Required,
		Description: p.Doc,
		Schema:      primitiveNameToSchema(p.Type, p.Multi),
		Example:     p.Example,
	}
	return param
}

// convertRequestBody converts request info to an OpenAPI RequestBody.
func (c *converter) convertRequestBody(req model.RequestDef) *RequestBody {
	contentType := req.ContentType
	if contentType == "" {
		contentType = "application/json"
	}

	schema := c.typeDefToSchema(req.Body)

	return &RequestBody{
		Required: true,
		Content: map[string]*MediaType{
			contentType: {Schema: schema},
		},
	}
}

// convertResponse converts a ResponseDef to an OpenAPI Response.
func (c *converter) convertResponse(r model.ResponseDef) *Response {
	resp := &Response{
		Description: coalesce(r.Description, defaultStatusDescription(r.StatusCode)),
	}

	if r.Body != nil {
		ct := r.ContentType
		if ct == "" {
			ct = "application/json"
		}
		schema := c.typeDefToSchema(r.Body)
		resp.Content = map[string]*MediaType{
			ct: {Schema: schema},
		}
	}

	return resp
}

// typeDefToSchema converts a TypeDef to an OpenAPI Schema, registering named
// structs in components/schemas and returning $ref pointers.
func (c *converter) typeDefToSchema(td *model.TypeDef) *Schema {
	if td == nil {
		return nil
	}

	switch td.Kind {
	case model.KindStruct:
		return c.structToSchema(td)
	case model.KindSlice:
		return &Schema{
			Type:  "array",
			Items: c.typeDefToSchema(td.Elem),
		}
	case model.KindMap:
		s := &Schema{
			Type: "object",
		}
		if td.Elem != nil {
			s.AdditionalProperties = c.typeDefToSchema(td.Elem)
		}
		return s
	case model.KindPrimitive:
		return primitiveTypeToSchema(td.Name)
	case model.KindInterface:
		return &Schema{Type: "object"}
	default:
		return &Schema{Type: "object"}
	}
}

// structToSchema handles struct types — named structs are registered in
// components/schemas and returned as $ref. Anonymous structs are inlined.
func (c *converter) structToSchema(td *model.TypeDef) *Schema {
	if td.Name == "" || len(td.Fields) == 0 {
		return &Schema{Type: "object"}
	}

	name := c.sanitizeSchemaName(td.Name, td.Package)

	// If already registered, return $ref.
	if _, exists := c.schemas[name]; exists {
		return &Schema{Ref: "#/components/schemas/" + name}
	}

	// Check for cycle: if in progress, return $ref (placeholder will be filled).
	if c.inProgress[name] {
		return &Schema{Ref: "#/components/schemas/" + name}
	}

	// Register placeholder to break cycles.
	c.inProgress[name] = true
	schema := &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
	}
	c.schemas[name] = schema

	c.populateStructFields(schema, td)

	delete(c.inProgress, name)
	return &Schema{Ref: "#/components/schemas/" + name}
}

// populateStructFields fills in properties, required, etc. for a struct schema.
func (c *converter) populateStructFields(schema *Schema, td *model.TypeDef) {
	for _, f := range td.Fields {
		jsonName := f.JSONName
		if jsonName == "" || jsonName == "-" {
			continue
		}

		fieldSchema := c.typeDefToSchema(&f.Type)
		if fieldSchema == nil {
			fieldSchema = &Schema{Type: "object"}
		}

		if f.Nullable {
			fieldSchema.Nullable = true
		}
		if f.Deprecated {
			fieldSchema.Deprecated = true
		}
		if f.Doc != "" {
			fieldSchema.Description = f.Doc
		}
		if f.Example != nil {
			fieldSchema.Example = f.Example
		}

		schema.Properties[jsonName] = fieldSchema

		if f.Required {
			schema.Required = append(schema.Required, jsonName)
		}
	}
}

// sanitizeSchemaName produces a valid, collision-free schema name.
func (c *converter) sanitizeSchemaName(name, pkg string) string {
	// Remove pointer prefix.
	name = strings.TrimPrefix(name, "*")
	// Remove package prefix if already in name.
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	// Remove []/* prefixes.
	name = strings.TrimPrefix(name, "[]")
	name = strings.TrimPrefix(name, "*")

	// Check for collision with different package.
	if existingPkg, exists := c.namePackages[name]; exists && existingPkg != pkg {
		// Append package suffix to disambiguate.
		parts := strings.Split(pkg, "/")
		if len(parts) > 0 {
			suffix := titleCase(parts[len(parts)-1])
			name = name + suffix
		}
	}
	c.namePackages[name] = pkg

	return name
}

// authSchemeToSecurity converts a model.AuthScheme to an OpenAPI SecurityScheme.
func authSchemeToSecurity(scheme model.AuthScheme) (string, *SecurityScheme) {
	switch scheme {
	case model.AuthBearer:
		return "bearerAuth", &SecurityScheme{
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		}
	case model.AuthAPIKey:
		return "apiKeyAuth", &SecurityScheme{
			Type: "apiKey",
			In:   "header",
			Name: "X-API-Key",
		}
	case model.AuthBasic:
		return "basicAuth", &SecurityScheme{
			Type:   "http",
			Scheme: "basic",
		}
	default:
		return string(scheme), &SecurityScheme{
			Type:   "http",
			Scheme: string(scheme),
		}
	}
}

// toOperationID generates a camelCase operation ID from method and path.
func toOperationID(method, path string) string {
	method = strings.ToLower(method)

	// Replace path params {id} with _id, strip slashes.
	path = strings.ReplaceAll(path, "{", "_")
	path = strings.ReplaceAll(path, "}", "")

	parts := strings.Split(path, "/")
	var sb strings.Builder
	sb.WriteString(method)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Remove leading underscores for path params, keep the name.
		p = strings.TrimPrefix(p, "_")
		sb.WriteString(titleCase(p))
	}
	return sb.String()
}

// primitiveTypeToSchema maps a Go type name to an OpenAPI schema.
func primitiveTypeToSchema(name string) *Schema {
	switch strings.ToLower(name) {
	case "string":
		return &Schema{Type: "string"}
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64":
		return &Schema{Type: "integer"}
	case "float32", "float64":
		return &Schema{Type: "number"}
	case "bool":
		return &Schema{Type: "boolean"}
	case "time.time":
		return &Schema{Type: "string", Format: "date-time"}
	case "uuid", "uuid.uuid":
		return &Schema{Type: "string", Format: "uuid"}
	default:
		return &Schema{Type: "string"}
	}
}

// primitiveNameToSchema maps a param type string to an OpenAPI schema.
func primitiveNameToSchema(typeName string, multi bool) *Schema {
	var base *Schema
	switch strings.ToLower(typeName) {
	case "integer", "int":
		base = &Schema{Type: "integer"}
	case "boolean", "bool":
		base = &Schema{Type: "boolean"}
	case "number", "float", "float64":
		base = &Schema{Type: "number"}
	case "uuid":
		base = &Schema{Type: "string", Format: "uuid"}
	case "file":
		base = &Schema{Type: "string", Format: "binary"}
	default:
		base = &Schema{Type: "string"}
	}

	if multi {
		return &Schema{
			Type:  "array",
			Items: base,
		}
	}
	return base
}

// defaultStatusDescription returns a description for common HTTP status codes.
func defaultStatusDescription(code int) string {
	switch code {
	case 200:
		return "Successful response"
	case 201:
		return "Created"
	case 204:
		return "No content"
	case 400:
		return "Bad request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not found"
	case 405:
		return "Method not allowed"
	case 409:
		return "Conflict"
	case 422:
		return "Unprocessable entity"
	case 500:
		return "Internal server error"
	default:
		return fmt.Sprintf("Response %d", code)
	}
}

// titleCase capitalizes the first letter of a string.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// coalesce returns the first non-empty string.
func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
