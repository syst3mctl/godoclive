package extractor

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

// maxHelperDepth bounds how deep registration-function expansion recurses so a
// function that (directly or mutually) calls itself cannot loop forever.
const maxHelperDepth = 8

// routerHelper is a function that takes part in route setup. paramIdx is the
// argument position of its router parameter, or -1 when the function builds its
// own router — a factory such as adminRouter() http.Handler, a method that
// registers on a struct field, or main() itself.
type routerHelper struct {
	decl     *ast.FuncDecl
	pkg      *packages.Package
	astFile  *ast.File
	paramIdx int
	// wraps marks a house router wrapper: a function that registers a route
	// whose path it takes as a parameter, so its call sites must be expanded
	// with their arguments bound for the route to resolve at all.
	wraps bool
}

// routerIndex maps function objects to their registration record and tracks
// which of them an expansion has already reached.
//
// Routers whose routes are registered outside main() all need the same two
// things: a way to find every function that registers routes regardless of what
// its signature says, and a way to expand a call to one of those functions so
// the prefix and middleware chain at the call site flow into the routes it
// registers. Each framework's extractor supplies only the parts that differ —
// which packages are in scope, what a registration looks like, and what its
// router type is.
type routerIndex struct {
	byObj   map[types.Object]*routerHelper
	ordered []*routerHelper
	reached map[*ast.FuncDecl]bool
}

// routerIndexSpec adapts one router framework to the shared index.
type routerIndexSpec struct {
	// inScope reports whether a package imports the framework at all.
	inScope func(*packages.Package) bool
	// registers reports whether a function body calls a registration method on
	// a value of the framework's router type.
	registers func(*ast.FuncDecl, *types.Info) bool
	// isRouter reports whether a type is the framework's router type.
	isRouter func(types.Type) bool
	// wrapsPath reports whether a function registers a route whose path it was
	// handed — a house router wrapper. Optional; a framework that leaves it nil
	// simply gets no wrapper expansion.
	wrapsPath func(*ast.FuncDecl, *types.Info) bool
}

// buildRouterIndex finds every function (including methods) that takes part in
// route setup, across all packages in scope.
//
// The first round indexes functions that call registration methods directly.
// Later rounds add the functions that hand a router to an already-indexed one —
// a main() whose whole routing body is routes.Register(r), or a delegator that
// fans out to per-resource registrars, registers nothing itself yet is the only
// link between the entry point and the routes. Rounds repeat until the set
// stops growing, so a chain of delegators is followed to its end.
func buildRouterIndex(pkgs []*packages.Package, spec routerIndexSpec) *routerIndex {
	idx := &routerIndex{
		byObj:   make(map[types.Object]*routerHelper),
		reached: make(map[*ast.FuncDecl]bool),
	}

	type candidate struct {
		fn      *ast.FuncDecl
		pkg     *packages.Package
		astFile *ast.File
	}
	var pending []candidate
	for _, pkg := range pkgs {
		if !spec.inScope(pkg) || pkg.TypesInfo == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				// Skip test and example functions so routes registered by test
				// fixtures do not reach the documentation.
				if strings.HasPrefix(fn.Name.Name, "Test") || strings.HasPrefix(fn.Name.Name, "Example") {
					continue
				}
				pending = append(pending, candidate{fn: fn, pkg: pkg, astFile: file})
			}
		}
	}

	for {
		remaining := pending[:0:0]
		var added bool
		for _, c := range pending {
			info := c.pkg.TypesInfo
			if !spec.registers(c.fn, info) && !feedsRegistration(c.fn, info, idx, spec.isRouter) {
				remaining = append(remaining, c)
				continue
			}
			h := &routerHelper{
				decl:     c.fn,
				pkg:      c.pkg,
				astFile:  c.astFile,
				paramIdx: routerParam(c.fn, info, spec.isRouter),
			}
			if spec.wrapsPath != nil {
				h.wraps = spec.wrapsPath(c.fn, info)
			}
			idx.ordered = append(idx.ordered, h)
			if obj, ok := info.Defs[c.fn.Name]; ok && obj != nil {
				idx.byObj[obj] = h
			}
			added = true
		}
		pending = remaining
		if !added {
			break
		}
	}
	return idx
}

// lookup resolves a call's function expression, or a function referenced by
// name, to its registration record — nil when it takes no part in route setup.
func (idx *routerIndex) lookup(info *types.Info, expr ast.Expr) *routerHelper {
	if idx == nil || info == nil {
		return nil
	}
	obj := identObject(info, expr)
	if obj == nil {
		return nil
	}
	return idx.byObj[obj]
}

// feedsRegistration reports whether fn takes part in route setup by calling an
// already-indexed function.
//
// Two shapes count. The first hands a router to a function that registers on
// it — a main() whose whole routing body is routes.Register(r). The second
// calls a house router wrapper, handing it the path and handler it registers;
// such a caller names no router at all, and without this is invisible, which
// leaves the wrapper with no call site and the service with no routes.
func feedsRegistration(fn *ast.FuncDecl, info *types.Info, idx *routerIndex, isRouter func(types.Type) bool) bool {
	if info == nil || len(idx.byObj) == 0 {
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
		h := idx.lookup(info, call.Fun)
		if h == nil {
			return true
		}
		if h.wraps && canWrap(h, call) {
			found = true
			return false
		}
		if h.paramIdx < 0 || h.paramIdx >= len(call.Args) {
			return true
		}
		if isRouter(info.TypeOf(call.Args[h.paramIdx])) {
			found = true
			return false
		}
		return true
	})
	return found
}

// routerParam returns the argument position of fn's first router parameter, or
// -1 when it has none. Grouped parameters (`a, b chi.Router`) count each name
// separately so the index matches the call's argument positions.
func routerParam(fn *ast.FuncDecl, info *types.Info, isRouter func(types.Type) bool) int {
	if fn.Type == nil || fn.Type.Params == nil {
		return -1
	}
	pos := 0
	for _, field := range fn.Type.Params.List {
		names := len(field.Names)
		if names == 0 {
			names = 1 // unnamed parameter still occupies a position
		}
		if isRouter(info.TypeOf(field.Type)) {
			return pos
		}
		pos += names
	}
	return -1
}

// identObject resolves an identifier or selector expression to the object it
// refers to.
func identObject(info *types.Info, expr ast.Expr) types.Object {
	if info == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.Ident:
		return info.Uses[e]
	case *ast.SelectorExpr:
		return info.Uses[e.Sel]
	}
	return nil
}

// receiverInPackage reports whether a method call resolves into the given
// package — true both when the receiver is that package's type and when the
// method is promoted from an embedded field of it.
func receiverInPackage(sel *ast.SelectorExpr, info *types.Info, pkgPath func(string) bool) bool {
	if info == nil {
		return false
	}
	s, ok := info.Selections[sel]
	if !ok || s.Obj() == nil {
		return false
	}
	pkg := s.Obj().Pkg()
	return pkg != nil && pkgPath(pkg.Path())
}

// unknownOriginNote explains why a registration function's routes may be
// missing their prefix.
func unknownOriginNote(name string) string {
	return "route group origin unknown: " + name +
		" registers routes on a router parameter, but no resolvable call site was found — " +
		"path prefix and middleware chain (including auth) are incomplete"
}
