package auth

import (
	"bytes"
	"go/ast"
	"go/constant"
	"go/printer"
	"go/token"
	"go/types"

	"github.com/syst3mctl/godoclive/internal/extractor"
	"github.com/syst3mctl/godoclive/internal/model"
	"golang.org/x/tools/go/packages"
)

// knownAuthPackages maps import paths to their inferred auth scheme.
var knownAuthPackages = map[string]model.AuthScheme{
	"github.com/golang-jwt/jwt/v5":          model.AuthBearer,
	"github.com/golang-jwt/jwt":             model.AuthBearer,
	"github.com/dgrijalva/jwt-go":           model.AuthBearer,
	"github.com/auth0/go-jwt-middleware":    model.AuthBearer,
	"github.com/auth0/go-jwt-middleware/v2": model.AuthBearer,
}

// DetectAuth examines a route's middleware chain to determine the
// authentication scheme(s) it applies. Each middleware is resolved to its
// function body and scanned for known auth patterns.
//
// A chain can span packages — a group's middleware is registered where the
// group is built while inline middleware is written inside a registration
// helper — so every entry carries the TypesInfo of the package it was written
// in; fallback is used only when an entry has none.
//
// The returned notes list middleware defined in the analyzed packages that
// could not be resolved: an unread middleware may be the very one enforcing
// auth, so the caller must be able to report the gap instead of reporting an
// endpoint as public.
func DetectAuth(middlewares []extractor.MiddlewareRef, fallback *types.Info, pkgs []*packages.Package) (model.AuthDef, []string) {
	var schemes []model.AuthScheme
	var notes []string
	seen := make(map[model.AuthScheme]bool)

	anyRequired := false
	anyOptional := false

	for _, mw := range middlewares {
		info := mw.Info
		if info == nil {
			info = fallback
		}
		if info == nil {
			continue
		}

		body, bodyInfo := resolveMiddlewareBody(mw.Expr, info, pkgs)
		if bodyInfo == nil {
			bodyInfo = info
		}
		if body == nil {
			if isLocalMiddleware(mw.Expr, info, pkgs) {
				notes = append(notes, "middleware: "+exprString(mw.Expr)+
					" could not be resolved — auth requirements for this route may be incomplete")
			}
			continue
		}

		detected := scanBodyForAuth(body, bodyInfo, pkgs)
		if len(detected) == 0 {
			continue
		}
		for _, s := range detected {
			if !seen[s] {
				seen[s] = true
				schemes = append(schemes, s)
			}
		}
		if middlewareRequiresAuth(mw.Expr, info, pkgs) {
			anyRequired = true
		} else {
			anyOptional = true
		}
	}

	if len(schemes) == 0 {
		return model.AuthDef{}, notes
	}

	return model.AuthDef{
		Required: anyRequired,
		Optional: !anyRequired && anyOptional,
		Schemes:  schemes,
		Source:   "middleware",
	}, notes
}

// middlewareRequiresAuth decides whether a middleware rejects unauthenticated
// requests or merely reads credentials when they happen to be present.
//
// The common idiom is a factory whose boolean argument gates the rejection:
//
//	func AuthMiddleware(auto401 bool) gin.HandlerFunc {
//		return func(c *gin.Context) {
//			if token == "" {
//				if auto401 { c.AbortWithStatus(http.StatusUnauthorized) }
//				return
//			}
//			…
//
// AuthMiddleware(true) requires auth; AuthMiddleware(false) makes it optional.
// When every 401 in the middleware sits behind such a gate, the constant passed
// at the call site decides. Anything else — an unconditional 401, an argument
// that is not a compile-time constant, an unresolvable factory — is treated as
// required, the safe reading for documentation.
func middlewareRequiresAuth(expr ast.Expr, info *types.Info, pkgs []*packages.Package) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return true
	}

	var decl *ast.FuncDecl
	var declInfo *types.Info
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		decl, declInfo = findFuncDeclAndInfoByIdent(fn, info, pkgs)
	case *ast.SelectorExpr:
		decl, declInfo = findFuncDeclAndInfoBySelector(fn, info, pkgs)
	}
	if decl == nil || decl.Body == nil || decl.Type.Params == nil {
		return true
	}
	if declInfo == nil {
		declInfo = info
	}

	// An unconditional rejection means auth is required whatever the arguments.
	if hasUnauthorizedOutsideGate(decl.Body, declInfo) {
		return true
	}

	pos := 0
	for _, field := range decl.Type.Params.List {
		names := len(field.Names)
		if names == 0 {
			names = 1
		}
		if isBoolType(declInfo.TypeOf(field.Type)) {
			for i, name := range field.Names {
				argIdx := pos + i
				if argIdx >= len(call.Args) {
					continue
				}
				argVal, ok := boolConstValue(call.Args[argIdx], info)
				if !ok {
					continue
				}
				if gated, negated, ok := unauthorizedGatedBy(decl.Body, name.Name, declInfo); ok && gated {
					if negated {
						return !argVal
					}
					return argVal
				}
			}
		}
		pos += names
	}

	return true
}

