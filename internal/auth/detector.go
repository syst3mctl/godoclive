package auth

import (
	"go/ast"
	"go/types"

	"github.com/dicki/godoclive/internal/model"
	"golang.org/x/tools/go/packages"
)

// knownAuthPackages maps import paths to their inferred auth scheme.
var knownAuthPackages = map[string]model.AuthScheme{
	"github.com/golang-jwt/jwt/v5":        model.AuthBearer,
	"github.com/golang-jwt/jwt":           model.AuthBearer,
	"github.com/dgrijalva/jwt-go":         model.AuthBearer,
	"github.com/auth0/go-jwt-middleware":   model.AuthBearer,
	"github.com/auth0/go-jwt-middleware/v2": model.AuthBearer,
}

// DetectAuth examines middleware expressions to determine the authentication
// scheme(s) required by an endpoint. It resolves each middleware to its
// function body and scans for known auth patterns.
func DetectAuth(middlewares []ast.Expr, info *types.Info, pkgs []*packages.Package) model.AuthDef {
	var schemes []model.AuthScheme
	seen := make(map[model.AuthScheme]bool)

	for _, mw := range middlewares {
		detected := detectFromExpr(mw, info, pkgs)
		for _, s := range detected {
			if !seen[s] {
				seen[s] = true
				schemes = append(schemes, s)
			}
		}
	}

	if len(schemes) == 0 {
		return model.AuthDef{}
	}

	return model.AuthDef{
		Required: true,
		Schemes:  schemes,
		Source:   "middleware",
	}
}

// detectFromExpr resolves a middleware expression to its function body and
// scans for auth patterns.
func detectFromExpr(expr ast.Expr, info *types.Info, pkgs []*packages.Package) []model.AuthScheme {
	body := resolveMiddlewareBody(expr, info, pkgs)
	if body == nil {
		return nil
	}
	return scanBodyForAuth(body, info, pkgs)
}

// resolveMiddlewareBody resolves a middleware expression (Ident, SelectorExpr,
// or CallExpr) to the function body statements.
func resolveMiddlewareBody(expr ast.Expr, info *types.Info, pkgs []*packages.Package) *ast.BlockStmt {
	switch e := expr.(type) {
	case *ast.Ident:
		fd := findFuncDeclByIdent(e, info, pkgs)
		if fd != nil {
			return fd.Body
		}
	case *ast.SelectorExpr:
		fd := findFuncDeclBySelector(e, info, pkgs)
		if fd != nil {
			return fd.Body
		}
	case *ast.CallExpr:
		// Middleware factories: e.g., authMiddleware("bearer")
		// Resolve the function being called.
		return resolveMiddlewareBody(e.Fun, info, pkgs)
	}
	return nil
}

// findFuncDeclByIdent finds the FuncDecl for an identifier.
func findFuncDeclByIdent(ident *ast.Ident, info *types.Info, pkgs []*packages.Package) *ast.FuncDecl {
	obj, ok := info.Uses[ident]
	if !ok {
		obj, ok = info.Defs[ident]
		if !ok {
			return nil
		}
	}
	return findFuncDeclByObj(obj, pkgs)
}

// findFuncDeclBySelector finds the FuncDecl for a selector expression.
func findFuncDeclBySelector(sel *ast.SelectorExpr, info *types.Info, pkgs []*packages.Package) *ast.FuncDecl {
	obj, ok := info.Uses[sel.Sel]
	if !ok {
		selection, ok := info.Selections[sel]
		if !ok {
			return nil
		}
		obj = selection.Obj()
	}
	return findFuncDeclByObj(obj, pkgs)
}

// findFuncDeclByObj finds the FuncDecl for a types.Object across all packages.
func findFuncDeclByObj(obj types.Object, pkgs []*packages.Package) *ast.FuncDecl {
	fn, ok := obj.(*types.Func)
	if !ok {
		return nil
	}

	fnPkg := fn.Pkg()
	if fnPkg == nil {
		return nil
	}

	var targetPkg *packages.Package
	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		if pkg.Types == fnPkg {
			targetPkg = pkg
			return false
		}
		return true
	}, nil)

	if targetPkg == nil {
		return nil
	}

	fnPos := fn.Pos()
	for _, file := range targetPkg.Syntax {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fd.Name.Pos() == fnPos {
				return fd
			}
		}
	}
	return nil
}

