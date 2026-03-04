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

---

> **Zero annotations. Zero code changes. Just your existing Go handlers.**
> GoDoc Live statically analyzes your chi and gin routers, extracts every route, parameter, request body, and response — then generates interactive API documentation.

## Installation

```bash
go install github.com/syst3mctl/godoclive/cmd/godoclive@latest
```

## Quickstart

```bash
# Install
go install github.com/syst3mctl/godoclive/cmd/godoclive@latest

# Generate docs from your project
godoclive generate ./...

# Open the docs
open docs/index.html
```

Or serve with **live reload**:

```bash
godoclive watch ./... --serve :8080
```

## What It Detects

GoDoc Live uses `go/ast` and `go/types` to extract everything automatically:

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
| **chi** (`go-chi/chi/v5`) | Phase 1 | Route, Group, Mount, inline handlers |
| **gin** (`gin-gonic/gin`) | Phase 1 | Groups, Use chains, ShouldBindJSON |
| gorilla/mux | Planned | — |
| echo | Planned | — |
| fiber | Planned | — |

## CLI Reference

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

### `godoclive generate [packages]`

Run analysis and generate a documentation site.

```bash
godoclive generate ./...
godoclive generate --output ./api-docs --theme dark ./...
godoclive generate --format single ./...         # Single self-contained HTML (~300KB)
godoclive generate --serve :8080 ./...           # Generate + serve
```

| Flag | Default | Description |
|------|---------|-------------|
| `--output` | `./docs` | Output directory |
| `--format` | `folder` | `folder` (separate files) or `single` (one self-contained HTML) |
| `--title` | auto | Project title displayed in docs |
| `--base-url` | — | Pre-fill base URL in Try It |
| `--theme` | `light` | `light` or `dark` |
| `--serve` | — | Start HTTP server after generation (e.g., `:8080`) |

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
2. **Detect** — Identifies the router framework (chi or gin) from imports
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

// Generate docs
err = godoclive.Generate(endpoints,
    godoclive.WithOutput("./api-docs"),
    godoclive.WithFormat("single"),
    godoclive.WithTheme("dark"),
)
```

## Accuracy (Phase 1)

Measured across 9 testdata projects with 37 endpoints:

| Feature | Accuracy | Target |
|---------|----------|--------|
| Route detection | **100%** (37 endpoints) | 95% |
| Path params | **100%** (37 endpoints) | 99% |
| Query params | **100%** (37 endpoints) | 85% |
| Response status codes | **100%** (37 endpoints) | 85% |
| Auth detection | **100%** (37 endpoints) | 87% |

## Roadmap

| Phase | Scope | Status |
|-------|-------|--------|
| **1** | chi + gin, full contract extraction, helper tracing, interactive docs UI | Done |
| **2** | gorilla/mux, echo, fiber, OpenAPI 3.1 export | Planned |
| **3** | VS Code extension, GitHub Action integration | Planned |
| **4** | Multi-service gateway view, API version diff | Planned |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on adding new router extractors, structuring testdata, and running the test suite.

## License

MIT — see [LICENSE](LICENSE).

---

<p align="center">
  Made with 💙 and <code>go/ast</code>
</p>
