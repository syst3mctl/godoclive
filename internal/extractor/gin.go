package extractor

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path"
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

// ginPkgPath is the import path every gin router type is defined in.
const ginPkgPath = "github.com/gin-gonic/gin"

// ginRouterTypes are the gin types that carry a route-registration surface. A
// function taking one of these as a parameter is a *registration helper*:
// `func UsersRegister(router *gin.RouterGroup)`. Its routes only have a real
// path once the group handed in at the call site is known.
var ginRouterTypes = map[string]bool{
	"RouterGroup": true,
	"Engine":      true,
	"IRouter":     true,
	"IRoutes":     true,
}

// maxGinHelperDepth bounds how deep registration-helper expansion recurses so
// a helper that (directly or mutually) calls itself cannot loop forever.
const maxGinHelperDepth = 8

// GinExtractor extracts routes from gin-gonic/gin router registrations.
type GinExtractor struct{}

// Extract walks all packages and extracts gin route registrations.
//
// Extraction runs in two passes. The first walks the functions that own a
// router outright — main(), setupRouter(), anything that is not itself a
// registration helper — expanding every call to a registration helper inline so
// the group prefix, accumulated middleware chain and trailing-slash semantics
// of the call site flow into the routes the helper registers. The second pass
// picks up registration helpers no call site reached, so their routes are still
// documented, flagged with the context that could not be established.
func (e *GinExtractor) Extract(pkgs []*packages.Package) ([]RawRoute, error) {
	idx := buildGinHelperIndex(pkgs)

	var routes []RawRoute

	// Pass 1: functions that are not registration helpers own their router.
	for _, pkg := range pkgs {
		if !isGinPackage(pkg) {
			continue
		}
		for _, file := range pkg.Syntax {
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
				if idx.isHelper(fn) {
					continue // reached through its call sites instead
				}
				w := newGinWalker(pkg, file, idx, nil)
				w.walkBlock(fn.Body.List, "", nil)
				routes = append(routes, w.routes...)
			}
		}
	}

	// Pass 2: registration helpers nothing called. Their prefix and middleware
	// chain are unknowable, so the routes carry that caveat rather than a
	// confidently wrong path.
	for _, h := range idx.ordered {
		if idx.reached[h.decl] {
			continue
		}
		w := newGinWalker(h.pkg, h.astFile, idx, []string{ginUnknownOriginNote(h)})
		if h.paramName != "" {
			w.groups[h.paramName] = &ginGroup{originUnknown: true}
		}
		w.walkBlock(h.decl.Body.List, "", nil)
		routes = append(routes, w.routes...)
	}

	return routes, nil
}

// ginUnknownOriginNote explains why a helper's routes are missing their prefix.
func ginUnknownOriginNote(h *ginHelper) string {
	return "route group origin unknown: " + h.decl.Name.Name +
		" registers routes on a router parameter, but no resolvable call site was found — " +
		"path prefix and middleware chain (including auth) are incomplete"
}

// isGinPackage returns true if the package imports gin.
func isGinPackage(pkg *packages.Package) bool {
	for imp := range pkg.Imports {
		if imp == ginPkgPath || strings.HasPrefix(imp, ginPkgPath+"/") {
			return true
		}
	}
	return false
}

// --- Registration helper index ---

// ginHelper is a function that registers routes on a router it receives as a
// parameter.
type ginHelper struct {
	decl      *ast.FuncDecl
	pkg       *packages.Package
	astFile   *ast.File
	paramIdx  int    // position of the router parameter in the call's arguments
	paramName string // the router parameter's identifier inside the helper
}

// ginHelperIndex maps function objects to their registration-helper record and
// tracks which helpers an expansion has already reached.
type ginHelperIndex struct {
	byObj   map[types.Object]*ginHelper
	byDecl  map[*ast.FuncDecl]*ginHelper
	ordered []*ginHelper
	reached map[*ast.FuncDecl]bool
}

func (idx *ginHelperIndex) isHelper(fn *ast.FuncDecl) bool {
	_, ok := idx.byDecl[fn]
	return ok
}

