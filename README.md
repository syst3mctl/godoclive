<p align="center">
  <h1 align="center">GoDoc Live</h1>
  <p align="center">
    <strong>API documentation that writes itself.</strong><br>
    Point it at your Go code. Get interactive docs. No annotations required.
  </p>
  <p align="center">
    <a href="https://github.com/syst3mctl/godoclive/actions"><img src="https://img.shields.io/github/actions/workflow/status/syst3mctl/godoclive/ci.yml?branch=main&style=flat-square&logo=github&label=CI" alt="CI"></a>
    <a href="https://goreportcard.com/report/github.com/syst3mctl/godoclive"><img src="https://goreportcard.com/badge/github.com/syst3mctl/godoclive?style=flat-square" alt="Go Report Card"></a>
    <a href="https://pkg.go.dev/github.com/syst3mctl/godoclive"><img src="https://img.shields.io/badge/go.dev-reference-007d9c?style=flat-square&logo=go&logoColor=white" alt="Go Reference"></a>
    <a href="https://github.com/syst3mctl/godoclive/releases"><img src="https://img.shields.io/github/v/release/syst3mctl/godoclive?style=flat-square&color=blue" alt="Release"></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License: MIT"></a>
    <a href="https://codecov.io/gh/syst3mctl/godoclive/branch/main/graph/badge.svg"><img src="https://codecov.io/gh/syst3mctl/godoclive/branch/main/graph/badge.svg" alt="Codecov"></a>
  </p>
</p>


<img src="assets/demo.gif"
     alt="GoDoc Live demo" width="800">


> **Your API docs are already in your code. GoDoc Live just reads them.**
>
> GoDoc Live uses `go/ast` and `go/types` — the same packages the Go compiler uses — to walk your source, extract every route, parameter, request body, response type, and auth pattern, then generates an interactive docs site and OpenAPI 3.1.0 spec. No YAML to maintain. No annotations to add. No code to change.

## Quickstart

```bash
go install github.com/syst3mctl/godoclive/cmd/godoclive@latest
godoclive generate ./...
open docs/index.html
```

Or serve with **live reload** — docs update as you save:

```bash
godoclive watch --serve :8080 ./...
```

## Why GoDoc Live?

API docs written by hand drift. Someone adds a query param and forgets the spec. Someone changes a status code and the YAML still says 200. Six months later your docs and your code contradict each other.

GoDoc Live has no drift problem — it reads the source of truth directly.

| | GoDoc Live | Swagger annotations | Manual OpenAPI |
|---|---|---|---|
| Setup | `go install` | Add annotations to every handler | Write YAML by hand |
| Stays in sync | Always | Only if you update annotations | Only if you update YAML |
| Code changes required | None | Yes | No |
| Works on existing code | Yes | Partial | No |

## What It Detects

| Feature | Description |
|---------|-------------|
| **Routes** | HTTP methods and path patterns from router registrations, across every framework a project uses |
| **Path Params** | Type inference from name heuristics (`{id}` → uuid, `{page}` → integer) and handler body analysis |
| **Query Params** | Required/optional detection, default values from `DefaultQuery` |
| **Request Body** | Struct extraction from `json.Decode` / `c.ShouldBindJSON` with full field metadata |
| **Responses** | Status codes paired with response body types via branch-aware analysis; a status answered with more than one shape keeps both, as a `oneOf` |
| **File Uploads** | Multipart detection from `r.FormFile` / `c.FormFile` |
| **Helper Tracing** | One-level tracing through `respond()`, `writeJSON()`, `sendError()` wrappers |
| **Cookies** | `r.Cookie` / `c.Cookie` / `c.Cookies` → OpenAPI `in: cookie` parameters |
| **Auth Detection** | JWT bearer, API key, and basic auth from middleware body scanning |
| **Validation Rules** | `binding` / `validate` tags → `minLength`, `maximum`, `pattern`, `format`, `enum` |
| **Enums** | Constants declared with a named string or integer type become the schema's `enum` |
| **Doc Comments** | The handler's own comment becomes the summary and description |
| **Auto Naming** | Tags, and summaries for handlers with no doc comment (`GetUserByID` → "Get User By ID") |

## Documenting an Endpoint

The comment already above the handler is the documentation. Its first sentence
becomes the summary and the rest becomes the description; the leading
identifier is dropped, because repeating the function name next to the method
and path reads as a stutter.

```go
// ListArticles returns the articles visible to the caller.
//
// The session cookie decides which drafts are included; without one only
// published articles are returned.
func ListArticles(w http.ResponseWriter, r *http.Request) {
```