// unauthorizedGatedBy reports whether every 401 response in body sits inside an
// `if <param>` (or `if !<param>`) guard, and which polarity that guard has.
func unauthorizedGatedBy(body *ast.BlockStmt, param string, info *types.Info) (gated, negated, found bool) {
	ast.Inspect(body, func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		cond, neg := condIdent(ifStmt.Cond)
		if cond != param {
			return true
		}
		if ifStmt.Body != nil && mentionsUnauthorized(ifStmt.Body, info) {
			gated, negated, found = true, neg, true
			return false
		}
		return true
	})
	return gated, negated, found
}

// condIdent unwraps `x` and `!x` to the identifier name and whether it was negated.
func condIdent(cond ast.Expr) (string, bool) {
	negated := false
	if unary, ok := cond.(*ast.UnaryExpr); ok && unary.Op == token.NOT {
		negated = true
		cond = unary.X
	}
	if ident, ok := cond.(*ast.Ident); ok {
		return ident.Name, negated
	}
	return "", negated
}

// hasUnauthorizedOutsideGate reports whether a 401 appears anywhere that is not
// inside an `if <bool ident>` guard.
func hasUnauthorizedOutsideGate(body *ast.BlockStmt, info *types.Info) bool {
	gatedBodies := make(map[ast.Node]bool)
	ast.Inspect(body, func(n ast.Node) bool {
		if ifStmt, ok := n.(*ast.IfStmt); ok {
			if name, _ := condIdent(ifStmt.Cond); name != "" && ifStmt.Body != nil {
				gatedBodies[ifStmt.Body] = true
			}
		}
		return true
	})

	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil || found {
			return false
		}
		if n != ast.Node(body) && gatedBodies[n] {
			return false // the gate decides whether this 401 applies
		}
		if isUnauthorizedExpr(n, info) {
			found = true
			return false
		}
		return true
	})
	return found
}

// mentionsUnauthorized reports whether a 401 status appears anywhere in n.
func mentionsUnauthorized(n ast.Node, info *types.Info) bool {
	found := false
	ast.Inspect(n, func(child ast.Node) bool {
		if found || child == nil {
			return false
		}
		if isUnauthorizedExpr(child, info) {
			found = true
			return false
		}
		return true
	})
	return found
}

// isUnauthorizedExpr reports whether an expression is the 401 status constant
// or literal.
func isUnauthorizedExpr(n ast.Node, info *types.Info) bool {
	expr, ok := n.(ast.Expr)
	if !ok {
		return false
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok && sel.Sel.Name == "StatusUnauthorized" {
		return true
	}
	if info == nil {
		return false
	}
	tv, ok := info.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.Int {
		return false
	}
	v, exact := constant.Int64Val(tv.Value)
	return exact && v == 401
}

// boolConstValue resolves an argument to a compile-time boolean.
func boolConstValue(expr ast.Expr, info *types.Info) (bool, bool) {
	if info == nil {
		return false, false
	}
	tv, ok := info.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.Bool {
		return false, false
	}
	return constant.BoolVal(tv.Value), true
}

// isBoolType reports whether t is the predeclared bool.
func isBoolType(t types.Type) bool {
	basic, ok := t.(*types.Basic)
	return ok && basic.Kind() == types.Bool
}

// isLocalMiddleware reports whether a middleware expression names something
// declared in one of the analyzed packages, as opposed to a framework or
// third-party middleware whose internals are out of scope.
func isLocalMiddleware(expr ast.Expr, info *types.Info, pkgs []*packages.Package) bool {
	var obj types.Object
	switch e := expr.(type) {
	case *ast.Ident:
		obj = info.Uses[e]
	case *ast.SelectorExpr:
		obj = info.Uses[e.Sel]
	case *ast.CallExpr:
		return isLocalMiddleware(e.Fun, info, pkgs)
	default:
		return false
	}
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	for _, pkg := range pkgs {
		if pkg.Types == obj.Pkg() {
			return true
		}
	}
	return false
}

// exprString renders a middleware expression compactly for diagnostics.
func exprString(expr ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, token.NewFileSet(), expr); err != nil {
		return "<expr>"
	}
	return buf.String()
}

