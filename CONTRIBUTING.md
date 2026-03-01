# Contributing to GoDoc Live

## Adding a New Router Extractor

GoDoc Live uses a plugin interface for router extraction. To add support for a new router (e.g., gorilla/mux, echo, fiber):

1. **Implement the `Extractor` interface** in `internal/extractor/`:

```go
// Extractor is the interface all router extractors must implement.
type Extractor interface {
    Extract(pkgs []*packages.Package) ([]RawRoute, error)
}
```

2. **Return `[]RawRoute`** with method, path, handler expression, middleware list, and source location:

```go
type RawRoute struct {
    Method      string
    Path        string       // Normalized: {param} format
    HandlerExpr ast.Expr     // The handler function/method expression
    Middlewares []ast.Expr   // Middleware chain applied to this route
    File        string       // Source file path
    Line        int          // Line number of route registration
}
```

3. **Add detection** in `internal/detector/detector.go` — check for the router's import path.

4. **Wire it** in `internal/pipeline/pipeline.go` — add a case to the router switch.

5. **Create testdata** — add a `testdata/your-router/` directory with a compilable Go module.

## Testdata Projects

Each `testdata/` sub-directory is a real, compilable Go module:

```
testdata/
  chi-basic/          # Simple chi routes with typed structs
  chi-nested/         # r.Route + r.Group + r.Mount patterns
  chi-helpers/        # respond()/writeJSON()/sendError() patterns
  chi-inline/         # Inline FuncLit handlers, non-standard param names
  gin-basic/          # Simple gin routes with ShouldBindJSON
  gin-groups/         # r.Group with nested auth middleware
  gin-helpers/        # respondOK()/respondError() gin helpers
  multipart/          # File upload endpoints
  mixed-auth/         # Multiple auth schemes (JWT, API key, basic)
```

Each must have its own `go.mod` and compile with `go build ./...`.

Tests run the full pipeline against these projects and assert on the resulting `[]EndpointDef`.

## Running Tests

```bash
# All tests
go test ./...

# Pipeline integration tests only
go test ./internal/pipeline/ -v

# Specific testdata project
go test ./internal/pipeline/ -v -run TestPipeline_ChiBasic

# Accuracy report
go test ./internal/pipeline/ -v -run TestPipeline_AccuracyReport

# Build check
go build ./cmd/godoclive
go vet ./...
```

## Code Style

- All analysis uses `go/ast` and `go/types` — never hardcode parameter names like `r` or `w`
- Mark anything unresolvable in `EndpointDef.Unresolved` — never guess
- Helper function tracing: one level only, no deeper recursion
- Prefer accuracy over completeness: it's better to leave something as `Unresolved` than to produce incorrect output
- Keep the dependency graph tight — every new dependency is a liability for a tool that analyzes source code

## Project Structure

```
cmd/godoclive/          CLI entry point (cobra)
internal/
  model/                EndpointDef and all data types
  loader/               go/packages source loading
  detector/             Router framework detection
  extractor/            Route extraction (chi.go, gin.go)
  resolver/             Handler + param name resolution
  contract/             Path/query/header/body/response extraction
  mapper/               types.Type → TypeDef recursive mapper
  auth/                 Middleware auth pattern detection
  config/               .godoclive.yaml parsing
  pipeline/             Orchestrator: load → detect → extract → resolve → contract → map → auth → infer
  generator/            HTML doc site output + //go:embed
    ui/                 Static UI files (HTML, CSS, JS, fonts)
pkg/godoclive/          Public API
```
