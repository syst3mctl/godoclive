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

// chiSubrouterMethods are the chi methods that structure a router rather than
// register a single endpoint.
var chiSubrouterMethods = map[string]bool{
	"Route": true,
	"Group": true,
	"Mount": true,
}

// ChiExtractor extracts routes from go-chi/chi router registrations.
type ChiExtractor struct{}

// Extract walks all packages and extracts chi route registrations.
//
// Extraction runs in two passes. The first walks the functions that own a
// router outright — main(), setupRoutes(), a method on a server struct — and
// expands every call to a registration function inline, so the mount prefix and
// middleware chain at the call site flow into the routes that function
// registers. A router factory another function mounts is emitted only through
// its mount site, never twice. The second pass picks up registration functions
// no call site reached, so their routes still appear, carrying the caveat that
// their prefix is unknown.
func (e *ChiExtractor) Extract(pkgs []*packages.Package) ([]RawRoute, error) {
	idx := buildRouterIndex(pkgs, routerIndexSpec{
		inScope:   isChiPackage,
		registers: registersChiRoutes,
		isRouter:  isChiType,
	})

	// Pass 1. Walking is separated from emission because reachability is only
	// known once every root has been walked: a factory mounted by a function
	// later in the index would otherwise already have been emitted at its bare
	// prefix.
	type walkedRoot struct {
		decl   *ast.FuncDecl
		routes []RawRoute
	}
	var walked []walkedRoot
	for _, h := range idx.ordered {
		if h.paramIdx >= 0 {
			continue // registers on a router it is handed: reached via call sites
		}
		w := newChiWalker(h.pkg, h.astFile, idx, nil)
		w.walkBlock(h.decl.Body, "", nil)
		walked = append(walked, walkedRoot{decl: h.decl, routes: w.routes})
	}

	var routes []RawRoute
	for _, root := range walked {
		if idx.reached[root.decl] {
			// Mounted by another function, which emitted these routes under the
			// mount prefix. Emitting them here too would duplicate every route
			// at a path missing that prefix.
			continue
		}
		routes = append(routes, root.routes...)
	}

	// Pass 2.
	for _, h := range idx.ordered {
		if h.paramIdx < 0 || idx.reached[h.decl] {
			continue
		}
		w := newChiWalker(h.pkg, h.astFile, idx, []string{unknownOriginNote(h.decl.Name.Name)})
		w.walkBlock(h.decl.Body, "", nil)
		routes = append(routes, w.routes...)
	}

	return routes, nil
}

// isChiPackage returns true if the package imports chi.
func isChiPackage(pkg *packages.Package) bool {
	for imp := range pkg.Imports {
		if isChiImport(imp) {
			return true
		}
	}
	return false
}

// isChiImport reports whether an import path is a chi package, for any major
// version suffix (github.com/go-chi/chi, .../chi/v5, …).
func isChiImport(importPath string) bool {
	return importPath == "github.com/go-chi/chi" ||
		strings.HasPrefix(importPath, "github.com/go-chi/chi/")
}

// isChiType reports whether a types.Type is a chi router type (chi.Router or
// chi.Mux, possibly behind a pointer). The owning package path is compared
// rather than the printed type string: chi v5 prints as
// "github.com/go-chi/chi/v5.Router", so substring-matching "chi.Router" misses
// every versioned import — which is every modern chi project.
func isChiType(t types.Type) bool {
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
	if obj == nil || obj.Pkg() == nil || !isChiImport(obj.Pkg().Path()) {
		return false
	}
	return obj.Name() == "Router" || obj.Name() == "Mux"
}

// isChiReceiver reports whether the receiver of a method call is a chi router
// value, e.g. the r in r.Get("/x", h).
func isChiReceiver(sel *ast.SelectorExpr, info *types.Info) bool {
	if info == nil {
		return false
	}
	// Resolving the selection first also covers a router reached through an
	// embedded field (type Server struct{ chi.Router }), where the receiver's
	// own type is not a chi type but the method resolves into chi.
	if receiverInPackage(sel, info, isChiImport) {
		return true
	}
	return isChiType(info.TypeOf(sel.X))
}