// maxMiddlewareHops bounds middleware-resolution indirection (var → initializer
// → factory call → …) so a pathological chain can never recurse unboundedly.
const maxMiddlewareHops = 4

// resolveMiddlewareBody resolves a middleware expression (Ident, SelectorExpr,
// CallExpr, or FuncLit) to the function body statements, along with the
// TypesInfo of the package that body was declared in.
func resolveMiddlewareBody(expr ast.Expr, info *types.Info, pkgs []*packages.Package) (*ast.BlockStmt, *types.Info) {
	return resolveMiddlewareBodyDepth(expr, info, pkgs, 0)
}

func resolveMiddlewareBodyDepth(expr ast.Expr, info *types.Info, pkgs []*packages.Package, depth int) (*ast.BlockStmt, *types.Info) {
	if depth > maxMiddlewareHops {
		return nil, nil
	}
	switch e := expr.(type) {
	case *ast.Ident:
		fd, declInfo := findFuncDeclAndInfoByIdent(e, info, pkgs)
		if fd != nil {
			return fd.Body, declInfo
		}
		// Not a declared function — the ident may be a local VARIABLE holding a
		// middleware built by a factory, the common edge idiom:
		//
		//	requireAuth := auth.RequireAuth(verifier, logger)
		//	mux.Handle("GET /x", requireAuth(http.HandlerFunc(h)))
		//
		// Trace the variable to its initializer expression and resolve THAT
		// (the factory call's function body contains the auth patterns).
		if init := findVarInit(e, info, pkgs); init != nil {
			return resolveMiddlewareBodyDepth(init, info, pkgs, depth+1)
		}
	case *ast.SelectorExpr:
		fd, declInfo := findFuncDeclAndInfoBySelector(e, info, pkgs)
		if fd != nil {
			return fd.Body, declInfo
		}
	case *ast.CallExpr:
		// Middleware factories: e.g., authMiddleware("bearer")
		// Resolve the function being called.
		return resolveMiddlewareBodyDepth(e.Fun, info, pkgs, depth+1)
	case *ast.FuncLit:
		return e.Body, info
	}
	return nil, nil
}

// findVarInit locates the initializer expression of a variable identifier: the
// matching RHS of its `:=` / `=` assignment or its declaration ValueSpec. Only
// a 1:1 LHS↔RHS pairing is traced — a multi-value assignment from a single call
// cannot be split syntactically and returns nil.
func findVarInit(ident *ast.Ident, info *types.Info, pkgs []*packages.Package) ast.Expr {
	obj, ok := info.Uses[ident]
	if !ok {
		if obj, ok = info.Defs[ident]; !ok {
			return nil
		}
	}
	if _, isVar := obj.(*types.Var); !isVar || obj.Pkg() == nil {
		return nil
	}

	var targetPkg *packages.Package
	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		if pkg.Types == obj.Pkg() {
			targetPkg = pkg
			return false
		}
		return true
	}, nil)
	if targetPkg == nil {
		return nil
	}

	var init ast.Expr
	for _, file := range targetPkg.Syntax {
		if init != nil {
			break
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if init != nil {
				return false
			}
			switch stmt := n.(type) {
			case *ast.AssignStmt:
				if len(stmt.Lhs) != len(stmt.Rhs) {
					return true
				}
				for i, lhs := range stmt.Lhs {
					id, ok := lhs.(*ast.Ident)
					if !ok {
						continue
					}
					if targetPkg.TypesInfo.Defs[id] == obj || targetPkg.TypesInfo.Uses[id] == obj {
						init = stmt.Rhs[i]
						return false
					}
				}
			case *ast.ValueSpec:
				if len(stmt.Names) != len(stmt.Values) {
					return true
				}
				for i, name := range stmt.Names {
					if targetPkg.TypesInfo.Defs[name] == obj {
						init = stmt.Values[i]
						return false
					}
				}
			}
			return true
		})
	}
	return init
}

