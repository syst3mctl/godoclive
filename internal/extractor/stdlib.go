package extractor

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

// StdlibExtractor extracts routes from net/http ServeMux registrations.
// Supports Go 1.22+ enhanced patterns ("GET /path/{id}") and pre-1.22 patterns.
type StdlibExtractor struct{}

// Extract walks all packages and extracts stdlib route registrations.
func (e *StdlibExtractor) Extract(pkgs []*packages.Package) ([]RawRoute, error) {
	var routes []RawRoute

	for _, pkg := range pkgs {
		if !isStdlibHTTPPackage(pkg) {
			continue
		}
		for _, file := range pkg.Syntax {
			fpath := pkg.Fset.Position(file.Pos()).Filename
			w := &stdlibWalker{
				fset:    pkg.Fset,
				astFile: file,
				file:    fpath,
				info:    pkg.TypesInfo,
				muxVars: make(map[string]bool),
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				// Skip test and example functions.
				if strings.HasPrefix(fn.Name.Name, "Test") || strings.HasPrefix(fn.Name.Name, "Example") {
					continue
				}
				w.walkBlock(fn.Body.List, nil)
			}
			routes = append(routes, w.routes...)
		}
	}

	return routes, nil
}

// isStdlibHTTPPackage returns true if the package imports net/http.
func isStdlibHTTPPackage(pkg *packages.Package) bool {
	for imp := range pkg.Imports {
		if imp == "net/http" {
			return true
		}
	}
	return false
}

// stdlibWalker extracts stdlib routes from a single file.
type stdlibWalker struct {
	fset    *token.FileSet
	astFile *ast.File
	file    string
	info    *types.Info
	routes  []RawRoute
	muxVars map[string]bool // tracks variables assigned from http.NewServeMux()
}

// walkBlock walks a list of statements looking for stdlib route registrations.
func (w *stdlibWalker) walkBlock(stmts []ast.Stmt, parentMW []ast.Expr) {
	scopeMW := copyExprs(parentMW)

	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			w.handleAssign(s)
		case *ast.ExprStmt:
			call, ok := s.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			w.processCall(call, scopeMW)
		default:
			// Routes are commonly mounted conditionally (e.g.
			// `if dep != nil { mux.Handle(...) }`); descend into nested blocks.
			for _, body := range nestedStmtBodies(stmt) {
				w.walkBlock(body, scopeMW)
			}
		}
	}
}

// handleAssign detects mux := http.NewServeMux() assignments.
func (w *stdlibWalker) handleAssign(assign *ast.AssignStmt) {
	if len(assign.Lhs) == 0 || len(assign.Rhs) == 0 {
		return
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok {
		return
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	if ident.Name == "http" && sel.Sel.Name == "NewServeMux" {
		lhs, ok := assign.Lhs[0].(*ast.Ident)
		if ok {
			w.muxVars[lhs.Name] = true
		}
	}
}

// processCall dispatches a call expression based on the method name.
func (w *stdlibWalker) processCall(call *ast.CallExpr, scopeMW []ast.Expr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	name := sel.Sel.Name

	switch {
	case (name == "HandleFunc" || name == "Handle") && len(call.Args) >= 2:
		// Could be http.HandleFunc or mux.HandleFunc
		isHTTPPkg := false
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == "http" {
				isHTTPPkg = true
			} else if w.muxVars[ident.Name] {
				isHTTPPkg = true // mux variable
			}
		}
		if isHTTPPkg {
			w.addRoute(call, scopeMW)
		}
	}
}

// addRoute parses the pattern string and records a route.
func (w *stdlibWalker) addRoute(call *ast.CallExpr, middlewares []ast.Expr) {
	if hasIgnoreDirective(w.fset, w.astFile, call.Pos()) {
		return
	}
	pattern := stringLitValue(call.Args[0])
	if pattern == "" {
		return
	}

	method, path := parseStdlibPattern(pattern)

	// The handler is the last argument. Any args between pattern and handler
	// could be considered but stdlib only takes (pattern, handler).
	handler := call.Args[len(call.Args)-1]

	// Unwrap middleware wrapping: authMiddleware(handler) → collect authMiddleware, use inner handler.
	handler, wrappedMW := unwrapMiddleware(handler)
	allMW := concatExprs(middlewares, wrappedMW)

	pos := w.fset.Position(call.Pos())
	w.routes = append(w.routes, RawRoute{
		Method:      method,
		Path:        path,
		HandlerExpr: handler,
		Middlewares: middlewareRefs(allMW, w.info),
		File:        w.file,
		Line:        pos.Line,
	})
}

// parseStdlibPattern parses a Go 1.22+ ServeMux pattern string.
// Patterns can be:
//   - "GET /users/{id}" → method="GET", path="/users/{id}"
//   - "POST /users"     → method="POST", path="/users"
//   - "/health"         → method="ANY", path="/health"
//   - "GET example.com/path" → method="GET", path="/path" (host ignored)
func parseStdlibPattern(pattern string) (method, path string) {
	pattern = strings.TrimSpace(pattern)

	// Check for method prefix: "GET /path" or "POST /path".
	parts := strings.SplitN(pattern, " ", 2)
	if len(parts) == 2 && isHTTPMethod(parts[0]) {
		method = parts[0]
		path = strings.TrimSpace(parts[1])
	} else {
		method = "ANY"
		path = pattern
	}

	// Strip host prefix if present: "example.com/path" → "/path".
	if !strings.HasPrefix(path, "/") {
		if idx := strings.Index(path, "/"); idx >= 0 {
			path = path[idx:]
		}
	}

	// Remove trailing {$} exact match marker (Go 1.22+).
	path = strings.TrimSuffix(path, "{$}")
	if path == "" {
		path = "/"
	}

	// Clean trailing slash for non-root paths to normalize,
	// but preserve it since in stdlib it means subtree matching.
	// We'll keep the path as-is for documentation purposes.

	return method, path
}

// isHTTPMethod returns true if s is a valid HTTP method.
func isHTTPMethod(s string) bool {
	switch s {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
		return true
	}
	return false
}

// unwrapMiddleware unwraps function call wrapping like authMiddleware(handler),
// collecting outer wrappers as middleware expressions.
// Returns the innermost handler and collected middleware.
func unwrapMiddleware(expr ast.Expr) (ast.Expr, []ast.Expr) {
	var middlewares []ast.Expr
	for {
		call, ok := expr.(*ast.CallExpr)
		if !ok {
			break
		}
		// If the call has exactly one argument and the function is an identifier
		// or selector (not a method call on an object), treat it as middleware wrapping.
		if len(call.Args) != 1 {
			break
		}
		// The wrapper function itself is middleware.
		middlewares = append(middlewares, call.Fun)
		expr = call.Args[0]
	}
	return expr, middlewares
}
