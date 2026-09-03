package resolver

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// ResolveHandler takes a handler expression from a route registration and
// resolves it to the actual function declaration or function literal.
//
// It handles four cases:
//   - Direct function reference: ListUsers → *ast.FuncDecl
//   - Inline function literal: func(w, r){...} → *ast.FuncLit
//   - Method expression: h.GetUser → *ast.FuncDecl via types.Info.Selections
//   - Package-qualified: handlers.GetUser → *ast.FuncDecl via types.Info.Uses
//
// Returns (funcDecl, funcLit, error). Exactly one of funcDecl/funcLit will be
// non-nil on success. Both are nil on error.
func ResolveHandler(expr ast.Expr, info *types.Info, pkgs []*packages.Package) (*ast.FuncDecl, *ast.FuncLit, error) {
	if expr == nil {
		return nil, nil, fmt.Errorf("nil handler expression")
	}

	switch e := expr.(type) {
	case *ast.FuncLit:
		// Case (b): inline function literal — return directly.
		return nil, e, nil

	case *ast.Ident:
		// Case (a): direct function reference in the same package (e.g., ListUsers).
		return resolveIdent(e, info, pkgs)

	case *ast.SelectorExpr:
		// Case (c)/(d): method expression (h.GetUser) or package-qualified (handlers.GetUser).
		return resolveSelector(e, info, pkgs)

	case *ast.UnaryExpr:
		// Case: &SomeHandler{} — address-of composite literal implementing http.Handler.
		return resolveHandlerExpr(e, info, pkgs)

	case *ast.CompositeLit:
		// Case: SomeHandler{} — composite literal implementing http.Handler.
		return resolveHandlerExpr(e, info, pkgs)

	case *ast.CallExpr:
		// Case: SomeFunc() returning an http.Handler, or middleware wrapping.
		return resolveHandlerExpr(e, info, pkgs)

	default:
		return nil, nil, fmt.Errorf("unsupported handler expression type %T", expr)
	}
}

// resolveIdent resolves a simple identifier (e.g., ListUsers) to its FuncDecl.
func resolveIdent(ident *ast.Ident, info *types.Info, pkgs []*packages.Package) (*ast.FuncDecl, *ast.FuncLit, error) {
	// Look up the object this identifier refers to via types.Info.Uses.
	obj, ok := info.Uses[ident]
	if !ok {
		// Could be a definition rather than a use — check Defs too.
		obj, ok = info.Defs[ident]
		if !ok {
			return nil, nil, fmt.Errorf("could not resolve identifier %q", ident.Name)
		}
	}

	return findFuncDecl(obj, pkgs)
}

// resolveSelector resolves a selector expression like h.Method or pkg.Func.
func resolveSelector(sel *ast.SelectorExpr, info *types.Info, pkgs []*packages.Package) (*ast.FuncDecl, *ast.FuncLit, error) {
	// First try types.Info.Uses on the selector identifier — this works for
	// both package-qualified functions and method references.
	obj, ok := info.Uses[sel.Sel]
	if !ok {
		// Try via Selections for method expressions.
		selection, ok := info.Selections[sel]
		if !ok {
			return nil, nil, fmt.Errorf("could not resolve selector %q", sel.Sel.Name)
		}
		obj = selection.Obj()
	}

	return findFuncDecl(obj, pkgs)
}

// resolveHandlerExpr resolves a handler expression that isn't a simple ident,
// selector, or func literal. It uses type info to check if the expression's
// type implements http.Handler, and if so resolves to ServeHTTP.
func resolveHandlerExpr(expr ast.Expr, info *types.Info, pkgs []*packages.Package) (*ast.FuncDecl, *ast.FuncLit, error) {
	t := info.TypeOf(expr)
	if t == nil {
		return nil, nil, fmt.Errorf("could not determine type of handler expression %T", expr)
	}

	// Check if the type (or pointer to it) has a ServeHTTP method.
	for _, candidate := range []types.Type{t, types.NewPointer(t)} {
		mset := types.NewMethodSet(candidate)
		for i := 0; i < mset.Len(); i++ {
			sel := mset.At(i)
			if sel.Obj().Name() == "ServeHTTP" {
				if fn, ok := sel.Obj().(*types.Func); ok {
					return findFuncDeclByPos(fn, pkgs)
				}
			}
		}
	}

	return nil, nil, fmt.Errorf("handler expression %T does not implement http.Handler", expr)
}

