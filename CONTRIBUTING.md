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

Run `make ci` before opening a PR. It requires Go, Node.js, jq, and
golangci-lint; the JavaScript and Action regressions execute their real startup
and shell code. The JSON Schema validator is imported by tests only.

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

## The Corpus Gate

`testdata/gin-realworld` is a compact, dependency-free reduction of the
[RealWorld "Conduit" gin backend](https://github.com/gothinkster/golang-gin-realworld-example-app).
It keeps every route-registration shape that matters for static analysis —
cross-package registration helpers, a middleware chain accumulated through
`.Use()`, gin's trailing-slash semantics, the `validator.Bind → common.Bind →
c.ShouldBindWith` chain, and `gin.H` response envelopes — and drops the
database and JWT layers.

`checkCorpusGates` in `internal/pipeline/corpus_test.go` holds analysis to the
counts derived by hand from that route table: **27 routes, 16 requiring
authentication, 10 request bodies, no OpenAPI collisions, nothing unresolved**.
Generated OpenAPI operation IDs must also be unique.
It runs on every PR as part of `go test ./...`.

The same gates run against the real upstream repository at a pinned commit,
nightly and on demand, so a regression the reduction happens to miss still
surfaces within a day:

```bash
# Clones the pinned commit into a temp dir
go test -tags corpus -run TestCorpus_UpstreamRealWorld ./internal/pipeline/

# Or point it at a checkout you already have
GODOCLIVE_CORPUS_DIR=/path/to/checkout go test -tags corpus \
  -run TestCorpus_UpstreamRealWorld ./internal/pipeline/
```

`testdata/gin-unresolved` is the inverse fixture: every endpoint in it has
something the analyzer cannot establish, and `TestCorpus_UnresolvedShapesReduceCoverage`
asserts coverage reports 0%. When you add a capability, add its shape to
`gin-realworld`; when you find one that cannot be resolved, add it there.

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
