package extractor

import (
	"go/ast"
	"go/token"
	"go/types"
)

// paramBinding ties a registration function's parameters to the arguments one
// call site passed for them.
//
// A team that wraps its router — a house type holding a chi.Mux or a ServeMux
// and exposing its own Handle, GET and POST — moves the path and the handler
// out of the registration call and into the wrapper's parameters:
//
//	func (r *Router) Handle(pattern string, h http.HandlerFunc) {
//	    r.mux.HandleFunc(pattern, h)
//	}
//
// Walking that body alone finds a registration whose path is an identifier and
// whose handler is a parameter, and neither resolves to anything. Every route
// in such a service disappears. Binding the call site's arguments to the
// wrapper's parameters and walking the body once per call site is what makes
// them resolve — the same trick the index already plays for a router handed to
// a registration function, extended to the values it is handed alongside.
//
// The bound expressions belong to the caller's package, so the binding carries
// the caller's type information and source position with them.
type paramBinding struct {
	values map[types.Object]ast.Expr
	info   *types.Info // type info for the bound expressions
	file   string      // call site, which is where the route is really registered
	line   int
}

// bindCallArgs pairs a function's parameters with the arguments a call supplied.
// It returns nil when the two do not line up, which is the honest answer for a
// variadic spread or a call the type checker could not resolve.
func bindCallArgs(h *routerHelper, call *ast.CallExpr, fset *token.FileSet, callerInfo *types.Info) *paramBinding {
	if h == nil || h.decl == nil || h.decl.Type == nil || h.decl.Type.Params == nil {
		return nil
	}
	if callerInfo == nil || call.Ellipsis.IsValid() {
		return nil
	}
	calleeInfo := h.pkg.TypesInfo
	if calleeInfo == nil {
		return nil
	}

	values := make(map[types.Object]ast.Expr)
	pos := 0
	for _, field := range h.decl.Type.Params.List {
		if len(field.Names) == 0 {
			pos++ // an unnamed parameter cannot be referred to; skip its slot
			continue
		}
		for _, name := range field.Names {
			if pos >= len(call.Args) {
				return nil // fewer arguments than parameters: not this call
			}
			if obj, ok := calleeInfo.Defs[name]; ok && obj != nil {
				values[obj] = call.Args[pos]
			}
			pos++
		}
	}
	if len(values) == 0 {
		return nil
	}

	position := fset.Position(call.Pos())
	return &paramBinding{
		values: values,
		info:   callerInfo,
		file:   position.Filename,
		line:   position.Line,
	}
}

// resolve returns the expression a bound parameter stands for, together with
// the type information that expression belongs to.
func (b *paramBinding) resolve(info *types.Info, expr ast.Expr) (ast.Expr, *types.Info, bool) {
	if b == nil || info == nil {
		return nil, nil, false
	}
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return nil, nil, false
	}
	obj, ok := info.Uses[ident]
	if !ok || obj == nil {
		return nil, nil, false
	}
	arg, ok := b.values[obj]
	if !ok {
		return nil, nil, false
	}
	return arg, b.info, true
}

// boundString reads a string constant from expr, following a bound parameter
// when expr is one. A path spelled as a literal in the wrapper's own body still
// resolves, so a wrapper that fixes part of the path keeps working.
func boundString(info *types.Info, b *paramBinding, expr ast.Expr) string {
	if s := stringLitValue(expr); s != "" {
		return s
	}
	if arg, _, ok := b.resolve(info, expr); ok {
		return stringLitValue(arg)
	}
	return ""
}

// boundExpr returns the expression to record for a handler or middleware,
// following a bound parameter, along with the type information it belongs to.
func boundExpr(info *types.Info, b *paramBinding, expr ast.Expr) (ast.Expr, *types.Info, bool) {
	if arg, argInfo, ok := b.resolve(info, expr); ok {
		return arg, argInfo, true
	}
	return expr, info, false
}

// site returns the call site a binding came from.
func (b *paramBinding) site() (string, int, bool) {
	if b == nil {
		return "", 0, false
	}
	return b.file, b.line, true
}

// canWrap reports whether a call to an indexed helper is worth expanding as a
// router wrapper: it registers routes without being handed a router, and it
// takes arguments a call site could bind.
//
// Helpers that take a router are already expanded through that parameter.
func canWrap(h *routerHelper, call *ast.CallExpr) bool {
	if h == nil || h.paramIdx >= 0 || len(call.Args) == 0 {
		return false
	}
	return h.decl != nil && h.decl.Type != nil &&
		h.decl.Type.Params != nil && len(h.decl.Type.Params.List) > 0
}

// paramIdent reports whether expr is an identifier naming one of decl's
// parameters.
//
// This is what separates a house router wrapper from an ordinary registration
// function. A function that spells its paths as literals resolves on its own
// and is already walked where it is declared; expanding its call sites as well
// would emit every one of its routes twice. Only a function that takes the path
// from its caller has anything to gain from being walked per call site.
func paramIdent(decl *ast.FuncDecl, info *types.Info, expr ast.Expr) bool {
	if decl == nil || decl.Type == nil || decl.Type.Params == nil || info == nil {
		return false
	}
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	obj, ok := info.Uses[ident]
	if !ok || obj == nil {
		return false
	}
	for _, field := range decl.Type.Params.List {
		for _, name := range field.Names {
			if def, ok := info.Defs[name]; ok && def == obj {
				return true
			}
		}
	}
	return false
}

// wrapsRoutes reports whether a function registers a route whose path it was
// handed as a parameter — the shape of a house router wrapper.
//
// isRegistration names the framework's registration methods and isRouter its
// router type; every supported framework puts the path first, so that is the
// argument checked.
func wrapsRoutes(fn *ast.FuncDecl, info *types.Info, isRegistration func(string) bool, isRouter func(types.Type) bool) bool {
	if fn == nil || fn.Body == nil || info == nil {
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
		if !ok || len(call.Args) < 2 || !isRegistration(sel.Sel.Name) {
			return true
		}
		if isRouter(info.TypeOf(sel.X)) && paramIdent(fn, info, call.Args[0]) {
			found = true
			return false
		}
		return true
	})
	return found
}

// applyBinding attributes a route to the call site when its handler came from a
// bound parameter.
//
// The handler expression is the caller's, not the wrapper's, so it type-checks
// only against the caller's package. Recording that type information alongside
// it — and the call site's file, which is how a route is otherwise traced back
// to a package — is what lets a handler resolve when the wrapper it was handed
// to lives somewhere else entirely.
func applyBinding(route RawRoute, b *paramBinding, handlerInfo *types.Info, substituted bool) RawRoute {
	if !substituted {
		return route
	}
	route.HandlerInfo = handlerInfo
	if file, line, ok := b.site(); ok {
		route.File, route.Line = file, line
	}
	return route
}
