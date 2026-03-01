package pipeline

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/syst3mctl/godoclive/internal/auth"
	"github.com/syst3mctl/godoclive/internal/config"
	"github.com/syst3mctl/godoclive/internal/contract"
	"github.com/syst3mctl/godoclive/internal/detector"
	"github.com/syst3mctl/godoclive/internal/extractor"
	"github.com/syst3mctl/godoclive/internal/loader"
	"github.com/syst3mctl/godoclive/internal/mapper"
	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/resolver"
	"golang.org/x/tools/go/packages"
)

// RunPipeline orchestrates the full analysis pipeline:
// load → detect → extract → resolve → contract → map → auth → infer → config.
func RunPipeline(dir, pattern string, cfg *config.Config) ([]model.EndpointDef, error) {
	// 1. Load packages.
	pkgs, err := loader.LoadPackages(dir, pattern)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}

	// 2. Detect router.
	routerKind := detector.DetectRouter(pkgs)
	if routerKind == detector.RouterKindUnknown {
		return nil, fmt.Errorf("no supported router detected (expected chi or gin)")
	}

	// 3. Choose and run the appropriate extractor.
	var ext extractor.Extractor
	switch routerKind {
	case detector.RouterKindChi:
		ext = &extractor.ChiExtractor{}
	case detector.RouterKindGin:
		ext = &extractor.GinExtractor{}
	}

	routes, err := ext.Extract(pkgs)
	if err != nil {
		return nil, fmt.Errorf("extracting routes: %w", err)
	}

	// 4-8. Process each route into a full EndpointDef.
	var endpoints []model.EndpointDef
	for _, route := range routes {
		ep, err := processRoute(route, pkgs)
		if err != nil {
			// Record the error as unresolved rather than failing the whole pipeline.
			endpoints = append(endpoints, model.EndpointDef{
				Method:     route.Method,
				Path:       route.Path,
				File:       route.File,
				Line:       route.Line,
				Unresolved: []string{err.Error()},
			})
			continue
		}
		endpoints = append(endpoints, ep)
	}

	// 9. Apply config exclusions and overrides.
	if cfg != nil {
		endpoints = config.ApplyExclusions(endpoints, cfg.Exclude)
		endpoints = config.ApplyOverrides(endpoints, cfg.Overrides)
	}

	return endpoints, nil
}

// processRoute converts a single RawRoute into a fully-resolved EndpointDef.
func processRoute(route extractor.RawRoute, pkgs []*packages.Package) (model.EndpointDef, error) {
	// Find the TypesInfo from the package that contains this route's file.
	info := findInfoForRoute(route, pkgs)
	if info == nil {
		return model.EndpointDef{}, fmt.Errorf("could not find type info for route %s %s", route.Method, route.Path)
	}

	// 4a. Resolve handler to function declaration.
	funcDecl, funcLit, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
	if err != nil {
		return model.EndpointDef{}, fmt.Errorf("resolving handler: %w", err)
	}

	// Get the handler AST node and resolve param names.
	var handlerNode ast.Node
	var paramNames resolver.HandlerParamNames
	var handlerName string
	var handlerPkg string
	var handlerFile string
	var handlerLine int

	if funcDecl != nil {
		handlerNode = funcDecl
		paramNames = resolver.ResolveHandlerParams(funcDecl.Type, info)
		handlerName = funcDecl.Name.Name
		handlerFile, handlerLine = posToFileLine(funcDecl.Pos(), pkgs)
	} else if funcLit != nil {
		handlerNode = funcLit
		paramNames = resolver.ResolveHandlerParams(funcLit.Type, info)
		handlerName = "anonymous"
		handlerFile, handlerLine = posToFileLine(funcLit.Pos(), pkgs)
	}

	// Resolve package from the function's object if possible.
	if funcDecl != nil {
		if obj, ok := info.Defs[funcDecl.Name]; ok && obj != nil {
			if fn, ok := obj.(*types.Func); ok && fn.Pkg() != nil {
				handlerPkg = fn.Pkg().Path()
			}
		}
	}

	// If handler name is still "anonymous" or empty, try the expression.
	if handlerName == "anonymous" || handlerName == "" {
		if sel, ok := route.HandlerExpr.(*ast.SelectorExpr); ok {
			handlerName = sel.Sel.Name
		} else if ident, ok := route.HandlerExpr.(*ast.Ident); ok {
			handlerName = ident.Name
		}
	}

	// 5. Extract contract (params, body, responses).
	req, responses, unresolved := contract.ExtractContract(route, handlerNode, info, paramNames, pkgs)

	// 6. Map body types using the struct mapper.
	pkg := findPackageForRoute(route, pkgs)
	if req.Body != nil {
		mapped := resolveAndMapType(req.Body, info, pkg, pkgs)
		if mapped != nil {
			req.Body = mapped
		}
	}
	for i, resp := range responses {
		if resp.Body != nil {
			mapped := resolveAndMapType(resp.Body, info, pkg, pkgs)
			if mapped != nil {
				responses[i].Body = mapped
			}
		}
	}

	// 7. Detect auth from middleware chain.
	authDef := auth.DetectAuth(route.Middlewares, info, pkgs)

	// 8. Infer summary and tags.
	summary := model.InferSummary(handlerName)
	tags := []string{model.InferTag(handlerName)}
	if tags[0] == "" {
		tags = nil
	}

	qualifiedName := handlerName
	if handlerPkg != "" {
		qualifiedName = handlerPkg + "." + handlerName
	}

	ep := model.EndpointDef{
		Method:      route.Method,
		Path:        route.Path,
		Summary:     summary,
		HandlerName: qualifiedName,
		Package:     handlerPkg,
		File:        handlerFile,
		Line:        handlerLine,
		Auth:        authDef,
		Request:     req,
		Responses:   responses,
		Tags:        tags,
		Unresolved:  unresolved,
	}

	return ep, nil
}

