package extractor

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/packages"
)

var echoParamRegex = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)

// NormalizeEchoPath converts Echo-style :param segments to {param}.
func NormalizeEchoPath(path string) string {
	return echoParamRegex.ReplaceAllString(path, "{$1}")
}

// EchoExtractor extracts routes from Echo v4 router registrations.
type EchoExtractor struct{}

// Extract walks all packages and extracts Echo route registrations.
//
// Functions that own an Echo instance are walked first, with every call to a
// registration function expanded inline so the group prefix and middleware
// chain at the call site flow into the routes that function registers. A second
// pass picks up registration functions no call site reached, so their routes
// still appear, carrying the caveat that their prefix is unknown.
func (e *EchoExtractor) Extract(pkgs []*packages.Package) ([]RawRoute, error) {
	idx := buildRouterIndex(pkgs, routerIndexSpec{
		inScope:   isEchoPackage,
		registers: registersEchoRoutes,
		isRouter:  isEchoRouterType,
	})

	var routes []RawRoute
	for _, h := range idx.ordered {
		if h.paramIdx >= 0 {
			continue // registers on a router it is handed: reached via call sites
		}
		w := newEchoWalker(h.pkg, h.astFile, idx, nil)
		w.walkBlock(h.decl.Body.List, "", nil)
		routes = append(routes, w.routes...)
	}

	for _, h := range idx.ordered {
		if h.paramIdx < 0 || idx.reached[h.decl] {
			continue
		}
		w := newEchoWalker(h.pkg, h.astFile, idx, []string{unknownOriginNote(h.decl.Name.Name)})
		w.walkBlock(h.decl.Body.List, "", nil)
		routes = append(routes, w.routes...)
	}

	return routes, nil
}

// isEchoPackage returns true if the package imports echo.
func isEchoPackage(pkg *packages.Package) bool {
	for imp := range pkg.Imports {
		if imp == "github.com/labstack/echo/v4" ||
			strings.HasPrefix(imp, "github.com/labstack/echo/v4/") ||
			imp == "github.com/labstack/echo" {
			return true
		}
	}
	return false
}

// isEchoPkgPath reports whether an import path is the echo root package, which
// is where Echo and Group are declared.
func isEchoPkgPath(p string) bool {
	return p == "github.com/labstack/echo/v4" || p == "github.com/labstack/echo"
}

// echoNamed returns the name of a named echo type, or "" for anything else.
func echoNamed(t types.Type) string {
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
	if obj == nil || obj.Pkg() == nil || !isEchoPkgPath(obj.Pkg().Path()) {
		return ""
	}
	return obj.Name()
}

// isEchoRouterType reports whether a type can have routes registered on it:
// the Echo instance itself, or a group carved out of it.
func isEchoRouterType(t types.Type) bool {
	switch echoNamed(t) {
	case "Echo", "Group":
		return true
	}
	return false
}

// isEchoReceiver reports whether the receiver of a method call is an Echo
// instance or a group, e.g. the e in e.GET("/x", h).
func isEchoReceiver(sel *ast.SelectorExpr, info *types.Info) bool {
	if info == nil {
		return false
	}
	// Resolving the selection first also covers a router reached through an
	// embedded field (type Server struct{ *echo.Echo }).
	if receiverInPackage(sel, info, isEchoPkgPath) {
		return true
	}
	return isEchoRouterType(info.TypeOf(sel.X))
}

// echoRegistrationMethods are the Echo methods that register a route or
// structure the router.
var echoRegistrationMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true,
	"HEAD": true, "OPTIONS": true, "CONNECT": true, "TRACE": true,
	"Any": true, "Use": true, "Group": true,
}

// registersEchoRoutes reports whether a function body calls an Echo
// registration method on an Echo or Group value. Gating on the body rather than
// on the signature covers the shapes real projects use to set routes up outside
// main() — a factory returning http.Handler, a method on a server struct —
// none of which name an echo type in their signature.
func registersEchoRoutes(fn *ast.FuncDecl, info *types.Info) bool {
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
		if !echoRegistrationMethods[sel.Sel.Name] {
			return true
		}
		if isEchoReceiver(sel, info) {
			found = true
			return false
		}
		return true
	})
	return found
}

type echoGroup struct {
	prefix string
	mw     []MiddlewareRef
}

type echoWalker struct {
	fset    *token.FileSet
	astFile *ast.File
	file    string
	info    *types.Info
	idx     *routerIndex
	notes   []string
	depth   int
	stack   map[*ast.FuncDecl]bool
	routes  []RawRoute
	groups  map[string]*echoGroup // variable name → group state
}

func newEchoWalker(pkg *packages.Package, file *ast.File, idx *routerIndex, notes []string) *echoWalker {
	return &echoWalker{
		fset:    pkg.Fset,
		astFile: file,
		file:    pkg.Fset.Position(file.Pos()).Filename,
		info:    pkg.TypesInfo,
		idx:     idx,
		notes:   notes,
		stack:   make(map[*ast.FuncDecl]bool),
		groups:  make(map[string]*echoGroup),
	}
}