// registersChiRoutes reports whether a function body calls a chi registration
// method on a chi router value. Gating on the body rather than on the signature
// covers every shape real projects use to set routes up outside main() — a
// helper returning http.Handler, a method on a server struct, a router held in
// a struct field — none of which name a chi type in their signature.
func registersChiRoutes(fn *ast.FuncDecl, info *types.Info) bool {
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
		if chiMethods[sel.Sel.Name] == "" && !chiSubrouterMethods[sel.Sel.Name] {
			return true
		}
		if isChiReceiver(sel, info) {
			found = true
			return false
		}
		return true
	})
	return found
}

// chiWalker extracts chi routes from a single file.
type chiWalker struct {
	fset       *token.FileSet
	astFile    *ast.File
	file       string
	info       *types.Info
	idx        *routerIndex
	notes      []string
	depth      int
	stack      map[*ast.FuncDecl]bool
	routerVars map[types.Object]*routerHelper
	routes     []RawRoute
}

func newChiWalker(pkg *packages.Package, file *ast.File, idx *routerIndex, notes []string) *chiWalker {
	return &chiWalker{
		fset:       pkg.Fset,
		astFile:    file,
		file:       pkg.Fset.Position(file.Pos()).Filename,
		info:       pkg.TypesInfo,
		idx:        idx,
		notes:      notes,
		stack:      make(map[*ast.FuncDecl]bool),
		routerVars: make(map[types.Object]*routerHelper),
	}
}

// walkBlock walks a block statement looking for chi route registrations.
// It tracks path prefix and middleware accumulation per scope.
func (w *chiWalker) walkBlock(block *ast.BlockStmt, prefix string, parentMW []MiddlewareRef) {
	if block == nil {
		return
	}
	w.walkStmts(block.List, prefix, parentMW)
}

// walkStmts processes a statement list, recording chi route registrations and
// descending into nested blocks (if/for/switch/…) so conditionally-mounted
// routes are not missed. Middleware added by a nested r.Use() stays scoped to
// that block, matching Go's lexical scoping.
func (w *chiWalker) walkStmts(stmts []ast.Stmt, prefix string, parentMW []MiddlewareRef) {
	scopeMW := copyMiddleware(parentMW)

	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.ExprStmt:
			if call, ok := s.X.(*ast.CallExpr); ok {
				if !w.processCall(call, prefix, &scopeMW) {
					w.expandRegistrationCall(call, prefix, scopeMW)
				}
			}
			continue
		case *ast.AssignStmt:
			w.recordRouterVar(s)
		}
		for _, body := range nestedStmtBodies(stmt) {
			w.walkStmts(body, prefix, scopeMW)
		}
	}
}

// recordRouterVar remembers `admin := adminRouter()`, so that a later
// r.Mount("/admin", admin) can be traced back to the function that built it.
func (w *chiWalker) recordRouterVar(assign *ast.AssignStmt) {
	if w.info == nil || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok {
		return
	}
	h := w.idx.lookup(w.info, call.Fun)
	if h == nil || h.paramIdx >= 0 {
		return
	}
	ident, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return
	}
	obj := w.info.Defs[ident]
	if obj == nil {
		obj = w.info.Uses[ident]
	}
	if obj != nil {
		w.routerVars[obj] = h
	}
}

// processCall dispatches a call expression based on the method name. It reports
// whether the call was a chi router registration.
func (w *chiWalker) processCall(call *ast.CallExpr, prefix string, scopeMW *[]MiddlewareRef) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	// Method names alone are far too common to key on — cache.Get("k", d) is
	// not a route. Every registration is confirmed against the receiver's type,
	// which is what makes it safe to walk non-entry-point functions.
	if !isChiReceiver(sel, w.info) {
		return false
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
		*scopeMW = append(*scopeMW, w.mwRefs(call.Args)...)

	case chiMethods[name] != "" && len(call.Args) >= 2:
		allMW := concatMiddleware(*scopeMW, w.mwRefs(withMW))
		w.addRoute(chiMethods[name], prefix, call, allMW)

	case name == "Route" && len(call.Args) >= 2:
		subPrefix := stringLitValue(call.Args[0])
		w.descendInto(call.Args[1], joinPath(prefix, subPrefix), *scopeMW)

	case name == "Group" && len(call.Args) >= 1:
		w.descendInto(call.Args[0], prefix, *scopeMW)

	case name == "Mount" && len(call.Args) >= 2:
		subPrefix := stringLitValue(call.Args[0])
		w.descendInto(call.Args[1], joinPath(prefix, subPrefix), *scopeMW)

	default:
		return false
	}
	return true
}

