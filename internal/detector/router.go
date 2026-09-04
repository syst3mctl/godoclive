package detector

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

// RouterKind identifies which HTTP router framework a project uses.
type RouterKind string

const (
	RouterKindChi     RouterKind = "chi"
	RouterKindGin     RouterKind = "gin"
	RouterKindStdlib  RouterKind = "stdlib"
	RouterKindGorilla RouterKind = "gorilla"
	RouterKindEcho    RouterKind = "echo"
	RouterKindFiber   RouterKind = "fiber"
	RouterKindUnknown RouterKind = "unknown"
)

// detectionOrder is the deterministic order in which frameworks are reported.
// It is also the priority order DetectRouter falls back on when a project
// registers routes on more than one framework.
var detectionOrder = []RouterKind{
	RouterKindChi,
	RouterKindGin,
	RouterKindGorilla,
	RouterKindEcho,
	RouterKindFiber,
	RouterKindStdlib,
}

// importMatchers pairs each third-party framework with the predicate that
// recognizes its import path.
var importMatchers = map[RouterKind]func(string) bool{
	RouterKindChi:     isChiImport,
	RouterKindGin:     isGinImport,
	RouterKindGorilla: isGorillaImport,
	RouterKindEcho:    isEchoImport,
	RouterKindFiber:   isFiberImport,
}

// DetectRouters returns every router framework the analyzed packages register
// routes on, in a stable order.
//
// A service is not obliged to pick one router. A gin API mounted beside a
// stdlib ServeMux for health and metrics, or a chi service that keeps a legacy
// gorilla/mux subtree, are ordinary shapes — and reporting only the winner of a
// priority contest silently drops every route belonging to the others. Each
// detected framework gets its own extractor run, so the result is the union.
//
// Only packages carrying syntax — the application's own code — are scanned.
// A framework that appears solely in the dependency graph is not evidence that
// this service registers routes on it.
func DetectRouters(pkgs []*packages.Package) []RouterKind {
	found := make(map[RouterKind]bool, len(detectionOrder))

	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		if len(pkg.Syntax) == 0 {
			return true
		}
		for imp := range pkg.Imports {
			for kind, match := range importMatchers {
				if match(imp) {
					found[kind] = true
				}
			}
		}
		return true
	}, nil)

	if hasStdlibMuxUsage(pkgs) {
		found[RouterKindStdlib] = true
	}

	kinds := make([]RouterKind, 0, len(found))
	for _, kind := range detectionOrder {
		if found[kind] {
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

// DetectRouter reports the primary router framework in use, or
// RouterKindUnknown when none is found. It exists for callers that need a
// single answer; use DetectRouters to see every framework a project registers
// routes on.
func DetectRouter(pkgs []*packages.Package) RouterKind {
	kinds := DetectRouters(pkgs)
	if len(kinds) == 0 {
		return RouterKindUnknown
	}
	return kinds[0]
}

// hasStdlibMuxUsage reports whether any analyzed package registers a route on a
// net/http ServeMux: the package-level http.HandleFunc / http.Handle, which
// register on http.DefaultServeMux, http.NewServeMux, or a Handle/HandleFunc
// call whose receiver type-checks to *http.ServeMux.
//
// The receiver is checked against go/types rather than by name. Every router
// in this package spells its catch-all registration "Handle", so matching the
// method name alone reports stdlib for any chi, gin or gorilla service that
// happens to import net/http — which all of them do.
func hasStdlibMuxUsage(pkgs []*packages.Package) bool {
	var found bool
	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		if found || len(pkg.Syntax) == 0 || !importsNetHTTP(pkg) {
			return true
		}
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if found {
					return false
				}
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "NewServeMux", "HandleFunc", "Handle":
				default:
					return true
				}
				// http.NewServeMux() / http.HandleFunc() / http.Handle().
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "http" {
					if pkg.TypesInfo == nil {
						found = true
						return false
					}
					// Guard against a local variable shadowing the import.
					if obj, ok := pkg.TypesInfo.Uses[ident].(*types.PkgName); ok &&
						obj.Imported().Path() == "net/http" {
						found = true
						return false
					}
				}
				// mux.Handle() / mux.HandleFunc() on a *http.ServeMux value.
				if pkg.TypesInfo != nil && isServeMux(pkg.TypesInfo.TypeOf(sel.X)) {
					found = true
					return false
				}
				return true
			})
		}
		return true
	}, nil)
	return found
}

// importsNetHTTP reports whether the package imports net/http.
func importsNetHTTP(pkg *packages.Package) bool {
	for imp := range pkg.Imports {
		if imp == "net/http" {
			return true
		}
	}
	return false
}

// isServeMux reports whether t is http.ServeMux, possibly behind a pointer.
func isServeMux(t types.Type) bool {
	if t == nil {
		return false
	}
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj != nil && obj.Pkg() != nil &&
		obj.Pkg().Path() == "net/http" && obj.Name() == "ServeMux"
}

func isChiImport(path string) bool {
	return path == "github.com/go-chi/chi" ||
		strings.HasPrefix(path, "github.com/go-chi/chi/")
}

func isGinImport(path string) bool {
	return path == "github.com/gin-gonic/gin" ||
		strings.HasPrefix(path, "github.com/gin-gonic/gin/")
}

func isGorillaImport(path string) bool {
	return path == "github.com/gorilla/mux"
}

func isEchoImport(path string) bool {
	return path == "github.com/labstack/echo/v4" ||
		strings.HasPrefix(path, "github.com/labstack/echo/v4/") ||
		path == "github.com/labstack/echo"
}

func isFiberImport(path string) bool {
	return path == "github.com/gofiber/fiber/v2" ||
		strings.HasPrefix(path, "github.com/gofiber/fiber/v2/")
}
