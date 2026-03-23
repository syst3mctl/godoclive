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
| **Routes** | HTTP methods and path patterns from router registrations |
| **Path Params** | Type inference from name heuristics (`{id}` → uuid, `{page}` → integer) and handler body analysis |
| **Query Params** | Required/optional detection, default values from `DefaultQuery` |
| **Request Body** | Struct extraction from `json.Decode` / `c.ShouldBindJSON` with full field metadata |
| **Responses** | Status codes paired with response body types via branch-aware analysis |
| **File Uploads** | Multipart detection from `r.FormFile` / `c.FormFile` |
| **Helper Tracing** | One-level tracing through `respond()`, `writeJSON()`, `sendError()` wrappers |
| **Auth Detection** | JWT bearer, API key, and basic auth from middleware body scanning |
| **Auto Naming** | Summaries and tags inferred from handler names (`GetUserByID` → "Get User By ID") |

## Supported Routers

| Router | Status | Features |
|--------|--------|----------|
| **chi** (`go-chi/chi/v5`) | Done | Route, Group, Mount, inline handlers |
| **gin** (`gin-gonic/gin`) | Done | Groups, Use chains, ShouldBindJSON |
| **net/http** (Go 1.22+ stdlib) | Done | `"METHOD /path"` patterns, `r.PathValue()`, `http.Handler` |
| **gorilla/mux** (`gorilla/mux`) | Done | `HandleFunc().Methods()`, `PathPrefix().Subrouter()`, `mux.Vars()`, regex params |
| **echo** (`labstack/echo/v4`) | Done | Groups, Use chains, `c.Bind()`, `c.QueryParam()`, `c.JSON()`, `c.NoContent()` |
| **fiber** (`gofiber/fiber/v2`) | Done | Groups, Use chains, `c.BodyParser()`, `c.Query()`, `c.Params()`, `c.Status().JSON()`, `c.SendStatus()` |

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

# Exclude endpoints from documentation
exclude:
  - "GET /internal/*"
  - "* /debug/*"

# Override or supplement analysis results
overrides:
  - path: "POST /users"
    summary: "Register a new user account"
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
3. **Extract** — Walks `main()` and `init()` AST to find route registrations
4. **Resolve** — Resolves handler expressions to function declarations
5. **Contract** — Extracts path params, query params, headers, body, and responses from handler ASTs
6. **Map** — Converts `types.Type` into recursive `TypeDef` with JSON tags, examples, and field metadata
7. **Auth** — Scans middleware function bodies for authentication patterns
8. **Generate** — Transforms endpoint contracts into an interactive HTML documentation site

> All analysis uses `go/ast` and `go/types` — **no runtime reflection, no annotations, no code generation**.

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

Benchmarks run on Apple M2 Pro, Go 1.25, using the testdata projects included in the repository.
Run them yourself: `go test -bench=. -benchmem ./internal/pipeline/ ./internal/generator/`

### Analysis pipeline

| Benchmark | Routes | Time | Memory |
|-----------|--------|------|--------|
| `LoadPackages` (chi-basic) | — | ~426 ms | 572 MB / 5.9 M allocs |
| `RunPipeline` chi-basic | 6 | ~429 ms | 572 MB / 5.9 M allocs |
| `RunPipeline` gorilla-basic | 8 | ~429 ms | 552 MB / 5.7 M allocs |
| `RunPipeline` gin-basic | 5 | ~538 ms | 840 MB / 8.7 M allocs |

**The dominant cost is `go/packages` type-checking** — loading and type-checking the full dependency tree via `NeedDeps`. Route extraction, contract analysis, struct mapping, and type lookups together add less than 5 ms on top. Memory is not retained after the call; it is held only during analysis and released when the returned `[]EndpointDef` is in scope.

The type-lookup path was optimised in v0.1.0: a single `packages.Visit` traversal builds a `pkgPath → name → types.Type` index at pipeline start, replacing the previous O(N×routes×deps) repeated traversals with O(1) map lookups.

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
| **3** | VS Code extension, GitHub Action integration | Planned |
| **4** | Multi-service gateway view, API version diff | Planned |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on adding new router extractors, structuring testdata, and running the test suite.

## License

MIT — see [LICENSE](LICENSE).

---

<p align="center">
  Made with love and <code>go/ast</code>
</p>
