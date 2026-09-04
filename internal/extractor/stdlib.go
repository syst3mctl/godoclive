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
//
// Every function is walked, since a ServeMux carries no prefix for a call site
// to contribute. What a call site can contribute is the path and the handler
// themselves, when a house wrapper takes them as parameters — so calls to a
// function that registers routes are expanded with its arguments bound.
func (e *StdlibExtractor) Extract(pkgs []*packages.Package) ([]RawRoute, error) {
	wrappers := indexStdlibWrappers(pkgs)

	var routes []RawRoute
	for _, pkg := range pkgs {
		if !isStdlibHTTPPackage(pkg) {
			continue
		}
		for _, file := range pkg.Syntax {
			fpath := pkg.Fset.Position(file.Pos()).Filename
			w := &stdlibWalker{
				fset:     pkg.Fset,
				astFile:  file,
				file:     fpath,
				info:     pkg.TypesInfo,
				wrappers: wrappers,
				stack:    make(map[*ast.FuncDecl]bool),
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

// indexStdlibWrappers records every function that registers a route on a
// ServeMux, keyed by the object a call site resolves to.
func indexStdlibWrappers(pkgs []*packages.Package) map[types.Object]*routerHelper {
	index := make(map[types.Object]*routerHelper)
	for _, pkg := range pkgs {
		if !isStdlibHTTPPackage(pkg) || pkg.TypesInfo == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || fn.Type == nil {
					continue
				}
				if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
					continue // nothing a call site could bind
				}
				if strings.HasPrefix(fn.Name.Name, "Test") || strings.HasPrefix(fn.Name.Name, "Example") {
					continue
				}
				if !wrapsStdlibRoutes(fn, pkg.TypesInfo) {
					continue
				}
				if obj, ok := pkg.TypesInfo.Defs[fn.Name]; ok && obj != nil {
					index[obj] = &routerHelper{decl: fn, pkg: pkg, astFile: file, paramIdx: -1}
				}
			}
		}
	}
	return index
}

// wrapsStdlibRoutes reports whether a function registers a route on a ServeMux
// using a pattern it was handed — the shape of a house router wrapper.
//
// A function whose patterns are literals is not a wrapper: it resolves where it
// is declared, and every function is walked there, so expanding its call sites
// too would emit its routes twice.
func wrapsStdlibRoutes(fn *ast.FuncDecl, info *types.Info) bool {
	probe := &stdlibWalker{info: info}
	var found bool
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		if sel.Sel.Name != "HandleFunc" && sel.Sel.Name != "Handle" {
			return true
		}
		if probe.isMuxRegistration(sel) && paramIdent(fn, info, call.Args[0]) {
			found = true
			return false
		}
		return true
	})
	return found
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

	// House-router wrapper expansion.
	wrappers map[types.Object]*routerHelper
	bind     *paramBinding
	depth    int
	stack    map[*ast.FuncDecl]bool
}

// walkBlock walks a list of statements looking for stdlib route registrations.
func (w *stdlibWalker) walkBlock(stmts []ast.Stmt, parentMW []ast.Expr) {
	scopeMW := copyExprs(parentMW)

	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.ExprStmt:
			call, ok := s.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			w.processCall(call, scopeMW)
			w.expandWrapperCall(call, scopeMW)
		default:
			// Routes are commonly mounted conditionally (e.g.
			// `if dep != nil { mux.Handle(...) }`); descend into nested blocks.
			for _, body := range nestedStmtBodies(stmt) {
				w.walkBlock(body, scopeMW)
			}
		}
	}
}

// isMuxRegistration reports whether a HandleFunc/Handle call registers on a
// ServeMux: either the package-level http.HandleFunc, which registers on
// http.DefaultServeMux, or a method call on any *http.ServeMux value.
//
// The receiver is identified by its type rather than by tracking which local
// variables were assigned from http.NewServeMux(). Name tracking only ever saw
// a mux built in the same function, so a mux arriving as a parameter — the
// shape of every routes.Register(mux) helper — or held in a struct field was
// invisible.
func (w *stdlibWalker) isMuxRegistration(sel *ast.SelectorExpr) bool {
	if w.info == nil {
		return false
	}
	if ident, ok := sel.X.(*ast.Ident); ok {
		if pkgName, ok := w.info.Uses[ident].(*types.PkgName); ok {
			return pkgName.Imported().Path() == "net/http"
		}
	}
	return isServeMuxType(w.info.TypeOf(sel.X))
}

// isServeMuxType reports whether t is http.ServeMux, possibly behind a pointer.
func isServeMuxType(t types.Type) bool {
	if t == nil {
		return false
	}
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj != nil && obj.Pkg() != nil &&
		obj.Pkg().Path() == "net/http" && obj.Name() == "ServeMux"
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
		if w.isMuxRegistration(sel) {
			w.addRoute(call, scopeMW)
		}
	}
}

// addRoute parses the pattern string and records a route.
func (w *stdlibWalker) addRoute(call *ast.CallExpr, middlewares []ast.Expr) {
	if hasIgnoreDirective(w.fset, w.astFile, call.Pos(), call.End()) {
		return
	}
	pattern := boundString(w.info, w.bind, call.Args[0])
	if pattern == "" {
		return
	}

	method, path := parseStdlibPattern(pattern)

	// The handler is the last argument. Any args between pattern and handler
	// could be considered but stdlib only takes (pattern, handler).
	handler := call.Args[len(call.Args)-1]

	// A wrapper takes the handler as a parameter, so the expression to record
	// — and the type information it belongs to — comes from the call site.
	handler, handlerInfo, substituted := boundExpr(w.info, w.bind, handler)

	// Unwrap middleware wrapping: authMiddleware(handler) → collect authMiddleware, use inner handler.
	handler, wrappedMW := unwrapMiddleware(handler)
	allMW := concatExprs(middlewares, wrappedMW)

	pos := w.fset.Position(call.Pos())
	route := RawRoute{
		Method:      method,
		Path:        path,
		HandlerExpr: handler,
		Middlewares: middlewareRefs(allMW, w.info),
		File:        w.file,
		Line:        pos.Line,
	}
	w.routes = append(w.routes, applyBinding(route, w.bind, handlerInfo, substituted))
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

// expandWrapperCall expands a call to a house router wrapper — a function that
// registers a route from values it is handed — by walking its body with the
// call's arguments bound to its parameters.
func (w *stdlibWalker) expandWrapperCall(call *ast.CallExpr, scopeMW []ast.Expr) {
	if w.wrappers == nil || w.info == nil {
		return
	}
	obj := identObject(w.info, call.Fun)
	if obj == nil {
		return
	}
	h := w.wrappers[obj]
	if !canWrap(h, call) {
		return
	}
	if w.depth >= maxHelperDepth || w.stack[h.decl] {
		return // recursion, or nesting deep enough to be a mistake
	}
	bind := bindCallArgs(h, call, w.fset, w.info)
	if bind == nil {
		return
	}

	inner := &stdlibWalker{
		fset:     h.pkg.Fset,
		astFile:  h.astFile,
		file:     h.pkg.Fset.Position(h.astFile.Pos()).Filename,
		info:     h.pkg.TypesInfo,
		wrappers: w.wrappers,
		bind:     bind,
		depth:    w.depth + 1,
		stack:    make(map[*ast.FuncDecl]bool),
	}
	for decl := range w.stack {
		inner.stack[decl] = true
	}
	inner.stack[h.decl] = true
	inner.walkBlock(h.decl.Body.List, scopeMW)

	w.routes = append(w.routes, inner.routes...)
}