> Summary: "Returns the articles visible to the caller."
> Description: "The session cookie decides which drafts are included; without one only published articles are returned."

A handler with no comment falls back to its name, as before. `Deprecated:`
paragraphs are recorded as the `deprecated` flag rather than shown as prose.

## Validation Rules and Enums

Validator rules are enforced at runtime, so a schema that omits them advertises
an API more permissive than the one actually running. Both spellings are read —
gin's `binding` and go-playground's `validate` — and each rule is applied to the
facet the field's type makes it mean: `min=3` bounds a string's length, an
integer's value, and a slice's item count.

```go
type CreateArticleRequest struct {
    Title      string   `json:"title"       validate:"required,min=3,max=120"`
    Email      string   `json:"email"       validate:"required,email"`
    Visibility string   `json:"visibility"  validate:"required,oneof=public unlisted private"`
    WordCount  int      `json:"word_count"  validate:"gte=300,lte=5000"`
    Tags       []string `json:"tags"        validate:"required,min=1,max=5"`
    Status     Status   `json:"status"`
}
```

| Rule | Emitted as |
|------|-----------|
| `min` / `max` | `minLength`/`maxLength`, `minimum`/`maximum`, or `minItems`/`maxItems` |
| `gte` / `lte` | `minimum` / `maximum` |
| `gt` / `lt` | `exclusiveMinimum` / `exclusiveMaximum` |
| `len` | Both ends of the matching pair |
| `oneof` | `enum` |
| `email`, `uuid`, `url`, `ip`, `datetime`, … | `format` |
| `alpha`, `alphanum`, `numeric`, `lowercase`, … | `pattern` |

`Status` above needs no rule at all. A named type over a string or an integer
with a const block is how Go spells an enumeration, and those constants become
the schema's `enum` in declaration order — with the first of them used as the
example, so nothing suggests sending a value the server will reject.

```go
type Status string

const (
    StatusDraft     Status = "draft"
    StatusPublished Status = "published"
)
```

## UI Features

### Environment URL Switcher

A dropdown in the topbar lets you switch base URLs on the fly — curl snippets and the Try It panel update immediately. Define environments in `.godoclive.yaml`:

```yaml
openapi:
  servers:
    - url: "http://localhost:8080"
      description: "Local"
    - url: "https://staging-api.example.com"
      description: "Staging"
    - url: "https://api.example.com"
      description: "Production"
```

If no servers are configured, a single **Default** entry is created from `--base-url`. The selected URL persists across page refreshes via `localStorage`. A pencil button lets you type a fully custom URL at any time.

### Client-Side Route Visibility

Hide individual endpoints directly from the sidebar — useful when you want to focus on a subset of routes without permanently excluding them from the source.

- Hover any sidebar row → an eye icon appears
- Click it → the endpoint disappears from the sidebar and content area
- The sidebar footer shows **Show N hidden** — click to reveal hidden routes dimmed in place
- Click the eye-off icon on a revealed route to unhide it
- Hidden state is keyed by `"METHOD /path"` and persisted in `localStorage`, surviving page refreshes and doc regeneration

## Supported Routers

| Router | Status | Features |
|--------|--------|----------|
| **chi** (`go-chi/chi/v5`) | Done | Route, Group, Mount, inline handlers, cross-package registration functions |
| **gin** (`gin-gonic/gin`) | Done | Groups, Use chains, ShouldBindJSON, cross-package registration functions |
| **net/http** (Go 1.22+ stdlib) | Done | `"METHOD /path"` patterns, `r.PathValue()`, `http.Handler`, cross-package registration functions |
| **gorilla/mux** (`gorilla/mux`) | Done | `HandleFunc().Methods()`, `PathPrefix().Subrouter()`, `mux.Vars()`, regex params, cross-package registration functions |
| **echo** (`labstack/echo/v4`) | Done | Groups, Use chains, `c.Bind()`, `c.QueryParam()`, `c.JSON()`, `c.NoContent()`, cross-package registration functions |
| **fiber** (`gofiber/fiber/v2`) | Done | Groups, Use chains, `c.BodyParser()`, `c.Query()`, `c.Params()`, `c.Status().JSON()`, `c.SendStatus()`, cross-package registration functions |

A service is not obliged to pick one. Every framework a project registers
routes on is analyzed and the results are merged, so a gin API mounted beside a
stdlib `ServeMux` for health and metrics — or a chi tree that still carries a
legacy gorilla subtree — is documented in full. Each endpoint records which
framework registered it.

