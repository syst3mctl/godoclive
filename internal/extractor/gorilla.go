package extractor

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"

	"golang.org/x/tools/go/packages"
)

var gorillaParamRegex = regexp.MustCompile(`\{([^:}]+)(:[^}]*)?\}`)

// GorillaExtractor extracts routes from gorilla/mux router registrations.
type GorillaExtractor struct{}

// Extract walks all packages and extracts gorilla/mux route registrations.
//
// Functions that own a router outright are walked first, with every call to a
// registration function expanded inline so the subrouter prefix and middleware
// chain at the call site flow into the routes that function registers. A second
// pass picks up registration functions no call site reached, so their routes
// still appear, carrying the caveat that their prefix is unknown.
func (e *GorillaExtractor) Extract(pkgs []*packages.Package) ([]RawRoute, error) {
	idx := buildRouterIndex(pkgs, routerIndexSpec{
		inScope:   isGorillaPackage,
		registers: registersGorillaRoutes,
		isRouter:  isGorillaMuxType,
		wrapsPath: wrapsGorillaRoutes,
	})

	var routes []RawRoute
	for _, h := range idx.ordered {
		if h.wraps {
			// A house router wrapper only means something once expanded at a
			// call site; on its own its path is a parameter name.
			continue
		}
		if h.paramIdx >= 0 {
			continue // registers on a router it is handed: reached via call sites
		}
		w := newGorillaWalker(h.pkg, h.astFile, idx, nil)
		w.walkBlock(h.decl.Body.List, "", nil)
		routes = append(routes, w.routes...)
	}

	for _, h := range idx.ordered {
		if h.paramIdx < 0 || idx.reached[h.decl] {
			continue
		}
		w := newGorillaWalker(h.pkg, h.astFile, idx, []string{unknownOriginNote(h.decl.Name.Name)})
		w.walkBlock(h.decl.Body.List, "", nil)
		routes = append(routes, w.routes...)
	}

	return routes, nil
}

// gorillaPkgPath is the import path of gorilla/mux.
const gorillaPkgPath = "github.com/gorilla/mux"

// isGorillaPackage returns true if the package imports gorilla/mux.
func isGorillaPackage(pkg *packages.Package) bool {
	for imp := range pkg.Imports {
		if imp == gorillaPkgPath {
			return true
		}
	}
	return false
}

// isGorillaMuxType reports whether a types.Type is *mux.Router, comparing the
// owning package path rather than the printed type string.
func isGorillaMuxType(t types.Type) bool {
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
		obj.Pkg().Path() == gorillaPkgPath && obj.Name() == "Router"
}

// isGorillaReceiver reports whether the receiver of a method call is a
// mux.Router, e.g. the r in r.HandleFunc("/x", h).
func isGorillaReceiver(sel *ast.SelectorExpr, info *types.Info) bool {
	if info == nil {
		return false
	}
	// Resolving the selection first also covers a router reached through an
	// embedded field (type Server struct{ *mux.Router }).
	if receiverInPackage(sel, info, func(p string) bool { return p == gorillaPkgPath }) {
		return true
	}
	return isGorillaMuxType(info.TypeOf(sel.X))
}

// gorillaRouterMethods are the mux.Router methods that structure a router or
// register on it.
var gorillaRouterMethods = map[string]bool{
	"HandleFunc": true,
	"Handle":     true,
	"Use":        true,
	"PathPrefix": true,
}

// registersGorillaRoutes reports whether a function body calls a mux.Router
// method on a router value. Gating on the body rather than on the signature
// covers the shapes real projects use to set routes up outside main() — a
// factory returning http.Handler, a method on a server struct — none of which
// name a mux type in their signature.
func registersGorillaRoutes(fn *ast.FuncDecl, info *types.Info) bool {
	if info == nil {
		return false
	}
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
		if !ok {
			return true
		}
		if !gorillaRouterMethods[sel.Sel.Name] {
			return true
		}
		if isGorillaReceiver(sel, info) {
			found = true
			return false
		}
		return true
	})
	return found
}

// gorillaWalker extracts gorilla/mux routes from a single file.
type gorillaWalker struct {
	bind       *paramBinding // set when walking a house router wrapper's body
	fset       *token.FileSet
	astFile    *ast.File
	file       string
	info       *types.Info
	idx        *routerIndex
	notes      []string
	depth      int
	stack      map[*ast.FuncDecl]bool
	routes     []RawRoute
	routerVars map[string]string // variable name → path prefix (for subrouters)
}