// buildGinHelperIndex finds every function (including methods) that accepts a
// gin router as a parameter.
func buildGinHelperIndex(pkgs []*packages.Package) *ginHelperIndex {
	idx := &ginHelperIndex{
		byObj:   make(map[types.Object]*ginHelper),
		byDecl:  make(map[*ast.FuncDecl]*ginHelper),
		reached: make(map[*ast.FuncDecl]bool),
	}

	for _, pkg := range pkgs {
		if !isGinPackage(pkg) || pkg.TypesInfo == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || fn.Type.Params == nil {
					continue
				}
				paramIdx, paramName, ok := ginRouterParam(fn, pkg.TypesInfo)
				if !ok {
					continue
				}
				h := &ginHelper{
					decl:      fn,
					pkg:       pkg,
					astFile:   file,
					paramIdx:  paramIdx,
					paramName: paramName,
				}
				idx.byDecl[fn] = h
				idx.ordered = append(idx.ordered, h)
				if obj, ok := pkg.TypesInfo.Defs[fn.Name]; ok && obj != nil {
					idx.byObj[obj] = h
				}
			}
		}
	}
	return idx
}

// ginRouterParam returns the argument position and identifier of the first gin
// router parameter of fn. Grouped parameters (`a, b *gin.RouterGroup`) count
// each name separately so the index matches the call's argument positions.
func ginRouterParam(fn *ast.FuncDecl, info *types.Info) (int, string, bool) {
	pos := 0
	for _, field := range fn.Type.Params.List {
		names := len(field.Names)
		if names == 0 {
			names = 1 // unnamed parameter still occupies a position
		}
		if isGinRouterType(info.TypeOf(field.Type)) {
			name := ""
			if len(field.Names) > 0 {
				name = field.Names[0].Name
			}
			if name == "_" {
				name = ""
			}
			return pos, name, true
		}
		pos += names
	}
	return 0, "", false
}

// isGinRouterType reports whether t is one of gin's route-registration types,
// with or without a pointer.
func isGinRouterType(t types.Type) bool {
	if t == nil {
		return false
	}
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	if named.Obj().Pkg().Path() != ginPkgPath {
		return false
	}
	return ginRouterTypes[named.Obj().Name()]
}

// --- Walker ---

// ginGroup tracks a gin RouterGroup value: the path prefix it contributes and
// the middleware chain accumulated at the point it was created.
type ginGroup struct {
	prefix        string
	middlewares   []MiddlewareRef
	originUnknown bool
}

// ginWalker extracts gin routes from a single function body.
type ginWalker struct {
	fset    *token.FileSet
	astFile *ast.File
	file    string
	info    *types.Info
	pkg     *packages.Package
	idx     *ginHelperIndex
	routes  []RawRoute
	groups  map[string]*ginGroup // varName → group info
	notes   []string             // caveats attached to every route this walker emits
	depth   int                  // registration-helper expansion depth
	stack   map[*ast.FuncDecl]bool
}

func newGinWalker(pkg *packages.Package, file *ast.File, idx *ginHelperIndex, notes []string) *ginWalker {
	return &ginWalker{
		fset:    pkg.Fset,
		astFile: file,
		file:    pkg.Fset.Position(file.Pos()).Filename,
		info:    pkg.TypesInfo,
		pkg:     pkg,
		idx:     idx,
		groups:  make(map[string]*ginGroup),
		notes:   notes,
		stack:   make(map[*ast.FuncDecl]bool),
	}
}

// walkBlock walks a list of statements in order, tracking group variables and
// `.Use()` calls as it goes so that middleware registered before a group is
// created applies to that group and middleware registered after does not —
// gin's own semantics, since Group() snapshots the parent's handler chain.
func (w *ginWalker) walkBlock(stmts []ast.Stmt, prefix string, parentMW []MiddlewareRef) {
	scopeMW := copyMiddleware(parentMW)

	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			w.handleAssign(s, prefix, scopeMW)
		case *ast.ExprStmt:
			call, ok := s.X.(*ast.CallExpr)
			if !ok {
				continue
			}

			// A call into a registration helper: expand it here so the group
			// handed over carries its prefix and middleware chain along.
			if w.expandHelperCall(call, prefix, scopeMW) {
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
				scopeMW = append(scopeMW, w.mwRefs(call.Args)...)
				continue
			}

			// .Use() on a group variable
			if name == "Use" && w.isGroup(receiverName) {
				g := w.groups[receiverName]
				g.middlewares = append(g.middlewares, w.mwRefs(call.Args)...)
				continue
			}

			// Route method on the current router
			if ginMethods[name] != "" && len(call.Args) >= 2 && !w.isGroup(receiverName) {
				w.addRoute(ginMethods[name], &ginGroup{prefix: prefix, middlewares: scopeMW}, call)
				continue
			}

			// Route method on a group variable
			if ginMethods[name] != "" && len(call.Args) >= 2 && w.isGroup(receiverName) {
				g := w.groups[receiverName]
				w.addRoute(ginMethods[name], &ginGroup{
					prefix:        g.prefix,
					middlewares:   concatMiddleware(scopeMW, g.middlewares),
					originUnknown: g.originUnknown,
				}, call)
				continue
			}
		default:
			// Routes may be registered conditionally; descend into nested blocks.
			for _, body := range nestedStmtBodies(stmt) {
				w.walkBlock(body, prefix, scopeMW)
			}
		}
	}
}