### House Router Wrappers

Plenty of teams never call their router directly. They put a type in front of
it and register through that:

```go
package httpx

type Router struct{ mux *http.ServeMux }

// Handle registers a handler for a method-and-path pattern.
func (r *Router) Handle(pattern string, h http.HandlerFunc) {
    r.mux.HandleFunc(pattern, h)
}
```

```go
r := httpx.New()
r.Handle("GET /users", handlers.ListUsers)
```

Walking that wrapper on its own finds a registration whose path is an
identifier and whose handler is a parameter — neither resolves, and the whole
service comes back empty. GoDoc Live binds each call site's arguments to the
wrapper's parameters and walks the body once per call, so the path and handler
resolve as if they had been written inline. It works for all six frameworks,
and across packages: the wrapper, the handlers and the wiring can each live in
their own.

## CLI Reference

### `godoclive generate [packages]`

```bash
godoclive generate ./...
godoclive generate --output ./api-docs --theme dark ./...
godoclive generate --format single ./...         # Single self-contained HTML (~300KB)
godoclive generate --serve :8080 ./...           # Generate + serve
godoclive generate --openapi ./openapi.json ./... # Also emit OpenAPI 3.1.0 spec
```

| Flag | Default | Description |
|------|---------|-------------|
| `--output` | `./docs` | Output directory |
| `--format` | `folder` | `folder` (separate files) or `single` (one self-contained HTML) |
| `--title` | auto | Project title displayed in docs |
| `--base-url` | — | Pre-fill base URL in Try It |
| `--theme` | `light` | `light` or `dark` |
| `--serve` | — | Start HTTP server after generation (e.g., `:8080`) |
| `--openapi` | — | Also generate an OpenAPI 3.1.0 spec at the given path (`.json` or `.yaml`) |

### `godoclive analyze [packages]`

Run analysis and print a contract summary to stdout.

```bash
godoclive analyze ./...
godoclive analyze --json ./...        # Machine-readable output
godoclive analyze --verbose ./...     # Show unresolved details
```

| Flag | Default | Description |
|------|---------|-------------|
| `--json` | `false` | Output as machine-readable JSON |
| `--verbose` | `false` | Show full unresolved list per endpoint |

### `godoclive openapi [packages]`

Generate an OpenAPI 3.1.0 specification without the HTML docs.

```bash
godoclive openapi ./...                          # Outputs ./openapi.json
godoclive openapi --output ./api.yaml ./...      # YAML format (inferred from extension)
godoclive openapi --server https://api.example.com ./...
```

| Flag | Default | Description |
|------|---------|-------------|
| `--output` | `./openapi.json` | Output file path (`.json` or `.yaml`) |
| `--format` | auto | `json` or `yaml` — inferred from file extension if omitted |
| `--title` | auto | API title in the spec `info` block |
| `--server` | — | Server URL to include in the `servers` array |

### `godoclive watch [packages]`

Watch for `.go` file changes and regenerate docs automatically. Supports the same flags as `generate`.

```bash
godoclive watch --serve :8080 ./...
```

When `--serve` is set, the browser **auto-reloads** via Server-Sent Events — edit your code, save, see updated docs instantly.

### `godoclive validate [packages]`

Report analysis coverage — what percentage of endpoints are fully resolved.

```bash
godoclive validate ./...
godoclive validate --json ./...
```

| Flag | Default | Description |
|------|---------|-------------|
| `--json` | `false` | Output as JSON |
| `--verbose` | `false` | Show full unresolved list per endpoint |

Coverage counts an endpoint as resolved only when nothing about it is in
doubt. These all reduce it, and are grouped by category in the report:

| Category | Meaning |
|----------|---------|
| `route group origin unknown` | Routes registered on a router parameter whose call site could not be traced — the prefix and middleware chain (including auth) are incomplete |
| `empty route path` / `unresolved route path` | The registered path is empty, or is not a compile-time constant |
| `openapi collision` | Two operations share a method and a path that differ only in parameter names — OpenAPI treats these as the same path, so one silently replaces the other |
| `middleware` | A middleware defined in the analyzed packages could not be resolved, so auth requirements may be understated |
| `request body` | A binding wrapper was recognized but its schema could not be traced to a concrete type |

## Ignoring Routes

### `//godoclive:ignore` directive

Add a `//godoclive:ignore` (or `//godoclive:skip`) comment directly in your source to permanently exclude a route from all output — HTML docs, OpenAPI spec, and analysis. The route is filtered at extraction time and never enters the pipeline.