// findInfoForRoute returns the types.Info for the package containing the route.
func findInfoForRoute(route extractor.RawRoute, pkgs []*packages.Package) *types.Info {
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, f := range pkg.GoFiles {
			if f == route.File {
				return pkg.TypesInfo
			}
		}
	}
	// Fallback: return the first package with TypesInfo.
	for _, pkg := range pkgs {
		if pkg.TypesInfo != nil {
			return pkg.TypesInfo
		}
	}
	return nil
}

// findPackageForRoute returns the *packages.Package containing the route.
func findPackageForRoute(route extractor.RawRoute, pkgs []*packages.Package) *packages.Package {
	for _, pkg := range pkgs {
		for _, f := range pkg.GoFiles {
			if f == route.File {
				return pkg
			}
		}
	}
	if len(pkgs) > 0 {
		return pkgs[0]
	}
	return nil
}

// resolveAndMapType looks up the types.Type for a TypeDef reference and maps it fully.
func resolveAndMapType(td *model.TypeDef, info *types.Info, pkg *packages.Package, pkgs []*packages.Package) *model.TypeDef {
	if td == nil || pkg == nil {
		return nil
	}

	// Try to find the type by name in the package scope.
	t := lookupType(td.Name, td.Package, pkgs)
	if t == nil {
		return nil
	}

	mapped := mapper.MapType(t, pkg)
	return &mapped
}

// lookupType finds a types.Type by name and package path across all loaded packages.
func lookupType(name, pkgPath string, pkgs []*packages.Package) types.Type {
	for _, pkg := range pkgs {
		if pkg.Types == nil {
			continue
		}
		// Match by package path if we have one.
		if pkgPath != "" && pkg.Types.Path() != pkgPath {
			continue
		}
		obj := pkg.Types.Scope().Lookup(name)
		if obj != nil {
			return obj.Type()
		}
	}
	// If pkgPath was set and didn't match root packages, search deps.
	var found types.Type
	for _, pkg := range pkgs {
		packages.Visit([]*packages.Package{pkg}, func(p *packages.Package) bool {
			if found != nil {
				return false
			}
			if p.Types == nil {
				return true
			}
			if pkgPath != "" && p.Types.Path() != pkgPath {
				return true
			}
			obj := p.Types.Scope().Lookup(name)
			if obj != nil {
				found = obj.Type()
				return false
			}
			return true
		}, nil)
		if found != nil {
			return found
		}
	}
	return nil
}

// posToFileLine converts a token.Pos to file path and line number.
func posToFileLine(pos token.Pos, pkgs []*packages.Package) (string, int) {
	if !pos.IsValid() {
		return "", 0
	}
	for _, pkg := range pkgs {
		if pkg.Fset != nil {
			position := pkg.Fset.Position(pos)
			if position.IsValid() {
				return position.Filename, position.Line
			}
		}
	}
	return "", 0
}
