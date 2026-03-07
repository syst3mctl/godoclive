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
func (e *EchoExtractor) Extract(pkgs []*packages.Package) ([]RawRoute, error) {
	var routes []RawRoute

	for _, pkg := range pkgs {
		if !isEchoPackage(pkg) {
			continue
		}
		for _, file := range pkg.Syntax {
			fpath := pkg.Fset.Position(file.Pos()).Filename
			w := &echoWalker{
				fset:       pkg.Fset,
				file:       fpath,
				info:       pkg.TypesInfo,
				routerVars: make(map[string]bool),
				groups:     make(map[string]*echoGroup),
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				if fn.Name.Name == "main" || fn.Name.Name == "init" || usesEcho(fn, pkg.TypesInfo) {
					w.walkBlock(fn.Body.List, "", nil)
				}
			}
			routes = append(routes, w.routes...)
		}
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

// isEchoType checks if a types.Type is *echo.Echo or echo.Echo.
func isEchoType(t types.Type) bool {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil &&
		(obj.Pkg().Path() == "github.com/labstack/echo/v4" ||
			obj.Pkg().Path() == "github.com/labstack/echo") &&
		obj.Name() == "Echo"
}

// usesEcho returns true if a FuncDecl has a param or return type involving echo.Echo.
func usesEcho(fn *ast.FuncDecl, info *types.Info) bool {
	if fn.Type == nil || info == nil {
		return false
	}
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			t := info.TypeOf(field.Type)
			if t != nil && isEchoType(t) {
				return true
			}
		}
	}
	if fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			t := info.TypeOf(field.Type)
			if t != nil && isEchoType(t) {
				return true
			}
		}
	}
	return false
}

type echoGroup struct {
	prefix string
	mw     []ast.Expr
}

type echoWalker struct {
	fset       *token.FileSet
	file       string
	info       *types.Info
	routes     []RawRoute
	routerVars map[string]bool       // variable is echo.Echo instance
	groups     map[string]*echoGroup // variable name → group state
}

// walkBlock walks a list of statements looking for Echo route registrations.
func (w *echoWalker) walkBlock(stmts []ast.Stmt, prefix string, parentMW []ast.Expr) {
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
		}
	}
}

// handleAssign detects echo.New() and e.Group() / g.Group() calls.
func (w *echoWalker) handleAssign(assign *ast.AssignStmt, currentPrefix string, parentMW []ast.Expr) {
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
		// e := echo.New()
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "echo" {
			w.routerVars[lhs.Name] = true
		}

	case "Group":
		// g := e.Group("/prefix") or g2 := g.Group("/sub")
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

		if w.routerVars[recvIdent.Name] {
			parentPrefix = currentPrefix
			inheritedMW = copyExprs(parentMW)
		} else if g, ok := w.groups[recvIdent.Name]; ok {
			parentPrefix = g.prefix
			inheritedMW = copyExprs(g.mw)
		} else {
			return
		}

		// Middleware passed as extra args to Group() (Echo supports this).
		var groupMW []ast.Expr
		if len(call.Args) > 1 {
			groupMW = copyExprs(call.Args[1:])
		}

		w.groups[lhs.Name] = &echoGroup{
			prefix: joinPath(parentPrefix, subPath),
			mw:     append(inheritedMW, groupMW...),
		}
	}
}

// processCall handles Use and route registration calls on echo.Echo or echo.Group.
func (w *echoWalker) processCall(call *ast.CallExpr, prefix string, scopeMW *[]ast.Expr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	name := sel.Sel.Name

	recvIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}

	// Determine receiver type and associated prefix/middleware.
	var callPrefix string
	var callMW []ast.Expr
	isRoot := w.routerVars[recvIdent.Name]
	grp, isGroup := w.groups[recvIdent.Name]

	if !isRoot && !isGroup {
		return
	}
	if isRoot {
		callPrefix = prefix
		callMW = *scopeMW
	} else {
		callPrefix = grp.prefix
		callMW = grp.mw
	}

	switch name {
	case "Use":
		if isRoot {
			*scopeMW = append(*scopeMW, call.Args...)
		} else {
			grp.mw = append(grp.mw, call.Args...)
		}
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "CONNECT", "TRACE":
		if len(call.Args) >= 2 {
			w.addRoute(call, callPrefix, callMW, name)
		}
	case "Any":
		if len(call.Args) >= 2 {
			w.addRoute(call, callPrefix, callMW, "ANY")
		}
	}
}

// addRoute records a route from an Echo registration call.
func (w *echoWalker) addRoute(call *ast.CallExpr, prefix string, middlewares []ast.Expr, method string) {
	patternArg := stringLitValue(call.Args[0])
	fullPath := NormalizeEchoPath(joinPath(prefix, patternArg))
	handler := call.Args[1]

	pos := w.fset.Position(call.Pos())
	w.routes = append(w.routes, RawRoute{
		Method:      method,
		Path:        fullPath,
		HandlerExpr: handler,
		Middlewares: copyExprs(middlewares),
		File:        w.file,
		Line:        pos.Line,
	})
}
