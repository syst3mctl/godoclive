package loader

import (
	"fmt"
	"log"

	"golang.org/x/tools/go/packages"
)

// LoadPackages loads, parses, and type-checks all Go packages matching the
// given pattern (e.g. "./..." or "./cmd/myapp"). The optional dir parameter
// sets the working directory for the load — use it when loading packages from
// a module outside the current working directory. If dir is empty, the current
// working directory is used.
func LoadPackages(dir, pattern string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Dir: dir,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedDeps |
			packages.NeedImports,
	}

	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		return nil, fmt.Errorf("loading packages %q: %w", pattern, err)
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages found matching %q", pattern)
	}

	// Check for hard errors — if every root package has errors and no syntax,
	// the load effectively failed.
	var allBroken = true
	for _, pkg := range pkgs {
		if len(pkg.Syntax) > 0 {
			allBroken = false
			break
		}
	}

	// Log warnings for packages with errors but continue loading.
	var hasErrors bool
	packages.Visit(pkgs, nil, func(pkg *packages.Package) {
		for _, err := range pkg.Errors {
			log.Printf("warning: package %s: %s", pkg.PkgPath, err)
			hasErrors = true
		}
	})

	if allBroken && hasErrors {
		return nil, fmt.Errorf("all packages failed to load for pattern %q", pattern)
	}

	if hasErrors {
		log.Printf("warning: some packages had errors; analysis may be incomplete")
	}

	return pkgs, nil
}
