package model

// EndpointDef represents the fully-analyzed contract of one HTTP endpoint.
type EndpointDef struct {
	Method      string        // GET, POST, PUT, DELETE, PATCH
	Path        string        // /users/{id}  — normalized, chi + gin → same format
	Summary     string        // Inferred: GetUserByID → "Get User By ID"
	HandlerName string        // Original: handlers.GetUserByID
	Package     string        // Full package import path
	File        string        // Source file path
	Line        int           // Line number of handler func declaration

	Auth       AuthDef       // Authentication requirements
	Request    RequestDef    // Complete inbound contract
	Responses  []ResponseDef // All possible outbound responses (200, 400, 404…)
	Tags       []string      // Inferred grouping tag (e.g. "users", "auth")
	Deprecated bool          // Detected from // Deprecated: comment
	Unresolved []string      // Explicit list of what could not be determined
}

// AuthDef describes authentication requirements for this endpoint.
type AuthDef struct {
	Required bool
	Schemes  []AuthScheme // may have multiple (e.g. bearer OR apikey)
	Source   string       // "middleware" | "inline" | "inferred"
}

// AuthScheme represents an authentication scheme type.
type AuthScheme string

const (
	AuthBearer AuthScheme = "bearer"
	AuthAPIKey AuthScheme = "apikey"
	AuthBasic  AuthScheme = "basic"
)

// RequestDef describes everything that comes INTO the endpoint.
type RequestDef struct {
	PathParams  []ParamDef // Extracted from route pattern + handler body type upgrades
	QueryParams []ParamDef // From r.URL.Query().Get / c.Query / c.DefaultQuery
	Headers     []ParamDef // From r.Header.Get / c.GetHeader — excludes Authorization
	Body        *TypeDef   // Struct from json.Decode / c.ShouldBindJSON / c.BindJSON
	ContentType string     // "application/json" | "multipart/form-data" | "application/x-www-form-urlencoded"
	IsMultipart bool       // True when r.FormFile / c.FormFile detected
}

// ParamDef is a path, query, or header parameter.
type ParamDef struct {
	Name     string  // Raw name as used in code: "page", "X-Tenant-ID"
	In       string  // "path" | "query" | "header"
	Type     string  // "string" | "integer" | "boolean" | "uuid" | "file"
	Required bool    // Path params always true; query: inferred from guard checks
	Default  *string // Non-nil when DefaultQuery / DefaultHeader used; value is the default
	Multi    bool    // True for QueryArray / r.URL.Query()["tags"] (multi-value)
	Doc      string  // From adjacent comment if present
	Example  string  // Auto-generated from name heuristics
}

// ResponseDef describes one possible response — one status code and its payload.
type ResponseDef struct {
	StatusCode  int      // 200, 201, 400, 404, 500 etc.
	Body        *TypeDef // nil for body-less responses (204, 302)
	Description string   // Inferred from status code + context
	ContentType string   // "application/json" | "text/plain" | "application/octet-stream"
	Source      string   // "explicit" | "helper" | "inferred" — how this was found
}

// TypeDef is the recursive representation of any Go type.
type TypeDef struct {
	Name      string      // "CreateUserRequest", "[]UserResponse", "map[string]interface{}"
	Package   string      // Import path of the package defining this type
	Kind      TypeKind
	Fields    []FieldDef  // Populated for KindStruct
	Elem      *TypeDef    // Populated for KindSlice (element type) and KindMap (value type)
	IsPointer bool
	Example   interface{} // Auto-generated example value
}

// TypeKind categorizes the kind of a TypeDef.
type TypeKind string

const (
	KindStruct    TypeKind = "struct"
	KindSlice     TypeKind = "slice"
	KindMap       TypeKind = "map"
	KindPrimitive TypeKind = "primitive" // string, int, bool, float64, etc.
	KindInterface TypeKind = "interface" // gin.H and similar — marked unresolvable
	KindUnknown   TypeKind = "unknown"
)

// FieldDef is one field within a struct TypeDef.
type FieldDef struct {
	Name       string      // Go field name: UserEmail
	JSONName   string      // From `json:"email"` tag; empty means field is skipped
	Type       TypeDef
	Required   bool        // From `binding:"required"` (gin) or `validate:"required"`
	Nullable   bool        // True when field type is a pointer
	OmitEmpty  bool        // From `json:",omitempty"`
	Deprecated bool        // From // Deprecated: comment on field
	Doc        string      // Field comment, if present
	Example    interface{} // Auto-generated
}
