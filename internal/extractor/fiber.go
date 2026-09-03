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
func (e *FiberExtractor) Extract(pkgs []*packages.Package) ([]RawRoute, error) {
	var routes []RawRoute

	for _, pkg := range pkgs {
		if !isFiberPackage(pkg) {
			continue
		}
		for _, file := range pkg.Syntax {
			fpath := pkg.Fset.Position(file.Pos()).Filename
			w := &fiberWalker{
				fset:    pkg.Fset,
				astFile: file,
				file:    fpath,
				info:    pkg.TypesInfo,
				appVars: make(map[string]bool),
				groups:  make(map[string]*fiberGroup),
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				if fn.Name.Name == "main" || fn.Name.Name == "init" || usesFiberApp(fn, pkg.TypesInfo) {
					w.walkBlock(fn.Body.List, "", nil)
				}
			}
			routes = append(routes, w.routes...)
		}
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

// isFiberAppType checks if a types.Type is *fiber.App or fiber.App.
func isFiberAppType(t types.Type) bool {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil &&
		obj.Pkg().Path() == "github.com/gofiber/fiber/v2" &&
		obj.Name() == "App"
}

// usesFiberApp returns true if a FuncDecl has a param or return type involving *fiber.App.
func usesFiberApp(fn *ast.FuncDecl, info *types.Info) bool {
	if fn.Type == nil || info == nil {
		return false
	}
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			t := info.TypeOf(field.Type)
			if t != nil && isFiberAppType(t) {
				return true
			}
		}
	}
	if fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			t := info.TypeOf(field.Type)
			if t != nil && isFiberAppType(t) {
				return true
			}
		}
	}
	return false
}

type fiberGroup struct {
	prefix string
	mw     []ast.Expr
}

type fiberWalker struct {
	fset    *token.FileSet
	astFile *ast.File
	file    string
	info    *types.Info
	routes  []RawRoute
	appVars map[string]bool        // variable is *fiber.App instance
	groups  map[string]*fiberGroup // variable name → group state
}

// walkBlock walks a list of statements looking for Fiber route registrations.
func (w *fiberWalker) walkBlock(stmts []ast.Stmt, prefix string, parentMW []ast.Expr) {
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
			w.processCall(call, prefix, &scopeMW)
		default:
			// Routes may be registered conditionally; descend into nested blocks.
			for _, body := range nestedStmtBodies(stmt) {
				w.walkBlock(body, prefix, scopeMW)
			}
		}
	}
}

// handleAssign detects fiber.New() and app.Group() / g.Group() calls.
func (w *fiberWalker) handleAssign(assign *ast.AssignStmt, currentPrefix string, parentMW []ast.Expr) {
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
	if !ok {
		return
	}

	switch sel.Sel.Name {
	case "New":
		// app := fiber.New()
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "fiber" {
			w.appVars[lhs.Name] = true
		}

	case "Group":
		// g := app.Group("/prefix", mw...) or g2 := g.Group("/sub")
		if len(call.Args) < 1 {
			return
		}
		subPath := stringLitValue(call.Args[0])
		recvIdent, ok := sel.X.(*ast.Ident)
		if !ok {
			return
		}

		var parentPrefix string
		var inheritedMW []ast.Expr

		if w.appVars[recvIdent.Name] {
			parentPrefix = currentPrefix
			inheritedMW = copyExprs(parentMW)
		} else if g, ok := w.groups[recvIdent.Name]; ok {
			parentPrefix = g.prefix
			inheritedMW = copyExprs(g.mw)
		} else {
			return
		}

		// Variadic middlewares passed directly to Group().
		var groupMW []ast.Expr
		if len(call.Args) > 1 {
			groupMW = copyExprs(call.Args[1:])
		}

		w.groups[lhs.Name] = &fiberGroup{
			prefix: joinPath(parentPrefix, subPath),
			mw:     append(inheritedMW, groupMW...),
		}
	}
}

// processCall handles Use and route registration calls on *fiber.App or *fiber.Group.
func (w *fiberWalker) processCall(call *ast.CallExpr, prefix string, scopeMW *[]ast.Expr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	name := sel.Sel.Name

	recvIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}

	isApp := w.appVars[recvIdent.Name]
	grp, isGroup := w.groups[recvIdent.Name]

	if !isApp && !isGroup {
		return
	}

	var callPrefix string
	var callMW []ast.Expr
	if isApp {
		callPrefix = prefix
		callMW = *scopeMW
	} else {
		callPrefix = grp.prefix
		callMW = grp.mw
	}

	switch name {
	case "Use":
		if isApp {
			*scopeMW = append(*scopeMW, call.Args...)
		} else {
			grp.mw = append(grp.mw, call.Args...)
		}

	default:
		if method, ok := fiberMethods[name]; ok && len(call.Args) >= 2 {
			w.addRoute(call, callPrefix, callMW, method)
		}
	}
}

// addRoute records a route from a Fiber registration call.
// Fiber routes are variadic: app.Get(path, mw1, mw2, handler).
// The handler is always the last arg; preceding args are inline middlewares.
func (w *fiberWalker) addRoute(call *ast.CallExpr, prefix string, middlewares []ast.Expr, method string) {
	if hasIgnoreDirective(w.fset, w.astFile, call.Pos()) {
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

	allMW := concatExprs(middlewares, inlineMW)

	pos := w.fset.Position(call.Pos())
	w.routes = append(w.routes, RawRoute{
		Method:      method,
		Path:        fullPath,
		HandlerExpr: handler,
		Middlewares: middlewareRefs(allMW, w.info),
		File:        w.file,
		Line:        pos.Line,
	})
}
