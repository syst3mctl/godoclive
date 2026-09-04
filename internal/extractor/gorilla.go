package extractor

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/packages"
)

var gorillaParamRegex = regexp.MustCompile(`\{([^:}]+)(:[^}]*)?\}`)

// GorillaExtractor extracts routes from gorilla/mux router registrations.
type GorillaExtractor struct{}

// Extract walks all packages and extracts gorilla/mux route registrations.
func (e *GorillaExtractor) Extract(pkgs []*packages.Package) ([]RawRoute, error) {
	var routes []RawRoute

	for _, pkg := range pkgs {
		if !isGorillaPackage(pkg) {
			continue
		}
		for _, file := range pkg.Syntax {
			fpath := pkg.Fset.Position(file.Pos()).Filename
			w := &gorillaWalker{
				fset:       pkg.Fset,
				astFile:    file,
				file:       fpath,
				info:       pkg.TypesInfo,
				routerVars: make(map[string]string),
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				if fn.Name.Name == "main" || fn.Name.Name == "init" || usesGorillaMux(fn, pkg.TypesInfo) {
					w.walkBlock(fn.Body.List, "", nil)
				}
			}
			routes = append(routes, w.routes...)
		}
	}

	return routes, nil
}

// isGorillaPackage returns true if the package imports gorilla/mux.
func isGorillaPackage(pkg *packages.Package) bool {
	for imp := range pkg.Imports {
		if imp == "github.com/gorilla/mux" {
			return true
		}
	}
	return false
}

// isGorillaMuxType checks if a types.Type is a gorilla/mux router type.
func isGorillaMuxType(t types.Type) bool {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	s := t.String()
	return strings.Contains(s, "mux.Router")
}

// usesGorillaMux returns true if a FuncDecl has a param or return type
// involving *mux.Router, indicating it sets up routes.
func usesGorillaMux(fn *ast.FuncDecl, info *types.Info) bool {
	if fn.Type == nil || info == nil {
		return false
	}
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			t := info.TypeOf(field.Type)
			if t != nil && isGorillaMuxType(t) {
				return true
			}
		}
	}
	if fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			t := info.TypeOf(field.Type)
			if t != nil && isGorillaMuxType(t) {
				return true
			}
		}
	}
	return false
}

// gorillaWalker extracts gorilla/mux routes from a single file.
type gorillaWalker struct {
	fset       *token.FileSet
	astFile    *ast.File
	file       string
	info       *types.Info
	routes     []RawRoute
	routerVars map[string]string // variable name → path prefix (for subrouters)
}

// walkBlock walks a list of statements looking for gorilla/mux route registrations.
func (w *gorillaWalker) walkBlock(stmts []ast.Stmt, prefix string, parentMW []ast.Expr) {
	scopeMW := copyExprs(parentMW)

	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			w.handleAssign(s, prefix)
		case *ast.ExprStmt:
			call, ok := s.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			w.processExprCall(call, prefix, &scopeMW)
		default:
			// Routes may be registered conditionally; descend into nested blocks.
			for _, body := range nestedStmtBodies(stmt) {
				w.walkBlock(body, prefix, scopeMW)
			}
		}
	}
}

// handleAssign detects router and subrouter creation.
func (w *gorillaWalker) handleAssign(assign *ast.AssignStmt, currentPrefix string) {
	if len(assign.Lhs) == 0 || len(assign.Rhs) == 0 {
		return
	}
	lhs, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return
	}

	rhs := assign.Rhs[0]

	call, ok := rhs.(*ast.CallExpr)
	if !ok {
		return
	}

	// Case 1: r := mux.NewRouter()
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == "mux" && sel.Sel.Name == "NewRouter" {
				w.routerVars[lhs.Name] = ""
				return
			}
		}
	}

	// Case 2: sub := r.PathPrefix("/api").Subrouter()
	// AST shape: CallExpr{Fun: SelectorExpr{X: CallExpr{...PathPrefix}, Sel: "Subrouter"}}
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Subrouter" {
		if innerCall, ok := sel.X.(*ast.CallExpr); ok {
			if innerSel, ok := innerCall.Fun.(*ast.SelectorExpr); ok {
				if innerSel.Sel.Name == "PathPrefix" && len(innerCall.Args) >= 1 {
					subPrefix := stringLitValue(innerCall.Args[0])
					if recvIdent, ok := innerSel.X.(*ast.Ident); ok {
						parentPrefix := ""
						if p, isRouter := w.routerVars[recvIdent.Name]; isRouter {
							parentPrefix = p
						}
						w.routerVars[lhs.Name] = joinPath(parentPrefix, subPrefix)
					}
				}
			}
		}
	}
}