// walkBlock walks a list of statements looking for Echo route registrations.
func (w *echoWalker) walkBlock(stmts []ast.Stmt, prefix string, parentMW []MiddlewareRef) {
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
// routes on a router it is handed, e.g. routes.Register(e).
func (w *echoWalker) expandRegistrationCall(call *ast.CallExpr, prefix string, scopeMW []MiddlewareRef) {
	h := w.idx.lookup(w.info, call.Fun)
	if h == nil || h.paramIdx < 0 || h.paramIdx >= len(call.Args) {
		return
	}
	arg := call.Args[h.paramIdx]
	if !isEchoRouterType(w.info.TypeOf(arg)) {
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
func (w *echoWalker) expandInto(h *routerHelper, prefix string, parentMW []MiddlewareRef) {
	if h == nil {
		return
	}
	w.idx.reached[h.decl] = true

	if w.depth >= maxHelperDepth || w.stack[h.decl] {
		return // recursion or runaway nesting: stop, but stay marked as reached
	}

	inner := newEchoWalker(h.pkg, h.astFile, w.idx, w.notes)
	inner.depth = w.depth + 1
	for decl := range w.stack {
		inner.stack[decl] = true
	}
	inner.stack[h.decl] = true
	inner.walkBlock(h.decl.Body.List, prefix, parentMW)

	w.routes = append(w.routes, inner.routes...)
}

// handleAssign detects e.Group() / g.Group() so a group variable carries its
// prefix and middleware to the registrations made on it.
func (w *echoWalker) handleAssign(assign *ast.AssignStmt, currentPrefix string, parentMW []MiddlewareRef) {
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
	if !isEchoReceiver(sel, w.info) {
		return
	}

	// A group carved out of another group inherits that group's prefix; one
	// carved out of the Echo instance inherits the enclosing scope's.
	parentPrefix := currentPrefix
	inheritedMW := copyMiddleware(parentMW)
	if recvIdent, ok := sel.X.(*ast.Ident); ok {
		if g, ok := w.groups[recvIdent.Name]; ok {
			parentPrefix = g.prefix
			inheritedMW = copyMiddleware(g.mw)
		}
	}

	// Middleware passed as extra args to Group() (Echo supports this).
	var groupMW []MiddlewareRef
	if len(call.Args) > 1 {
		groupMW = w.mwRefs(call.Args[1:])
	}

	w.groups[lhs.Name] = &echoGroup{
		prefix: joinPath(parentPrefix, stringLitValue(call.Args[0])),
		mw:     append(inheritedMW, groupMW...),
	}
}

// mwRefs pairs middleware expressions with the type information of the package
// they were written in, so a chain assembled across packages stays resolvable.
func (w *echoWalker) mwRefs(exprs []ast.Expr) []MiddlewareRef {
	return middlewareRefs(exprs, w.info)
}

// processCall handles Use and route registration calls on echo.Echo or
// echo.Group. It reports whether the call was a registration.
func (w *echoWalker) processCall(call *ast.CallExpr, prefix string, scopeMW *[]MiddlewareRef) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	// The receiver is identified by its type rather than by tracking which
	// local variables were assigned from echo.New(). Name tracking only ever
	// saw an instance built in the same function, so an Echo arriving as a
	// parameter — the shape of every routes.Register(e) helper — or held in a
	// struct field was invisible.
	if !isEchoReceiver(sel, w.info) {
		return false
	}
	name := sel.Sel.Name

	// A known group variable brings its own prefix and chain; anything else
	// registers at the prefix this walk was entered with.
	callPrefix, callMW := prefix, *scopeMW
	var grp *echoGroup
	if recvIdent, ok := sel.X.(*ast.Ident); ok {
		if g, ok := w.groups[recvIdent.Name]; ok {
			grp, callPrefix, callMW = g, g.prefix, g.mw
		}
	}

	switch name {
	case "Use":
		if grp != nil {
			grp.mw = append(grp.mw, w.mwRefs(call.Args)...)
		} else {
			*scopeMW = append(*scopeMW, w.mwRefs(call.Args)...)
		}
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "CONNECT", "TRACE":
		if len(call.Args) < 2 {
			return false
		}
		w.addRoute(call, callPrefix, callMW, name)
	case "Any":
		if len(call.Args) < 2 {
			return false
		}
		w.addRoute(call, callPrefix, callMW, "ANY")
	default:
		return false
	}
	return true
}

// addRoute records a route from an Echo registration call.
func (w *echoWalker) addRoute(call *ast.CallExpr, prefix string, middlewares []MiddlewareRef, method string) {
	if hasIgnoreDirective(w.fset, w.astFile, call.Pos(), call.End()) {
		return
	}
	patternArg := stringLitValue(call.Args[0])
	fullPath := NormalizeEchoPath(joinPath(prefix, patternArg))
	handler := call.Args[1]

	pos := w.fset.Position(call.Pos())
	w.routes = append(w.routes, RawRoute{
		Method:      method,
		Path:        fullPath,
		HandlerExpr: handler,
		Middlewares: middlewares,
		File:        w.file,
		Line:        pos.Line,
		Unresolved:  w.notes,
	})
}
