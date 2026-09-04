package extractor

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"

	"golang.org/x/tools/go/packages"
)

var fiberNamedParam = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)\??`)
var fiberWildcard = regexp.MustCompile(`\*`)

// NormalizeFiberPath converts Fiber-style :param and :param? to {param}, * to {wildcard}.
func NormalizeFiberPath(path string) string {
	path = fiberNamedParam.ReplaceAllString(path, "{$1}")
	path = fiberWildcard.ReplaceAllString(path, "{wildcard}")
	return path
}

// fiberMethods maps Fiber router method names to HTTP methods.
var fiberMethods = map[string]string{
	"Get":     "GET",
	"Post":    "POST",
	"Put":     "PUT",
	"Delete":  "DELETE",
	"Patch":   "PATCH",
	"Head":    "HEAD",
	"Options": "OPTIONS",
	"All":     "ANY",
	"Connect": "CONNECT",
	"Trace":   "TRACE",
}

// FiberExtractor extracts routes from gofiber/fiber/v2 router registrations.
type FiberExtractor struct{}

// Extract walks all packages and extracts Fiber route registrations.
//
// Functions that own an App are walked first, with every call to a registration
// function expanded inline so the group prefix and middleware chain at the call
// site flow into the routes that function registers. A second pass picks up
// registration functions no call site reached, so their routes still appear,
// carrying the caveat that their prefix is unknown.
func (e *FiberExtractor) Extract(pkgs []*packages.Package) ([]RawRoute, error) {
	idx := buildRouterIndex(pkgs, routerIndexSpec{
		inScope:   isFiberPackage,
		registers: registersFiberRoutes,
		isRouter:  isFiberRouterType,
	})

	var routes []RawRoute
	for _, h := range idx.ordered {
		if h.paramIdx >= 0 {
			continue // registers on a router it is handed: reached via call sites
		}
		w := newFiberWalker(h.pkg, h.astFile, idx, nil)
		w.walkBlock(h.decl.Body.List, "", nil)
		routes = append(routes, w.routes...)
	}

	for _, h := range idx.ordered {
		if h.paramIdx < 0 || idx.reached[h.decl] {
			continue
		}
		w := newFiberWalker(h.pkg, h.astFile, idx, []string{unknownOriginNote(h.decl.Name.Name)})
		w.walkBlock(h.decl.Body.List, "", nil)
		routes = append(routes, w.routes...)
	}

	return routes, nil
}

// isFiberPackage returns true if the package imports gofiber/fiber/v2.
func isFiberPackage(pkg *packages.Package) bool {
	for imp := range pkg.Imports {
		if imp == "github.com/gofiber/fiber/v2" {
			return true
		}
	}
	return false
}

// fiberPkgPath is the import path of fiber v2.
const fiberPkgPath = "github.com/gofiber/fiber/v2"

// fiberNamed returns the name of a named fiber type, or "" for anything else.
func fiberNamed(t types.Type) string {
	if t == nil {
		return ""
	}
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return ""
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != fiberPkgPath {
		return ""
	}
	return obj.Name()
}

// isFiberRouterType reports whether a type can have routes registered on it:
// the App itself, or a group carved out of it. App.Group returns the
// fiber.Router interface, so a group variable's static type is usually that.
func isFiberRouterType(t types.Type) bool {
	switch fiberNamed(t) {
	case "App", "Group", "Router":
		return true
	}
	return false
}

// isFiberReceiver reports whether the receiver of a method call is an App or a
// group, e.g. the app in app.Get("/x", h).
func isFiberReceiver(sel *ast.SelectorExpr, info *types.Info) bool {
	if info == nil {
		return false
	}
	// Resolving the selection first also covers a router reached through an
	// embedded field (type Server struct{ *fiber.App }).
	if receiverInPackage(sel, info, func(p string) bool { return p == fiberPkgPath }) {
		return true
	}
	return isFiberRouterType(info.TypeOf(sel.X))
}

// registersFiberRoutes reports whether a function body calls a Fiber
// registration method on an App or group value. Gating on the body rather than
// on the signature covers the shapes real projects use to set routes up outside
// main() — a factory returning http.Handler, a method on a server struct —
// none of which name a fiber type in their signature.
func registersFiberRoutes(fn *ast.FuncDecl, info *types.Info) bool {
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
		name := sel.Sel.Name
		if fiberMethods[name] == "" && name != "Use" && name != "Group" {
			return true
		}
		if isFiberReceiver(sel, info) {
			found = true
			return false
		}
		return true
	})
	return found
}

type fiberGroup struct {
	prefix string
	mw     []MiddlewareRef
}

type fiberWalker struct {
	fset    *token.FileSet
	astFile *ast.File
	file    string
	info    *types.Info
	idx     *routerIndex
	notes   []string
	depth   int
	stack   map[*ast.FuncDecl]bool
	routes  []RawRoute
	groups  map[string]*fiberGroup // variable name → group state
}

func newFiberWalker(pkg *packages.Package, file *ast.File, idx *routerIndex, notes []string) *fiberWalker {
	return &fiberWalker{
		fset:    pkg.Fset,
		astFile: file,
		file:    pkg.Fset.Position(file.Pos()).Filename,
		info:    pkg.TypesInfo,
		idx:     idx,
		notes:   notes,
		stack:   make(map[*ast.FuncDecl]bool),
		groups:  make(map[string]*fiberGroup),
	}
}

// walkBlock walks a list of statements looking for Fiber route registrations.
func (w *fiberWalker) walkBlock(stmts []ast.Stmt, prefix string, parentMW []MiddlewareRef) {
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
			if !w.processCall(call, prefix, &scopeMW) {
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

// expandRegistrationCall handles a plain call to a function that registers
// routes on a router it is handed, e.g. routes.Register(app).
func (w *fiberWalker) expandRegistrationCall(call *ast.CallExpr, prefix string, scopeMW []MiddlewareRef) {
	h := w.idx.lookup(w.info, call.Fun)
	if h == nil || h.paramIdx < 0 || h.paramIdx >= len(call.Args) {
		return
	}
	arg := call.Args[h.paramIdx]
	if !isFiberRouterType(w.info.TypeOf(arg)) {
		return
	}
	// A group handed to the registrar carries its own prefix and middleware.
	argPrefix, argMW := prefix, scopeMW
	if ident, ok := arg.(*ast.Ident); ok {
		if g, ok := w.groups[ident.Name]; ok {
			argPrefix, argMW = g.prefix, g.mw
		}
	}
	w.expandInto(h, argPrefix, argMW)
}

// expandInto walks a registration function's body under the given prefix and
// middleware chain, folding the routes it finds into this walker.
func (w *fiberWalker) expandInto(h *routerHelper, prefix string, parentMW []MiddlewareRef) {
	if h == nil {
		return
	}
	w.idx.reached[h.decl] = true

	if w.depth >= maxHelperDepth || w.stack[h.decl] {
		return // recursion or runaway nesting: stop, but stay marked as reached
	}

	inner := newFiberWalker(h.pkg, h.astFile, w.idx, w.notes)
	inner.depth = w.depth + 1
	for decl := range w.stack {
		inner.stack[decl] = true
	}
	inner.stack[h.decl] = true
	inner.walkBlock(h.decl.Body.List, prefix, parentMW)

	w.routes = append(w.routes, inner.routes...)
}

// mwRefs pairs middleware expressions with the type information of the package
// they were written in, so a chain assembled across packages stays resolvable.
func (w *fiberWalker) mwRefs(exprs []ast.Expr) []MiddlewareRef {
	return middlewareRefs(exprs, w.info)
}

// handleAssign detects app.Group() / g.Group() so a group variable carries its
// prefix and middleware to the registrations made on it.
func (w *fiberWalker) handleAssign(assign *ast.AssignStmt, currentPrefix string, parentMW []MiddlewareRef) {
	if len(assign.Lhs) == 0 || len(assign.Rhs) == 0 {
		return
	}
	lhs, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
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
	if !isFiberReceiver(sel, w.info) {
		return
	}

	// A group carved out of another group inherits that group's prefix; one
	// carved out of the App inherits the enclosing scope's.
	parentPrefix := currentPrefix
	inheritedMW := copyMiddleware(parentMW)
	if recvIdent, ok := sel.X.(*ast.Ident); ok {
		if g, ok := w.groups[recvIdent.Name]; ok {
			parentPrefix = g.prefix
			inheritedMW = copyMiddleware(g.mw)
		}
	}

	// Variadic middlewares passed directly to Group().
	var groupMW []MiddlewareRef
	if len(call.Args) > 1 {
		groupMW = w.mwRefs(call.Args[1:])
	}

	w.groups[lhs.Name] = &fiberGroup{
		prefix: joinPath(parentPrefix, stringLitValue(call.Args[0])),
		mw:     append(inheritedMW, groupMW...),
	}
}

// processCall handles Use and route registration calls on *fiber.App or a
// group. It reports whether the call was a registration.
func (w *fiberWalker) processCall(call *ast.CallExpr, prefix string, scopeMW *[]MiddlewareRef) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	// The receiver is identified by its type rather than by tracking which
	// local variables were assigned from fiber.New(). Name tracking only ever
	// saw an App built in the same function, so an App arriving as a parameter
	// — the shape of every routes.Register(app) helper — or held in a struct
	// field was invisible.
	if !isFiberReceiver(sel, w.info) {
		return false
	}
	name := sel.Sel.Name

	// A known group variable brings its own prefix and chain; anything else
	// registers at the prefix this walk was entered with.
	callPrefix, callMW := prefix, *scopeMW
	var grp *fiberGroup
	if recvIdent, ok := sel.X.(*ast.Ident); ok {
		if g, ok := w.groups[recvIdent.Name]; ok {
			grp, callPrefix, callMW = g, g.prefix, g.mw
		}
	}

	switch {
	case name == "Use":
		if grp != nil {
			grp.mw = append(grp.mw, w.mwRefs(call.Args)...)
		} else {
			*scopeMW = append(*scopeMW, w.mwRefs(call.Args)...)
		}

	case fiberMethods[name] != "" && len(call.Args) >= 2:
		w.addRoute(call, callPrefix, callMW, fiberMethods[name])

	default:
		return false
	}
	return true
}

// addRoute records a route from a Fiber registration call.
// Fiber routes are variadic: app.Get(path, mw1, mw2, handler).
// The handler is always the last arg; preceding args are inline middlewares.
func (w *fiberWalker) addRoute(call *ast.CallExpr, prefix string, middlewares []MiddlewareRef, method string) {
	if hasIgnoreDirective(w.fset, w.astFile, call.Pos(), call.End()) {
		return
	}
	patternArg := stringLitValue(call.Args[0])
	fullPath := NormalizeFiberPath(joinPath(prefix, patternArg))

	// Last arg is the handler; args between path and handler are inline middlewares.
	handler := call.Args[len(call.Args)-1]
	var inlineMW []ast.Expr
	if len(call.Args) > 2 {
		inlineMW = copyExprs(call.Args[1 : len(call.Args)-1])
	}

	allMW := concatMiddleware(middlewares, w.mwRefs(inlineMW))

	pos := w.fset.Position(call.Pos())
	w.routes = append(w.routes, RawRoute{
		Method:      method,
		Path:        fullPath,
		HandlerExpr: handler,
		Middlewares: allMW,
		File:        w.file,
		Line:        pos.Line,
		Unresolved:  w.notes,
	})
}
