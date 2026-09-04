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

// maxChiHelperDepth bounds how deep registration-function expansion recurses so
// a function that (directly or mutually) calls itself cannot loop forever.
const maxChiHelperDepth = 8

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
	idx := buildChiIndex(pkgs)

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
		w := newChiWalker(h.pkg, h.astFile, idx, []string{chiUnknownOriginNote(h)})
		w.walkBlock(h.decl.Body, "", nil)
		routes = append(routes, w.routes...)
	}

	return routes, nil
}

// chiUnknownOriginNote explains why a registration function's routes may be
// missing their prefix.
func chiUnknownOriginNote(h *chiHelper) string {
	return "route group origin unknown: " + h.decl.Name.Name +
		" registers routes on a router parameter, but no resolvable call site was found — " +
		"path prefix and middleware chain (including auth) are incomplete"
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
	if s, ok := info.Selections[sel]; ok && s.Obj() != nil {
		if pkg := s.Obj().Pkg(); pkg != nil && isChiImport(pkg.Path()) {
			return true
		}
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

// --- Registration index ---

// chiHelper is a function that registers chi routes. paramIdx is the argument
// position of its chi router parameter, or -1 when the function builds its own
// router — a factory such as adminRouter() http.Handler, or main() itself.
type chiHelper struct {
	decl     *ast.FuncDecl
	pkg      *packages.Package
	astFile  *ast.File
	paramIdx int
}

// chiIndex maps function objects to their registration record and tracks which
// of them an expansion has already reached.
type chiIndex struct {
	byObj   map[types.Object]*chiHelper
	ordered []*chiHelper
	reached map[*ast.FuncDecl]bool
}

// buildChiIndex finds every function (including methods) that takes part in
// chi route setup, across all packages that import chi.
//
// The first round indexes functions that call chi registration methods
// directly. Later rounds add the functions that hand a chi router to an
// already-indexed one — a main() whose whole routing body is
// routes.Register(r), or a delegator that fans out to per-resource registrars,
// registers nothing itself yet is the only link between the entry point and the
// routes. Rounds repeat until the set stops growing, so a chain of delegators
// is followed to its end.
func buildChiIndex(pkgs []*packages.Package) *chiIndex {
	idx := &chiIndex{
		byObj:   make(map[types.Object]*chiHelper),
		reached: make(map[*ast.FuncDecl]bool),
	}

	type candidate struct {
		fn      *ast.FuncDecl
		pkg     *packages.Package
		astFile *ast.File
	}
	var pending []candidate
	for _, pkg := range pkgs {
		if !isChiPackage(pkg) || pkg.TypesInfo == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				// Skip test and example functions so routes registered by test
				// fixtures do not reach the documentation.
				if strings.HasPrefix(fn.Name.Name, "Test") || strings.HasPrefix(fn.Name.Name, "Example") {
					continue
				}
				pending = append(pending, candidate{fn: fn, pkg: pkg, astFile: file})
			}
		}
	}

	for {
		remaining := pending[:0:0]
		var added bool
		for _, c := range pending {
			info := c.pkg.TypesInfo
			if !registersChiRoutes(c.fn, info) && !handsRouterToRegistration(c.fn, info, idx) {
				remaining = append(remaining, c)
				continue
			}
			h := &chiHelper{
				decl:     c.fn,
				pkg:      c.pkg,
				astFile:  c.astFile,
				paramIdx: chiRouterParam(c.fn, info),
			}
			idx.ordered = append(idx.ordered, h)
			if obj, ok := info.Defs[c.fn.Name]; ok && obj != nil {
				idx.byObj[obj] = h
			}
			added = true
		}
		pending = remaining
		if !added {
			break
		}
	}
	return idx
}