```go
//godoclive:ignore
r.Get("/debug/pprof", pprof.Index)

r.Post("/internal/webhook", webhookHandler) //godoclive:ignore
```

Both placement styles are supported:
- **Preceding line** — comment on the line immediately above the route registration
- **Trailing comment** — comment on the same line as the route registration

The directive works across all supported routers (chi, gin, gorilla/mux, echo, fiber, net/http stdlib).

> For pattern-based exclusion (e.g. all routes matching `GET /internal/*`), use the `exclude` field in `.godoclive.yaml` instead.

## Configuration

### `.env` file

Create a `.env` file in your project root to set the API base URL for the Try It panel:

```env
API_URL="http://localhost:8080"
```

Precedence: `--base-url` CLI flag > `.env` `API_URL` > `.godoclive.yaml` `base_url` > default.

### `.godoclive.yaml`

Create an optional `.godoclive.yaml` in your project root:

```yaml
# Project metadata
title: "My API"
version: "v2.1.0"
base_url: "https://api.example.com"
theme: "dark"

# Exclude endpoints by pattern (glob on "METHOD /path")
# Use //godoclive:ignore in source for per-route exclusion instead.
exclude:
  - "GET /internal/*"
  - "* /debug/*"

# Override or supplement analysis results
overrides:
  - path: "POST /users"
    summary: "Register a new user account"
    description: "Creates an account and sends a confirmation email."
    tags: ["accounts"]
    responses:
      - status: 409
        description: "Email already registered"
      - status: 503
        description: "Service temporarily unavailable"

# Auth configuration
auth:
  header: "Authorization"
  scheme: "bearer"

# OpenAPI 3.1.0 spec metadata
openapi:
  description: "Full description of the API."
  contact:
    name: "API Support"
    url: "https://example.com/support"
    email: "support@example.com"
  license:
    name: "MIT"
    url: "https://opensource.org/licenses/MIT"
  servers:
    - url: "https://api.example.com"
      description: "Production"
    - url: "https://staging.example.com"
      description: "Staging"
```

> **Zero configuration is always valid** — the tool produces useful output without any config file.

## How It Works

```
┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐
│  1. Load  │──▶│2. Detect │──▶│3. Extract│──▶│4. Resolve│
│ go/pkgs   │   │ chi/gin  │   │  routes  │   │ handlers │
└──────────┘   └──────────┘   └──────────┘   └──────────┘
                                                    │
┌──────────┐   ┌──────────┐   ┌──────────┐   ┌─────▼────┐
│8.Generate│◀──│ 7. Auth  │◀──│ 6. Map   │◀──│5.Contract│
│ HTML/CSS │   │ middleware│   │ structs  │   │params/body│
└──────────┘   └──────────┘   └──────────┘   └──────────┘
```

1. **Load** — Uses `go/packages` to load and type-check your Go source code
2. **Detect** — Identifies the router framework (chi, gin, gorilla/mux, echo, fiber, or stdlib) from imports
3. **Extract** — Walks the AST to find route registrations, wherever they live: `main()`, a `routes` package, a method on a server struct, or a sub-router factory followed through its mount site. Works the same for every supported router
4. **Resolve** — Resolves handler expressions to function declarations
5. **Contract** — Extracts path params, query params, headers, body, and responses from handler ASTs
6. **Map** — Converts `types.Type` into recursive `TypeDef` with JSON tags, examples, and field metadata
7. **Auth** — Scans middleware function bodies for authentication patterns
8. **Generate** — Transforms endpoint contracts into an interactive HTML documentation site

> All analysis uses `go/ast` and `go/types` — **no runtime reflection, no annotations, no code generation**.

## GitHub Action

Generate the spec and the docs in CI, gate a pull request on how much of the
API the analyzer could resolve, and fail it on a breaking change.

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0          # needed by fail-on-breaking

- uses: syst3mctl/godoclive@v1
  with:
    packages: ./...
    openapi: openapi.json
    docs: site
    min-coverage: "95"
    fail-on-breaking: "true"