// handleAssign processes assignment statements, detecting group creation.
func (w *ginWalker) handleAssign(assign *ast.AssignStmt, prefix string, scopeMW []MiddlewareRef) {
	if len(assign.Lhs) == 0 || len(assign.Rhs) == 0 {
		return
	}
	lhs, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return
	}
	g, ok := w.resolveRouterExpr(assign.Rhs[0], prefix, scopeMW)
	if !ok {
		return
	}
	w.groups[lhs.Name] = g
}

// resolveRouterExpr resolves an expression that evaluates to a gin router into
// the prefix and middleware chain it represents. It handles the group variable
// (`v1`), the inline group call (`v1.Group("/users")`), and the engine itself
// (`r`). Anything else — a router returned by an opaque call, a struct field —
// is reported as unresolved so the caller can flag rather than guess.
func (w *ginWalker) resolveRouterExpr(expr ast.Expr, prefix string, scopeMW []MiddlewareRef) (*ginGroup, bool) {
	switch e := expr.(type) {
	case *ast.Ident:
		if g, ok := w.groups[e.Name]; ok {
			return &ginGroup{
				prefix:        g.prefix,
				middlewares:   concatMiddleware(scopeMW, g.middlewares),
				originUnknown: g.originUnknown,
			}, true
		}
		// The engine variable itself contributes no prefix; the routes it owns
		// inherit whatever middleware is in scope.
		if w.info != nil && isGinEngine(w.info.TypeOf(e)) {
			return &ginGroup{prefix: prefix, middlewares: copyMiddleware(scopeMW)}, true
		}
		return nil, false

	case *ast.CallExpr:
		sel, ok := e.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Group" || len(e.Args) < 1 {
			return nil, false
		}
		parent, ok := w.resolveRouterExpr(sel.X, prefix, scopeMW)
		if !ok {
			// The receiver is unknown, but the literal suffix still tells us
			// something; report it as unresolved so the caller flags it.
			return nil, false
		}
		groupPath, resolved := w.pathArgValue(e.Args[0])
		if !resolved {
			return nil, false
		}
		return &ginGroup{
			prefix:        joinGinPath(parent.prefix, groupPath),
			middlewares:   concatMiddleware(parent.middlewares, w.mwRefs(e.Args[1:])),
			originUnknown: parent.originUnknown,
		}, true
	}
	return nil, false
}

// isGinEngine reports whether t is *gin.Engine.
func isGinEngine(t types.Type) bool {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == ginPkgPath && named.Obj().Name() == "Engine"
}