// addRoute records a discovered route.
func (w *chiWalker) addRoute(method, prefix string, call *ast.CallExpr, middlewares []MiddlewareRef) {
	if hasIgnoreDirective(w.fset, w.astFile, call.Pos(), call.End()) {
		return
	}
	pathArg := stringLitValue(call.Args[0])
	fullPath := joinPath(prefix, pathArg)

	pos := w.fset.Position(call.Pos())
	w.routes = append(w.routes, RawRoute{
		Method:      method,
		Path:        fullPath,
		HandlerExpr: call.Args[1],
		Middlewares: middlewares,
		File:        w.file,
		Line:        pos.Line,
		Unresolved:  w.notes,
	})
}

// descendInto walks into the sub-router argument of Route/Group/Mount. Besides
// the inline func literal, that argument is often a router built elsewhere —
// r.Mount("/admin", adminRouter()) — which has to be followed for the mounted
// routes to carry their prefix.
func (w *chiWalker) descendInto(arg ast.Expr, prefix string, parentMW []MiddlewareRef) {
	switch a := arg.(type) {
	case *ast.FuncLit:
		w.walkBlock(a.Body, prefix, parentMW)
	case *ast.CallExpr:
		// r.Mount("/admin", adminRouter())
		w.expandInto(w.idx.lookup(w.info, a.Fun), prefix, parentMW)
	case *ast.Ident, *ast.SelectorExpr:
		// r.Route("/api", api.Register) — a func value passed by name — or a
		// variable holding a router a factory returned earlier.
		w.expandInto(w.lookupRouterValue(arg), prefix, parentMW)
	}
}

// expandRegistrationCall handles a plain call to a function that registers
// routes on a router it is handed, e.g. routes.RegisterRoutes(r) in main().
func (w *chiWalker) expandRegistrationCall(call *ast.CallExpr, prefix string, scopeMW []MiddlewareRef) {
	h := w.idx.lookup(w.info, call.Fun)
	if h == nil || h.paramIdx < 0 {
		return
	}
	// Only follow the call when a router really is being handed over; a call
	// passing something else in that position is not this registration.
	if h.paramIdx >= len(call.Args) || !isChiType(w.info.TypeOf(call.Args[h.paramIdx])) {
		return
	}
	w.expandInto(h, prefix, scopeMW)
}

// expandInto walks a registration function's body under the given prefix and
// middleware chain, folding the routes it finds into this walker.
func (w *chiWalker) expandInto(h *routerHelper, prefix string, parentMW []MiddlewareRef) {
	if h == nil {
		return
	}
	w.idx.reached[h.decl] = true

	if w.depth >= maxHelperDepth || w.stack[h.decl] {
		return // recursion or runaway nesting: stop, but stay marked as reached
	}

	inner := newChiWalker(h.pkg, h.astFile, w.idx, w.notes)
	inner.depth = w.depth + 1
	for decl := range w.stack {
		inner.stack[decl] = true
	}
	inner.stack[h.decl] = true
	inner.walkBlock(h.decl.Body, prefix, parentMW)

	w.routes = append(w.routes, inner.routes...)
}

// lookupRouterValue resolves an identifier used as a sub-router: either a
// registration function referenced by name, or a variable a factory assigned.
func (w *chiWalker) lookupRouterValue(expr ast.Expr) *routerHelper {
	if h := w.idx.lookup(w.info, expr); h != nil {
		return h
	}
	if obj := identObject(w.info, expr); obj != nil {
		return w.routerVars[obj]
	}
	return nil
}

// mwRefs pairs middleware expressions with the type information of the package
// they were written in, so a chain assembled across packages stays resolvable.
func (w *chiWalker) mwRefs(exprs []ast.Expr) []MiddlewareRef {
	return middlewareRefs(exprs, w.info)
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