```

The breaking-change check generates the base branch's spec **from the base
branch's own source** rather than reading a spec committed to the repository. A
committed spec can itself be stale, and comparing against a stale one reports
the drift as an API change. The comparison runs through
[oasdiff](https://github.com/oasdiff/oasdiff); findings are annotated inline on
the pull request and summarized in the run.

| Input | Default | Description |
|-------|---------|-------------|
| `packages` | `./...` | Package pattern to analyze |
| `working-directory` | `.` | Module root of the service |
| `version` | `latest` | godoclive version to install — pin it |
| `go-version` | from `go.mod` | Toolchain to type-check against |
| `openapi` | `openapi.json` | Where to write the spec; empty to skip |
| `docs` | — | Where to write the HTML site; empty to skip |
| `docs-format` | `folder` | `folder` or `single` |
| `min-coverage` | — | Fail below this resolved-endpoint percentage |
| `fail-on-breaking` | `false` | Fail on a breaking change against the base ref |
| `base-ref` | PR base branch | Ref to compare against |

Outputs: `coverage`, `endpoints`, `unresolved`, `spec`, `breaking`.

## Programmatic API

Use GoDoc Live as a library in your own tools:

```go
import "github.com/syst3mctl/godoclive"

// Analyze a project
endpoints, err := godoclive.Analyze(".", "./...",
    godoclive.WithTitle("My API"),
)

// Generate HTML docs
err = godoclive.Generate(endpoints,
    godoclive.WithOutput("./api-docs"),
    godoclive.WithFormat("single"),
    godoclive.WithTheme("dark"),
)

// Generate HTML docs + OpenAPI spec in one call
err = godoclive.Generate(endpoints,
    godoclive.WithOutput("./api-docs"),
    godoclive.WithOpenAPIOutput("./openapi.json"),
)

// Generate only an OpenAPI 3.1.0 spec (returns JSON bytes)
specBytes, err := godoclive.GenerateOpenAPI(endpoints,
    godoclive.WithTitle("My API"),
    godoclive.WithVersion("v2.1.0"),
)
```

## Accuracy (Phase 1)

Measured across 12 testdata projects with 50 endpoints:

| Feature | Accuracy | Target |
|---------|----------|--------|
| Route detection | **100%** (50 endpoints) | 95% |
| Path params | **100%** (50 endpoints) | 99% |
| Query params | **100%** (50 endpoints) | 85% |
| Response status codes | **100%** (50 endpoints) | 85% |
| Auth detection | **100%** (50 endpoints) | 87% |

## Performance

Benchmarks run on Apple M2 Pro against the 27-route `gin-realworld` fixture. Results are the
median of five runs with a warm Go build cache. Run them yourself with
`go test -bench=. -benchmem ./internal/pipeline/ ./internal/generator/`.

### Analysis pipeline

| Implementation | Go | Time | Heap allocated | Allocations | Speedup |
|----------------|----|------|----------------|-------------|---------|
| v0.4.15 baseline | 1.25 | ~725 ms | 823 MiB | 8.80 M | — |
| Current | 1.25 | **~155 ms** | **10.3 MiB** | **88.0 K** | **4.66×** |
| v0.4.15 baseline | 1.27 | ~691 ms | 882 MiB | 9.64 M | — |
| Current | 1.27 | **~170 ms** | **12.0 MiB** | **93.6 K** | **4.06×** |

Dependencies are type-checked from export data rather than parsed from source. Only the
application packages retain syntax trees, and declaration lookups scan those packages directly
instead of walking the dependency graph once per route. Compared with v0.4.15, the combined
change is 4.1–4.7× faster and performs about 100× fewer allocations. The reported heap is total
memory allocated during one analysis; it is not retained after the call.

### Documentation generation

| Benchmark | Endpoints | Time | Memory |
|-----------|-----------|------|--------|
| `Generate` folder mode | 6 | ~1.6 ms | 317 KB / 190 allocs |
| `Generate` single mode | 6 | ~1.0 ms | 3.2 MB / 173 allocs |

Single-file mode writes more memory (≈10× more per run) because all CSS, JS, and WOFF2 font assets are base64-encoded and inlined into one self-contained HTML file (~300 KB on disk).

## Roadmap

| Phase | Scope | Status |
|-------|-------|--------|
| **1** | chi + gin + net/http stdlib + gorilla/mux, full contract extraction, helper tracing, interactive docs UI | Done |
| **2** | OpenAPI 3.1.0 export (`openapi` command + `--openapi` flag) | Done |
| **2b** | echo | Done |
| **2c** | fiber | Done |
| **2d** | Environment URL switcher, client-side route visibility toggle, `//godoclive:ignore` directive | Done |
| **3** | GitHub Action with coverage and breaking-change gates | Done |
| **3b** | VS Code extension | Planned |
| **4** | Multi-service gateway view, API version diff | Planned |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on adding new router extractors, structuring testdata, and running the test suite.

## License

MIT — see [LICENSE](LICENSE).

---

<p align="center">
  Made with love and <code>go/ast</code>
</p>