// findFuncDeclAndInfoByIdent resolves an identifier to its declaration and to
// the TypesInfo of the package that declares it. Scanning a callee's body with
// the caller's TypesInfo yields nothing once the two are in different packages,
// so every descent has to switch to the declaring package's information.
func findFuncDeclAndInfoByIdent(ident *ast.Ident, info *types.Info, pkgs []*packages.Package) (*ast.FuncDecl, *types.Info) {
	obj, ok := info.Uses[ident]
	if !ok {
		obj, ok = info.Defs[ident]
		if !ok {
			return nil, nil
		}
	}
	return findFuncDeclAndInfoByObj(obj, pkgs)
}

// findFuncDeclAndInfoBySelector resolves a selector to its declaration and the
// TypesInfo of the declaring package.
func findFuncDeclAndInfoBySelector(sel *ast.SelectorExpr, info *types.Info, pkgs []*packages.Package) (*ast.FuncDecl, *types.Info) {
	obj, ok := info.Uses[sel.Sel]
	if !ok {
		selection, ok := info.Selections[sel]
		if !ok {
			return nil, nil
		}
		obj = selection.Obj()
	}
	return findFuncDeclAndInfoByObj(obj, pkgs)
}

// findFuncDeclAndInfoByObj finds the FuncDecl for a types.Object across all
// packages, together with the declaring package's TypesInfo.
func findFuncDeclAndInfoByObj(obj types.Object, pkgs []*packages.Package) (*ast.FuncDecl, *types.Info) {
	fn, ok := obj.(*types.Func)
	if !ok {
		return nil, nil
	}

	fnPkg := fn.Pkg()
	if fnPkg == nil {
		return nil, nil
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
		return nil, nil
	}

	fnPos := fn.Pos()
	for _, file := range targetPkg.Syntax {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fd.Name.Pos() == fnPos {
				return fd, targetPkg.TypesInfo
			}
		}
	}
	return nil, nil
}

// scanBodyForAuth scans a function body (and inner function literals) for
// auth-related patterns.
func scanBodyForAuth(body *ast.BlockStmt, info *types.Info, pkgs []*packages.Package) []model.AuthScheme {
	return scanBodyForAuthDepth(body, info, pkgs, 0)
}

// maxAuthScanDepth bounds how many call levels below a middleware body the scan
// follows when looking for the credential read.
const maxAuthScanDepth = 1

func scanBodyForAuthDepth(body *ast.BlockStmt, info *types.Info, pkgs []*packages.Package, depth int) []model.AuthScheme {
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
			// Middleware commonly delegates the credential read to a small
			// helper — extractToken(c), tokenFromRequest(r). Follow such a call
			// ONE level so the scheme is still identified; deeper chains stay
			// out of scope.
			if depth < maxAuthScanDepth {
				if helper, helperInfo := helperBodyForCall(node, info, pkgs); helper != nil {
					for _, s := range scanBodyForAuthDepth(helper, helperInfo, pkgs, depth+1) {
						if !seen[s] {
							seen[s] = true
							schemes = append(schemes, s)
						}
					}
				}
			}
		case *ast.FuncLit:
			// Scan inner function literals (e.g., http.HandlerFunc wrappers).
			inner := scanBodyForAuthDepth(node.Body, info, pkgs, depth)
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

// helperBodyForCall resolves a plain function call inside a middleware body to
// the callee's body, when that callee is declared in a package we loaded.
func helperBodyForCall(call *ast.CallExpr, info *types.Info, pkgs []*packages.Package) (*ast.BlockStmt, *types.Info) {
	var decl *ast.FuncDecl
	var declInfo *types.Info
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		decl, declInfo = findFuncDeclAndInfoByIdent(fn, info, pkgs)
	case *ast.SelectorExpr:
		decl, declInfo = findFuncDeclAndInfoBySelector(fn, info, pkgs)
	}
	if decl == nil {
		return nil, nil
	}
	if declInfo == nil {
		declInfo = info
	}
	return decl.Body, declInfo
}