// findFuncDecl searches all packages for the ast.FuncDecl that declares the
// given types.Object (which should be a *types.Func).
// If the object is not a function but implements http.Handler, it resolves
// to the ServeHTTP method declaration.
func findFuncDecl(obj types.Object, pkgs []*packages.Package) (*ast.FuncDecl, *ast.FuncLit, error) {
	fn, ok := obj.(*types.Func)
	if !ok {
		// Check if the object's type implements http.Handler (has ServeHTTP method).
		if serveHTTP := findServeHTTP(obj); serveHTTP != nil {
			return findFuncDeclByPos(serveHTTP, pkgs)
		}
		return nil, nil, fmt.Errorf("resolved object %q is %T, not a function", obj.Name(), obj)
	}

	// Find the package that contains this function.
	fnPkg := fn.Pkg()
	if fnPkg == nil {
		return nil, nil, fmt.Errorf("function %q has no package", fn.Name())
	}

	targetPkg := packageWithTypes(pkgs, fnPkg)

	if targetPkg == nil {
		return nil, nil, fmt.Errorf("package %q not found in loaded packages", fnPkg.Path())
	}

	// Find the FuncDecl by matching the position of the function object.
	fnPos := fn.Pos()
	for _, file := range targetPkg.Syntax {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fd.Name.Pos() == fnPos {
				return fd, nil, nil
			}
		}
	}

	return nil, nil, fmt.Errorf("FuncDecl for %q not found in AST (pos=%v)", fn.Name(), fnPos)
}

// findServeHTTP checks if an object's type implements http.Handler
// (has a ServeHTTP(http.ResponseWriter, *http.Request) method) and returns
// the *types.Func for that method if found.
func findServeHTTP(obj types.Object) *types.Func {
	t := obj.Type()
	if t == nil {
		return nil
	}

	// Try the type and its pointer variant.
	for _, candidate := range []types.Type{t, types.NewPointer(t)} {
		mset := types.NewMethodSet(candidate)
		for i := 0; i < mset.Len(); i++ {
			sel := mset.At(i)
			if sel.Obj().Name() == "ServeHTTP" {
				if fn, ok := sel.Obj().(*types.Func); ok {
					return fn
				}
			}
		}
	}
	return nil
}

// findFuncDeclByPos finds the ast.FuncDecl for a *types.Func by matching position.
func findFuncDeclByPos(fn *types.Func, pkgs []*packages.Package) (*ast.FuncDecl, *ast.FuncLit, error) {
	fnPkg := fn.Pkg()
	if fnPkg == nil {
		return nil, nil, fmt.Errorf("function %q has no package", fn.Name())
	}

	targetPkg := packageWithTypes(pkgs, fnPkg)

	if targetPkg == nil {
		return nil, nil, fmt.Errorf("package %q not found in loaded packages", fnPkg.Path())
	}

	fnPos := fn.Pos()
	for _, file := range targetPkg.Syntax {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fd.Name.Pos() == fnPos {
				return fd, nil, nil
			}
		}
	}

	return nil, nil, fmt.Errorf("FuncDecl for ServeHTTP not found in AST (pos=%v)", fnPos)
}

// packageWithTypes finds the analyzed package that was type-checked into tp.
//
// It scans the analyzed packages directly rather than calling packages.Visit
// over the whole graph. Visit allocates and sorts a key slice for every
// package it walks, on every call — at one call per route that was the single
// largest source of allocation in the analyzer — and the walk cannot succeed
// anyway: resolving a declaration needs syntax, and only the analyzed packages
// are parsed. Dependencies are type-checked from export data and have none.
func packageWithTypes(pkgs []*packages.Package, tp *types.Package) *packages.Package {
	for _, pkg := range pkgs {
		if pkg.Types == tp {
			return pkg
		}
	}
	return nil
}