// processExprCall dispatches the outermost call expression.
// It first checks for the .Methods() chain pattern, then falls through to normal dispatch.
func (w *gorillaWalker) processExprCall(call *ast.CallExpr, prefix string, scopeMW *[]ast.Expr) {
	// Check if outermost call is .Methods("GET", ...)
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Methods" {
		methods := extractMethodStrings(call.Args)
		// The receiver of .Methods() should be the HandleFunc/Handle call.
		if innerCall, ok := sel.X.(*ast.CallExpr); ok {
			w.addChainedRoute(innerCall, prefix, *scopeMW, methods)
		}
		return
	}

	// Otherwise, normal dispatch (Use, HandleFunc without .Methods, etc.)
	w.processCall(call, prefix, scopeMW)
}

// processCall handles non-chained calls: Use and HandleFunc/Handle without .Methods().
func (w *gorillaWalker) processCall(call *ast.CallExpr, prefix string, scopeMW *[]ast.Expr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	name := sel.Sel.Name

	// Resolve receiver prefix for subrouters.
	callPrefix := prefix
	if recvIdent, ok := sel.X.(*ast.Ident); ok {
		if p, isRouter := w.routerVars[recvIdent.Name]; isRouter {
			callPrefix = p
		}
	}

	switch name {
	case "Use":
		*scopeMW = append(*scopeMW, call.Args...)

	case "HandleFunc", "Handle":
		if len(call.Args) >= 2 {
			w.addRoute(call, callPrefix, *scopeMW)
		}
	}
}

// addRoute records a route without .Methods() chain (ANY method).
func (w *gorillaWalker) addRoute(call *ast.CallExpr, prefix string, middlewares []ast.Expr) {
	if hasIgnoreDirective(w.fset, w.astFile, call.Pos(), call.End()) {
		return
	}
	patternArg := stringLitValue(call.Args[0])
	fullPath := NormalizeGorillaPath(joinPath(prefix, patternArg))
	handler := call.Args[1]

	pos := w.fset.Position(call.Pos())
	w.routes = append(w.routes, RawRoute{
		Method:      "ANY",
		Path:        fullPath,
		HandlerExpr: handler,
		Middlewares: middlewareRefs(middlewares, w.info),
		File:        w.file,
		Line:        pos.Line,
	})
}

// addChainedRoute records routes from a HandleFunc/Handle call chained with .Methods().
func (w *gorillaWalker) addChainedRoute(call *ast.CallExpr, prefix string, middlewares []ast.Expr, methods []string) {
	if hasIgnoreDirective(w.fset, w.astFile, call.Pos(), call.End()) {
		return
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) < 2 {
		return
	}
	name := sel.Sel.Name
	if name != "HandleFunc" && name != "Handle" {
		return
	}

	// Resolve receiver prefix.
	callPrefix := prefix
	if recvIdent, ok := sel.X.(*ast.Ident); ok {
		if p, isRouter := w.routerVars[recvIdent.Name]; isRouter {
			callPrefix = p
		}
	}

	patternArg := stringLitValue(call.Args[0])
	fullPath := NormalizeGorillaPath(joinPath(callPrefix, patternArg))
	handler := call.Args[1]

	pos := w.fset.Position(call.Pos())

	// One route per method.
	for _, m := range methods {
		w.routes = append(w.routes, RawRoute{
			Method:      m,
			Path:        fullPath,
			HandlerExpr: handler,
			Middlewares: middlewareRefs(middlewares, w.info),
			File:        w.file,
			Line:        pos.Line,
		})
	}
}

// extractMethodStrings extracts string literals from .Methods("GET", "POST") args.
func extractMethodStrings(args []ast.Expr) []string {
	var methods []string
	for _, arg := range args {
		if s := stringLitValue(arg); s != "" {
			methods = append(methods, s)
		}
	}
	return methods
}

// normalizeGorillaPath converts {id:[0-9]+} → {id}.
// NormalizeGorillaPath strips regex constraints from gorilla path parameters.
// e.g. "/items/{id:[0-9]+}" → "/items/{id}"
func NormalizeGorillaPath(path string) string {
	return gorillaParamRegex.ReplaceAllString(path, "{$1}")
}