func newGorillaWalker(pkg *packages.Package, file *ast.File, idx *routerIndex, notes []string) *gorillaWalker {
	return &gorillaWalker{
		fset:       pkg.Fset,
		astFile:    file,
		file:       pkg.Fset.Position(file.Pos()).Filename,
		info:       pkg.TypesInfo,
		idx:        idx,
		notes:      notes,
		stack:      make(map[*ast.FuncDecl]bool),
		routerVars: make(map[string]string),
	}
}

// walkBlock walks a list of statements looking for gorilla/mux route registrations.
func (w *gorillaWalker) walkBlock(stmts []ast.Stmt, prefix string, parentMW []MiddlewareRef) {
	scopeMW := copyMiddleware(parentMW)

	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			w.handleAssign(s, prefix)
		case *ast.ExprStmt:
			call, ok := s.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			if !w.processExprCall(call, prefix, &scopeMW) {
				w.expandRegistrationCall(call, prefix, scopeMW)
			}
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
func (w *gorillaWalker) processExprCall(call *ast.CallExpr, prefix string, scopeMW *[]MiddlewareRef) bool {
	// Check if outermost call is .Methods("GET", ...)
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Methods" {
		methods := extractMethodStrings(call.Args)
		// The receiver of .Methods() should be the HandleFunc/Handle call.
		if innerCall, ok := sel.X.(*ast.CallExpr); ok {
			return w.addChainedRoute(innerCall, prefix, *scopeMW, methods)
		}
		return false
	}

	// Otherwise, normal dispatch (Use, HandleFunc without .Methods, etc.)
	return w.processCall(call, prefix, scopeMW)
}

// processCall handles non-chained calls: Use and HandleFunc/Handle without
// .Methods(). It reports whether the call was a router registration.
func (w *gorillaWalker) processCall(call *ast.CallExpr, prefix string, scopeMW *[]MiddlewareRef) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	// Handle and Use are ordinary method names on plenty of types. Now that
	// every route-setup function is walked, the receiver has to be confirmed as
	// a mux.Router or an unrelated bus.Handle("topic", fn) becomes a route.
	if !isGorillaReceiver(sel, w.info) {
		return false
	}
	name := sel.Sel.Name

	switch name {
	case "Use":
		*scopeMW = append(*scopeMW, middlewareRefs(call.Args, w.info)...)

	case "HandleFunc", "Handle":
		if len(call.Args) >= 2 {
			w.addRoute(call, w.receiverPrefix(sel, prefix), *scopeMW)
		}

	default:
		return false
	}
	return true
}

// receiverPrefix resolves the path prefix a registration's receiver carries: a
// subrouter variable brings its own, anything else uses the current scope.
func (w *gorillaWalker) receiverPrefix(sel *ast.SelectorExpr, prefix string) string {
	if recvIdent, ok := sel.X.(*ast.Ident); ok {
		if p, isRouter := w.routerVars[recvIdent.Name]; isRouter {
			return p
		}
	}
	return prefix
}

// expandRegistrationCall handles a plain call to a function that registers
// routes on a router it is handed, e.g. routes.Register(r).
func (w *gorillaWalker) expandRegistrationCall(call *ast.CallExpr, prefix string, scopeMW []MiddlewareRef) {
	h := w.idx.lookup(w.info, call.Fun)
	if h == nil {
		return
	}
	if h.wraps && canWrap(h, call) {
		w.expandWrapperCall(h, call, prefix, scopeMW)
		return
	}
	if h.paramIdx < 0 || h.paramIdx >= len(call.Args) {
		return
	}
	arg := call.Args[h.paramIdx]
	if !isGorillaMuxType(w.info.TypeOf(arg)) {
		return
	}
	// A subrouter handed to the registrar carries its own prefix.
	argPrefix := prefix
	if ident, ok := arg.(*ast.Ident); ok {
		if p, isRouter := w.routerVars[ident.Name]; isRouter {
			argPrefix = p
		}
	}
	w.expandInto(h, argPrefix, scopeMW)
}