// scanBodyForAuth scans a function body (and inner function literals) for
// auth-related patterns.
func scanBodyForAuth(body *ast.BlockStmt, info *types.Info, pkgs []*packages.Package) []model.AuthScheme {
	var schemes []model.AuthScheme
	seen := make(map[model.AuthScheme]bool)

	// Check for known auth package imports used in function calls.
	schemes = append(schemes, checkAuthPackageImports(body, info, pkgs)...)
	for _, s := range schemes {
		seen[s] = true
	}

	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if s, ok := detectCallPattern(node, info); ok && !seen[s] {
				seen[s] = true
				schemes = append(schemes, s)
			}
		case *ast.FuncLit:
			// Scan inner function literals (e.g., http.HandlerFunc wrappers).
			inner := scanBodyForAuth(node.Body, info, pkgs)
			for _, s := range inner {
				if !seen[s] {
					seen[s] = true
					schemes = append(schemes, s)
				}
			}
			return false // already scanned
		}
		return true
	})

	return schemes
}

// detectCallPattern checks a single call expression for known auth patterns.
func detectCallPattern(call *ast.CallExpr, info *types.Info) (model.AuthScheme, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}

	methodName := sel.Sel.Name

	switch methodName {
	case "BasicAuth":
		// r.BasicAuth() → basic
		return model.AuthBasic, true

	case "Get":
		// r.Header.Get("Authorization") or r.Header.Get("X-API-Key")
		if isHeaderGet(sel) && len(call.Args) == 1 {
			headerName := stringLitValue(call.Args[0])
			switch headerName {
			case "Authorization":
				return model.AuthBearer, true
			case "X-API-Key", "Api-Key", "X-Api-Key":
				return model.AuthAPIKey, true
			}
		}
		// c.Get("Authorization") — Fiber direct header access on context
		if _, ok := sel.X.(*ast.Ident); ok && len(call.Args) == 1 {
			headerName := stringLitValue(call.Args[0])
			switch headerName {
			case "Authorization":
				return model.AuthBearer, true
			case "X-API-Key", "Api-Key", "X-Api-Key":
				return model.AuthAPIKey, true
			}
		}

	case "GetHeader":
		// c.GetHeader("Authorization") (gin)
		if len(call.Args) == 1 {
			headerName := stringLitValue(call.Args[0])
			switch headerName {
			case "Authorization":
				return model.AuthBearer, true
			case "X-API-Key", "Api-Key", "X-Api-Key":
				return model.AuthAPIKey, true
			}
		}

	case "Parse", "ParseWithClaims":
		// jwt.Parse / jwt.ParseWithClaims → bearer
		return model.AuthBearer, true
	}

	return "", false
}

// isHeaderGet checks if a selector expression is of the form ?.Header.Get.
func isHeaderGet(sel *ast.SelectorExpr) bool {
	if sel.Sel.Name != "Get" {
		return false
	}
	// The receiver should be ?.Header (another SelectorExpr).
	inner, ok := sel.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return inner.Sel.Name == "Header"
}

// stringLitValue extracts the string value from a basic literal expression.
func stringLitValue(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok {
		return ""
	}
	// Remove quotes.
	s := lit.Value
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// checkAuthPackageImports checks if any function calls in the body reference
// known auth packages.
func checkAuthPackageImports(body *ast.BlockStmt, info *types.Info, pkgs []*packages.Package) []model.AuthScheme {
	var schemes []model.AuthScheme

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check if the called function's package is a known auth package.
		var obj types.Object
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			obj = info.Uses[fn.Sel]
		case *ast.Ident:
			obj = info.Uses[fn]
		}

		if obj == nil {
			return true
		}

		fn, ok := obj.(*types.Func)
		if !ok {
			return true
		}

		if fn.Pkg() != nil {
			if scheme, ok := knownAuthPackages[fn.Pkg().Path()]; ok {
				schemes = append(schemes, scheme)
			}
		}

		return true
	})

	return schemes
}
