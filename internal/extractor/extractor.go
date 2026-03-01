package extractor

import (
	"go/ast"

	"golang.org/x/tools/go/packages"
)

// RawRoute represents a single HTTP route extracted from the AST before
// handler resolution and contract analysis. This is the output of the
// route extraction phase — router-agnostic after normalization.
type RawRoute struct {
	Method      string     // GET, POST, PUT, DELETE, PATCH
	Path        string     // Normalized path with {param} format
	HandlerExpr ast.Expr   // The AST expression referencing the handler function
	Middlewares []ast.Expr // Middleware expressions applied to this route
	File        string     // Source file where this route was registered
	Line        int        // Line number of the route registration
}

// Extractor discovers HTTP route registrations from parsed Go packages.
// Each router framework (chi, gin) has its own implementation.
type Extractor interface {
	Extract(pkgs []*packages.Package) ([]RawRoute, error)
}
