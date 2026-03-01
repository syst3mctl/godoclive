package extractor

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ginMethods maps gin router method names to HTTP methods.
var ginMethods = map[string]string{
	"GET":     "GET",
	"POST":    "POST",
	"PUT":     "PUT",
	"DELETE":  "DELETE",
	"PATCH":   "PATCH",
	"HEAD":    "HEAD",
	"OPTIONS": "OPTIONS",
	"Any":     "ANY",
}

// GinExtractor extracts routes from gin-gonic/gin router registrations.
type GinExtractor struct{}

// Extract walks all packages and extracts gin route registrations.
func (e *GinExtractor) Extract(pkgs []*packages.Package) ([]RawRoute, error) {
	var routes []RawRoute

	for _, pkg := range pkgs {
		if !isGinPackage(pkg) {
			continue
		}
		for _, file := range pkg.Syntax {
			fpath := pkg.Fset.Position(file.Pos()).Filename
			w := &ginWalker{
				fset:   pkg.Fset,
				file:   fpath,
				groups: make(map[string]*ginGroup),
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				// Skip test and example functions to avoid extracting
				// routes from test code.
				if strings.HasPrefix(fn.Name.Name, "Test") || strings.HasPrefix(fn.Name.Name, "Example") {
					continue
				}
				w.walkBlock(fn.Body.List, "", nil)
			}
			routes = append(routes, w.routes...)
		}
	}

	return routes, nil
}

// isGinPackage returns true if the package imports gin.
func isGinPackage(pkg *packages.Package) bool {
	for imp := range pkg.Imports {
		if imp == "github.com/gin-gonic/gin" ||
			strings.HasPrefix(imp, "github.com/gin-gonic/gin/") {
			return true
		}
	}
	return false
}

// ginGroup tracks a gin RouterGroup variable.
type ginGroup struct {
	prefix      string
	middlewares []ast.Expr
}

// ginWalker extracts gin routes from a single file.
type ginWalker struct {
	fset   *token.FileSet
	file   string
	routes []RawRoute
	groups map[string]*ginGroup // varName → group info
}

// walkBlock walks a list of statements in two passes:
// 1. Identify all group variable assignments and build the group map.
// 2. Process all statements, resolving group variables as needed.
func (w *ginWalker) walkBlock(stmts []ast.Stmt, prefix string, parentMW []ast.Expr) {
	// Pass 1: discover groups and .Use() calls to build group info.
	// We process statements in order so .Use() calls accumulate properly.
	scopeMW := copyExprs(parentMW)

	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			w.handleAssign(s, prefix, scopeMW)
		case *ast.ExprStmt:
			call, ok := s.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}

			receiverName := w.identName(sel.X)
			name := sel.Sel.Name

			// .Use() on the current router (not a group variable)
			if name == "Use" && !w.isGroup(receiverName) {
				for _, arg := range call.Args {
					scopeMW = append(scopeMW, arg)
				}
				continue
			}

			// .Use() on a group variable
			if name == "Use" && w.isGroup(receiverName) {
				g := w.groups[receiverName]
				for _, arg := range call.Args {
					g.middlewares = append(g.middlewares, arg)
				}
				continue
			}

			// Route method on the current router
			if ginMethods[name] != "" && len(call.Args) >= 2 && !w.isGroup(receiverName) {
				w.addRoute(ginMethods[name], prefix, call, scopeMW)
				continue
			}

			// Route method on a group variable
			if ginMethods[name] != "" && len(call.Args) >= 2 && w.isGroup(receiverName) {
				g := w.groups[receiverName]
				allMW := concatExprs(scopeMW, g.middlewares)
				w.addRoute(ginMethods[name], g.prefix, call, allMW)
				continue
			}
		}
	}
}

// handleAssign processes assignment statements, detecting group creation.
func (w *ginWalker) handleAssign(assign *ast.AssignStmt, prefix string, scopeMW []ast.Expr) {
	if len(assign.Lhs) == 0 || len(assign.Rhs) == 0 {
		return
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok {
		return
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Group" || len(call.Args) < 1 {
		return
	}
	lhs, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return
	}

	// Determine the parent prefix. If the receiver is a known group, use its prefix.
	parentPrefix := prefix
	parentMW := copyExprs(scopeMW)
	receiverName := w.identName(sel.X)
	if g, ok := w.groups[receiverName]; ok {
		parentPrefix = g.prefix
		parentMW = concatExprs(scopeMW, g.middlewares)
	}

	groupPrefix := stringLitValue(call.Args[0])
	var groupMW []ast.Expr
	for _, arg := range call.Args[1:] {
		groupMW = append(groupMW, arg)
	}

	w.groups[lhs.Name] = &ginGroup{
		prefix:      joinPath(parentPrefix, groupPrefix),
		middlewares: concatExprs(parentMW, groupMW),
	}
}

// identName returns the name of an identifier expression, or empty string.
func (w *ginWalker) identName(expr ast.Expr) string {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// isGroup checks if a variable name is a known group.
func (w *ginWalker) isGroup(name string) bool {
	_, ok := w.groups[name]
	return ok
}

// addRoute records a discovered gin route with path normalization.
func (w *ginWalker) addRoute(method, prefix string, call *ast.CallExpr, middlewares []ast.Expr) {
	pathArg := stringLitValue(call.Args[0])
	fullPath := joinPath(prefix, pathArg)
	fullPath = normalizeGinPath(fullPath)

	// For gin, the handler is the last argument; middlewares are in between.
	handler := call.Args[len(call.Args)-1]

	// Any args between path and handler are inline middlewares.
	var inlineMW []ast.Expr
	if len(call.Args) > 2 {
		inlineMW = call.Args[1 : len(call.Args)-1]
	}

	allMW := concatExprs(middlewares, inlineMW)

	pos := w.fset.Position(call.Pos())
	w.routes = append(w.routes, RawRoute{
		Method:      method,
		Path:        fullPath,
		HandlerExpr: handler,
		Middlewares: copyExprs(allMW),
		File:        w.file,
		Line:        pos.Line,
	})
}

// normalizeGinPath converts gin path params to the normalized {param} format.
func normalizeGinPath(p string) string {
	segments := strings.Split(p, "/")
	for i, seg := range segments {
		if strings.HasPrefix(seg, ":") {
			segments[i] = "{" + seg[1:] + "}"
		} else if strings.HasPrefix(seg, "*") {
			segments[i] = "{" + seg[1:] + "}"
		}
	}
	return strings.Join(segments, "/")
}