// expandHelperCall detects a call to a registration helper and walks the
// helper's body with its router parameter bound to the group passed in at this
// call site. It reports whether the call was a registration helper — including
// when the router argument could not be resolved, in which case the helper's
// routes are still emitted, carrying the caveat.
func (w *ginWalker) expandHelperCall(call *ast.CallExpr, prefix string, scopeMW []MiddlewareRef) bool {
	if w.info == nil || w.idx == nil {
		return false
	}

	var obj types.Object
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		obj = w.info.Uses[fn]
	case *ast.SelectorExpr:
		obj = w.info.Uses[fn.Sel]
	default:
		return false
	}
	if obj == nil {
		return false
	}
	h, ok := w.idx.byObj[obj]
	if !ok {
		return false
	}

	w.idx.reached[h.decl] = true

	if w.depth >= maxGinHelperDepth || w.stack[h.decl] {
		return true // recursion or runaway nesting: stop, but do not re-walk as a root
	}

	notes := w.notes
	group := &ginGroup{}
	if h.paramIdx < len(call.Args) {
		if resolved, ok := w.resolveRouterExpr(call.Args[h.paramIdx], prefix, scopeMW); ok {
			group = resolved
		} else {
			group.originUnknown = true
			notes = appendNote(notes, "route group origin unknown: the router passed to "+
				h.decl.Name.Name+" could not be traced to a group — path prefix and middleware chain (including auth) are incomplete")
		}
	} else {
		group.originUnknown = true
		notes = appendNote(notes, "route group origin unknown: "+h.decl.Name.Name+
			" was called without a resolvable router argument")
	}
	if group.originUnknown && len(notes) == len(w.notes) {
		notes = appendNote(notes, ginUnknownOriginNote(h))
	}

	inner := newGinWalker(h.pkg, h.astFile, w.idx, notes)
	inner.depth = w.depth + 1
	for decl := range w.stack {
		inner.stack[decl] = true
	}
	inner.stack[h.decl] = true
	if h.paramName != "" {
		inner.groups[h.paramName] = group
	}
	inner.walkBlock(h.decl.Body.List, group.prefix, group.middlewares)

	w.routes = append(w.routes, inner.routes...)
	return true
}

// appendNote appends to a note slice without aliasing the caller's backing array.
func appendNote(notes []string, note string) []string {
	out := make([]string, 0, len(notes)+1)
	out = append(out, notes...)
	return append(out, note)
}

// mwRefs pairs middleware expressions with the type information of the package
// they were written in, so a chain assembled across packages stays resolvable.
func (w *ginWalker) mwRefs(exprs []ast.Expr) []MiddlewareRef {
	return middlewareRefs(exprs, w.info)
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
func (w *ginWalker) addRoute(method string, group *ginGroup, call *ast.CallExpr) {
	if hasIgnoreDirective(w.fset, w.astFile, call.Pos()) {
		return
	}
	notes := w.notes

	pathArg, resolved := w.pathArgValue(call.Args[0])
	if !resolved {
		notes = appendNote(notes, "unresolved route path: the path argument is not a compile-time constant, "+
			"so only the group prefix is known")
	}
	fullPath := normalizeGinPath(joinGinPath(group.prefix, pathArg))

	// For gin, the handler is the last argument; middlewares are in between.
	handler := call.Args[len(call.Args)-1]

	// Any args between path and handler are inline middlewares.
	var inlineMW []ast.Expr
	if len(call.Args) > 2 {
		inlineMW = call.Args[1 : len(call.Args)-1]
	}

	if fullPath == "" {
		notes = appendNote(notes, "empty route path: the registered path is empty and no group prefix was resolved")
	}

	pos := w.fset.Position(call.Pos())
	w.routes = append(w.routes, RawRoute{
		Method:      method,
		Path:        fullPath,
		HandlerExpr: handler,
		Middlewares: concatMiddleware(group.middlewares, w.mwRefs(inlineMW)),
		File:        w.file,
		Line:        pos.Line,
		Unresolved:  notes,
	})
}

// pathArgValue resolves a route or group path argument to its compile-time
// string value. A path built at run time cannot be documented as if it were
// literal, so an unresolvable argument is reported rather than silently
// treated as the empty string.
func (w *ginWalker) pathArgValue(expr ast.Expr) (string, bool) {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return stringLitValue(lit), true
	}
	if w.info != nil {
		if tv, ok := w.info.Types[expr]; ok && tv.Value != nil && tv.Value.Kind() == constant.String {
			return constant.StringVal(tv.Value), true
		}
	}
	return "", false
}

// joinGinPath joins a group prefix with a registered path the way gin's own
// joinPaths does — in particular a trailing slash on the registered path is
// preserved, so `router.POST("")` and `router.POST("/")` on the same group are
// two distinct routes rather than one silently deduplicated collision.
func joinGinPath(prefix, suffix string) string {
	if suffix == "" {
		return prefix
	}
	if prefix == "" {
		return suffix
	}
	joined := path.Join(prefix, suffix)
	if strings.HasSuffix(suffix, "/") && !strings.HasSuffix(joined, "/") {
		return joined + "/"
	}
	return joined
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
