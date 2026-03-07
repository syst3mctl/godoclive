package detector

import (
	"go/ast"
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
	RouterKindUnknown RouterKind = "unknown"
)

// DetectRouter scans all import paths across all packages in the provided set
// and determines which router framework is in use.
// Priority: chi > gin > stdlib > unknown.
// Stdlib detection requires actual route registration calls (http.HandleFunc,
// http.Handle, http.NewServeMux), not just net/http imports.
func DetectRouter(pkgs []*packages.Package) RouterKind {
	var chiImports, ginImports, gorillaImports, echoImports int

	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		for imp := range pkg.Imports {
			if isChiImport(imp) {
				chiImports++
			}
			if isGinImport(imp) {
				ginImports++
			}
			if isGorillaImport(imp) {
				gorillaImports++
			}
			if isEchoImport(imp) {
				echoImports++
			}
		}
		return true
	}, nil)

	switch {
	case chiImports > 0 && chiImports >= ginImports:
		return RouterKindChi
	case ginImports > 0:
		return RouterKindGin
	case gorillaImports > 0:
		return RouterKindGorilla
	case echoImports > 0:
		return RouterKindEcho
	}

	// No third-party router found. Check for stdlib mux usage.
	if hasStdlibMuxUsage(pkgs) {
		return RouterKindStdlib
	}

	return RouterKindUnknown
}

// hasStdlibMuxUsage checks if any package uses stdlib HTTP route registration
// patterns: http.NewServeMux(), http.HandleFunc(), http.Handle(), or
// mux.HandleFunc()/mux.Handle() on a *http.ServeMux variable.
func hasStdlibMuxUsage(pkgs []*packages.Package) bool {
	var found bool
	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		if found {
			return false
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
				name := sel.Sel.Name
				// http.NewServeMux(), http.HandleFunc(), http.Handle()
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "http" {
					if name == "NewServeMux" || name == "HandleFunc" || name == "Handle" {
						found = true
						return false
					}
				}
				// mux.HandleFunc() / mux.Handle() — any variable calling HandleFunc/Handle
				if name == "HandleFunc" || name == "Handle" {
					if _, ok := sel.X.(*ast.Ident); ok {
						// Check that the package imports net/http
						for imp := range pkg.Imports {
							if imp == "net/http" {
								found = true
								return false
							}
						}
					}
				}
				return true
			})
		}
		return true
	}, nil)
	return found
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
