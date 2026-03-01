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

// findFuncDecl searches all packages for the ast.FuncDecl that declares the
// given types.Object (which should be a *types.Func).
func findFuncDecl(obj types.Object, pkgs []*packages.Package) (*ast.FuncDecl, *ast.FuncLit, error) {
	fn, ok := obj.(*types.Func)
	if !ok {
		return nil, nil, fmt.Errorf("resolved object %q is %T, not a function", obj.Name(), obj)
	}

	// Find the package that contains this function.
	fnPkg := fn.Pkg()
	if fnPkg == nil {
		return nil, nil, fmt.Errorf("function %q has no package", fn.Name())
	}

	// Search through all loaded packages (including dependencies) for the
	// matching package and then find the FuncDecl by position.
	var targetPkg *packages.Package
	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		if pkg.Types == fnPkg {
			targetPkg = pkg
			return false
		}
		return true
	}, nil)

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
