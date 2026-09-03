package extractor

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// RawRoute represents a single HTTP route extracted from the AST before
// handler resolution and contract analysis. This is the output of the
// route extraction phase — router-agnostic after normalization.
type RawRoute struct {
	Method      string          // GET, POST, PUT, DELETE, PATCH
	Path        string          // Normalized path with {param} format
	HandlerExpr ast.Expr        // The AST expression referencing the handler function
	Middlewares []MiddlewareRef // Middleware expressions applied to this route
	File        string          // Source file where this route was registered
	Line        int             // Line number of the route registration
	Unresolved  []string        // Caveats about the registration itself (unknown group origin, empty path)
}

// MiddlewareRef is one middleware expression together with the type information
// of the package it was written in. A single route's chain can span packages —
// the group's chain is assembled at the registration site while inline
// middleware is written inside a registration helper — so each expression has
// to carry the TypesInfo needed to resolve it.
type MiddlewareRef struct {
	Expr ast.Expr
	Info *types.Info
}

// middlewareRefs pairs each expression with the given type information.
func middlewareRefs(exprs []ast.Expr, info *types.Info) []MiddlewareRef {
	if len(exprs) == 0 {
		return nil
	}
	refs := make([]MiddlewareRef, 0, len(exprs))
	for _, e := range exprs {
		refs = append(refs, MiddlewareRef{Expr: e, Info: info})
	}
	return refs
}

// copyMiddleware returns a shallow copy of a middleware slice.
func copyMiddleware(refs []MiddlewareRef) []MiddlewareRef {
	if len(refs) == 0 {
		return nil
	}
	cp := make([]MiddlewareRef, len(refs))
	copy(cp, refs)
	return cp
}

// concatMiddleware returns a new slice containing both chains in order.
func concatMiddleware(a, b []MiddlewareRef) []MiddlewareRef {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	out := make([]MiddlewareRef, 0, len(a)+len(b))
	out = append(out, a...)
	return append(out, b...)
}

// Extractor discovers HTTP route registrations from parsed Go packages.
// Each router framework (chi, gin) has its own implementation.
type Extractor interface {
	Extract(pkgs []*packages.Package) ([]RawRoute, error)
}