// expandInto walks a registration function's body under the given prefix and
// middleware chain, folding the routes it finds into this walker.
func (w *gorillaWalker) expandInto(h *routerHelper, prefix string, parentMW []MiddlewareRef) {
	if h == nil {
		return
	}
	w.idx.reached[h.decl] = true

	if w.depth >= maxHelperDepth || w.stack[h.decl] {
		return // recursion or runaway nesting: stop, but stay marked as reached
	}

	inner := newGorillaWalker(h.pkg, h.astFile, w.idx, w.notes)
	inner.depth = w.depth + 1
	for decl := range w.stack {
		inner.stack[decl] = true
	}
	inner.stack[h.decl] = true
	inner.walkBlock(h.decl.Body.List, prefix, parentMW)

	w.routes = append(w.routes, inner.routes...)
}

// addRoute records a route without .Methods() chain (ANY method).
func (w *gorillaWalker) addRoute(call *ast.CallExpr, prefix string, middlewares []MiddlewareRef) {
	if hasIgnoreDirective(w.fset, w.astFile, call.Pos(), call.End()) {
		return
	}
	patternArg := boundString(w.info, w.bind, call.Args[0])
	fullPath := NormalizeGorillaPath(joinPath(prefix, patternArg))
	handler, handlerInfo, substituted := boundExpr(w.info, w.bind, call.Args[1])

	pos := w.fset.Position(call.Pos())
	route := RawRoute{
		Method:      "ANY",
		Path:        fullPath,
		HandlerExpr: handler,
		Middlewares: middlewares,
		File:        w.file,
		Line:        pos.Line,
		Unresolved:  w.notes,
	}
	w.routes = append(w.routes, applyBinding(route, w.bind, handlerInfo, substituted))
}

// addChainedRoute records routes from a HandleFunc/Handle call chained with .Methods().
func (w *gorillaWalker) addChainedRoute(call *ast.CallExpr, prefix string, middlewares []MiddlewareRef, methods []string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) < 2 {
		return false
	}
	name := sel.Sel.Name
	if name != "HandleFunc" && name != "Handle" {
		return false
	}
	if !isGorillaReceiver(sel, w.info) {
		return false
	}
	if hasIgnoreDirective(w.fset, w.astFile, call.Pos(), call.End()) {
		return true // a registration all the same, just suppressed
	}

	callPrefix := w.receiverPrefix(sel, prefix)

	patternArg := boundString(w.info, w.bind, call.Args[0])
	fullPath := NormalizeGorillaPath(joinPath(callPrefix, patternArg))
	handler, handlerInfo, substituted := boundExpr(w.info, w.bind, call.Args[1])

	pos := w.fset.Position(call.Pos())

	// One route per method.
	for _, m := range methods {
		route := RawRoute{
			Method:      m,
			Path:        fullPath,
			HandlerExpr: handler,
			Middlewares: middlewares,
			File:        w.file,
			Line:        pos.Line,
			Unresolved:  w.notes,
		}
		w.routes = append(w.routes, applyBinding(route, w.bind, handlerInfo, substituted))
	}
	return true
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

// wrapsGorillaRoutes reports whether a function is a house router wrapper over
// gorilla: it registers a route whose path it takes as a parameter.
func wrapsGorillaRoutes(fn *ast.FuncDecl, info *types.Info) bool {
	return wrapsRoutes(fn, info, func(name string) bool { return name == "HandleFunc" || name == "Handle" }, isGorillaMuxType)
}

// expandWrapperCall walks a house router wrapper's body with the call's
// arguments bound to its parameters, so the path and handler it was handed
// resolve.
func (w *gorillaWalker) expandWrapperCall(h *routerHelper, call *ast.CallExpr, prefix string, parentMW []MiddlewareRef) {
	if w.depth >= maxHelperDepth || w.stack[h.decl] {
		return
	}
	bind := bindCallArgs(h, call, w.fset, w.info)
	if bind == nil {
		return
	}
	w.idx.reached[h.decl] = true

	inner := newGorillaWalker(h.pkg, h.astFile, w.idx, w.notes)
	inner.depth = w.depth + 1
	inner.bind = bind
	for decl := range w.stack {
		inner.stack[decl] = true
	}
	inner.stack[h.decl] = true
	inner.walkBlock(h.decl.Body.List, prefix, parentMW)

	w.routes = append(w.routes, inner.routes...)
}
