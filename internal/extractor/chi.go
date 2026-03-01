package extractor

import (
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// chiMethods maps chi router method names to HTTP methods.
var chiMethods = map[string]string{
	"Get":     "GET",
	"Post":    "POST",
	"Put":     "PUT",
	"Delete":  "DELETE",
	"Patch":   "PATCH",
	"Head":    "HEAD",
	"Options": "OPTIONS",
}

// ChiExtractor extracts routes from go-chi/chi router registrations.
type ChiExtractor struct{}

// Extract walks all packages and extracts chi route registrations.
func (e *ChiExtractor) Extract(pkgs []*packages.Package) ([]RawRoute, error) {
	var routes []RawRoute

	for _, pkg := range pkgs {
		if !isChiPackage(pkg) {
			continue
		}
		for _, file := range pkg.Syntax {
			fpath := pkg.Fset.Position(file.Pos()).Filename
			w := &chiWalker{fset: pkg.Fset, file: fpath}
			// Walk entry-point functions (main, init) and any function that
			// receives or returns a chi.Router / chi.Mux, since real-world
			// apps commonly set up routes in setupRoutes() or similar helpers.
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				if fn.Name.Name == "main" || fn.Name.Name == "init" || usesChiRouter(fn, pkg.TypesInfo) {
					w.walkBlock(fn.Body, "", nil)
				}
			}
			routes = append(routes, w.routes...)
		}
	}

	return routes, nil
}

// usesChiRouter returns true if a function declaration has a parameter or
// return type involving chi.Router or chi.Mux, indicating it sets up routes.
func usesChiRouter(fn *ast.FuncDecl, info *types.Info) bool {
	if fn.Type == nil || info == nil {
		return false
	}
	// Check parameters.
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			if isChiRouterType(field.Type, info) {
				return true
			}
		}
	}
	// Check return types.
	if fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			if isChiRouterType(field.Type, info) {
				return true
			}
		}
	}
	return false
}

// isChiRouterType checks if an AST type expression refers to a chi router type.
func isChiRouterType(expr ast.Expr, info *types.Info) bool {
	t := info.TypeOf(expr)
	if t == nil {
		return false
	}
	return isChiType(t)
}

// isChiType recursively checks if a types.Type is a chi router type.
func isChiType(t types.Type) bool {
	// Unwrap pointer types.
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	s := t.String()
	return strings.Contains(s, "chi.Mux") || strings.Contains(s, "chi.Router")
}

// isChiPackage returns true if the package imports chi.
func isChiPackage(pkg *packages.Package) bool {
	for imp := range pkg.Imports {
		if imp == "github.com/go-chi/chi" ||
			imp == "github.com/go-chi/chi/v5" ||
			strings.HasPrefix(imp, "github.com/go-chi/chi/") {
			return true
		}
	}
	return false
}

// chiWalker extracts chi routes from a single file.
type chiWalker struct {
	fset   *token.FileSet
	file   string
	routes []RawRoute
}

// walkBlock walks a block statement looking for chi route registrations.
// It tracks path prefix and middleware accumulation per scope.
func (w *chiWalker) walkBlock(block *ast.BlockStmt, prefix string, parentMW []ast.Expr) {
	if block == nil {
		return
	}
	scopeMW := copyExprs(parentMW)

	for _, stmt := range block.List {
		exprStmt, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := exprStmt.X.(*ast.CallExpr)
		if !ok {
			continue
		}

		w.processCall(call, prefix, &scopeMW)
	}
}

// processCall dispatches a call expression based on the method name.
func (w *chiWalker) processCall(call *ast.CallExpr, prefix string, scopeMW *[]ast.Expr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	name := sel.Sel.Name

	// Detect chained With(): r.With(mw).Get("/path", handler)
	var withMW []ast.Expr
	if innerCall, ok := sel.X.(*ast.CallExpr); ok {
		if innerSel, ok := innerCall.Fun.(*ast.SelectorExpr); ok {
			if innerSel.Sel.Name == "With" {
				withMW = innerCall.Args
			}
		}
	}

	switch {
	case name == "Use":
		for _, arg := range call.Args {
			*scopeMW = append(*scopeMW, arg)
		}

	case chiMethods[name] != "" && len(call.Args) >= 2:
		allMW := concatExprs(*scopeMW, withMW)
		w.addRoute(chiMethods[name], prefix, call, allMW)

	case name == "Route" && len(call.Args) >= 2:
		subPrefix := stringLitValue(call.Args[0])
		w.descendInto(call.Args[1], joinPath(prefix, subPrefix), *scopeMW)

	case name == "Group" && len(call.Args) >= 1:
		w.descendInto(call.Args[0], prefix, *scopeMW)

	case name == "Mount" && len(call.Args) >= 2:
		subPrefix := stringLitValue(call.Args[0])
		w.descendInto(call.Args[1], joinPath(prefix, subPrefix), *scopeMW)
	}
}

// addRoute records a discovered route.
func (w *chiWalker) addRoute(method, prefix string, call *ast.CallExpr, middlewares []ast.Expr) {
	pathArg := stringLitValue(call.Args[0])
	fullPath := joinPath(prefix, pathArg)

	pos := w.fset.Position(call.Pos())
	w.routes = append(w.routes, RawRoute{
		Method:      method,
		Path:        fullPath,
		HandlerExpr: call.Args[1],
		Middlewares: copyExprs(middlewares),
		File:        w.file,
		Line:        pos.Line,
	})
}

// descendInto walks into a function literal argument with a new scope.
func (w *chiWalker) descendInto(arg ast.Expr, prefix string, parentMW []ast.Expr) {
	funcLit, ok := arg.(*ast.FuncLit)
	if !ok {
		return
	}
	w.walkBlock(funcLit.Body, prefix, parentMW)
}

// stringLitValue extracts the string value from a basic literal expression.
// Uses strconv.Unquote to correctly handle all Go string literal forms
// (double-quoted with escape sequences, raw backtick strings).
func stringLitValue(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		// Fallback: trim quotes manually.
		v := lit.Value
		if len(v) >= 2 {
			v = v[1 : len(v)-1]
		}
		return v
	}
	return s
}

// joinPath joins path segments, handling slashes correctly.
func joinPath(prefix, suffix string) string {
	if prefix == "" {
		return suffix
	}
	if suffix == "" || suffix == "/" {
		return prefix
	}
	return path.Join(prefix, suffix)
}

// copyExprs returns a shallow copy of an ast.Expr slice.
func copyExprs(exprs []ast.Expr) []ast.Expr {
	if len(exprs) == 0 {
		return nil
	}
	cp := make([]ast.Expr, len(exprs))
	copy(cp, exprs)
	return cp
}

// concatExprs returns a new slice containing elements from both slices.
func concatExprs(a, b []ast.Expr) []ast.Expr {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	result := make([]ast.Expr, 0, len(a)+len(b))
	result = append(result, a...)
	result = append(result, b...)
	return result
}
