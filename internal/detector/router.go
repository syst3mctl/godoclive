package detector

import (
	"strings"

	"golang.org/x/tools/go/packages"
)

// RouterKind identifies which HTTP router framework a project uses.
type RouterKind string

const (
	RouterKindChi     RouterKind = "chi"
	RouterKindGin     RouterKind = "gin"
	RouterKindUnknown RouterKind = "unknown"
)

// DetectRouter scans all import paths across all files in the provided packages
// and determines which router framework is in use. If both chi and gin are
// present, the one used in more files wins (chi as tiebreaker).
func DetectRouter(pkgs []*packages.Package) RouterKind {
	var chiFiles, ginFiles int

	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		for imp := range pkg.Imports {
			if isChiImport(imp) {
				chiFiles++
			}
			if isGinImport(imp) {
				ginFiles++
			}
		}
		return true
	}, nil)

	switch {
	case chiFiles == 0 && ginFiles == 0:
		return RouterKindUnknown
	case chiFiles >= ginFiles:
		return RouterKindChi
	default:
		return RouterKindGin
	}
}

func isChiImport(path string) bool {
	return path == "github.com/go-chi/chi" ||
		strings.HasPrefix(path, "github.com/go-chi/chi/")
}

func isGinImport(path string) bool {
	return path == "github.com/gin-gonic/gin" ||
		strings.HasPrefix(path, "github.com/gin-gonic/gin/")
}