// handsRouterToRegistration reports whether fn calls an already-indexed
// registration function, passing a chi router in the parameter position that
// function registers on.
func handsRouterToRegistration(fn *ast.FuncDecl, info *types.Info, idx *chiIndex) bool {
	if info == nil || len(idx.byObj) == 0 {
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
		var ident *ast.Ident
		switch f := call.Fun.(type) {
		case *ast.Ident:
			ident = f
		case *ast.SelectorExpr:
			ident = f.Sel
		default:
			return true
		}
		obj := info.Uses[ident]
		if obj == nil {
			return true
		}
		h, ok := idx.byObj[obj]
		if !ok || h.paramIdx < 0 || h.paramIdx >= len(call.Args) {
			return true
		}
		if isChiType(info.TypeOf(call.Args[h.paramIdx])) {
			found = true
			return false
		}
		return true
	})
	return found
}

// chiRouterParam returns the argument position of the first chi router
// parameter of fn, or -1 when it has none. Grouped parameters (`a, b
// chi.Router`) count each name separately so the index matches the call's
// argument positions.
func chiRouterParam(fn *ast.FuncDecl, info *types.Info) int {
	if fn.Type == nil || fn.Type.Params == nil {
		return -1
	}
	pos := 0
	for _, field := range fn.Type.Params.List {
		names := len(field.Names)
		if names == 0 {
			names = 1 // unnamed parameter still occupies a position
		}
		if isChiType(info.TypeOf(field.Type)) {
			return pos
		}
		pos += names
	}
	return -1
}

// chiWalker extracts chi routes from a single file.
type chiWalker struct {
	fset       *token.FileSet
	astFile    *ast.File
	file       string
	info       *types.Info
	idx        *chiIndex
	notes      []string
	depth      int
	stack      map[*ast.FuncDecl]bool
	routerVars map[types.Object]*chiHelper
	routes     []RawRoute
}

func newChiWalker(pkg *packages.Package, file *ast.File, idx *chiIndex, notes []string) *chiWalker {
	return &chiWalker{
		fset:       pkg.Fset,
		astFile:    file,
		file:       pkg.Fset.Position(file.Pos()).Filename,
		info:       pkg.TypesInfo,
		idx:        idx,
		notes:      notes,
		stack:      make(map[*ast.FuncDecl]bool),
		routerVars: make(map[types.Object]*chiHelper),
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
	h := w.lookupRegistrationFunc(call.Fun)
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
		w.expandInto(w.lookupRegistrationFunc(a.Fun), prefix, parentMW)
	case *ast.Ident, *ast.SelectorExpr:
		// r.Route("/api", api.Register) — a func value passed by name — or a
		// variable holding a router a factory returned earlier.
		w.expandInto(w.lookupRouterValue(arg), prefix, parentMW)
	}
}

// expandRegistrationCall handles a plain call to a function that registers
// routes on a router it is handed, e.g. routes.RegisterRoutes(r) in main().
func (w *chiWalker) expandRegistrationCall(call *ast.CallExpr, prefix string, scopeMW []MiddlewareRef) {
	h := w.lookupRegistrationFunc(call.Fun)
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
func (w *chiWalker) expandInto(h *chiHelper, prefix string, parentMW []MiddlewareRef) {
	if h == nil {
		return
	}
	w.idx.reached[h.decl] = true

	if w.depth >= maxChiHelperDepth || w.stack[h.decl] {
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

// lookupRegistrationFunc resolves a call's function expression to its
// registration record, or nil when it does not register chi routes.
func (w *chiWalker) lookupRegistrationFunc(expr ast.Expr) *chiHelper {
	obj := w.identObject(expr)
	if obj == nil {
		return nil
	}
	return w.idx.byObj[obj]
}

// lookupRouterValue resolves an identifier used as a sub-router: either a
// registration function referenced by name, or a variable a factory assigned.
func (w *chiWalker) lookupRouterValue(expr ast.Expr) *chiHelper {
	if h := w.lookupRegistrationFunc(expr); h != nil {
		return h
	}
	if obj := w.identObject(expr); obj != nil {
		return w.routerVars[obj]
	}
	return nil
}

// identObject resolves an identifier or selector expression to the object it
// refers to.
func (w *chiWalker) identObject(expr ast.Expr) types.Object {
	if w.info == nil || w.idx == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.Ident:
		return w.info.Uses[e]
	case *ast.SelectorExpr:
		return w.info.Uses[e.Sel]
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
