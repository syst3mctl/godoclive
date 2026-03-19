package pipeline

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/dicki/godoclive/internal/auth"
	"github.com/dicki/godoclive/internal/config"
	"github.com/dicki/godoclive/internal/contract"
	"github.com/dicki/godoclive/internal/detector"
	"github.com/dicki/godoclive/internal/extractor"
	"github.com/dicki/godoclive/internal/loader"
	"github.com/dicki/godoclive/internal/mapper"
	"github.com/dicki/godoclive/internal/model"
	"github.com/dicki/godoclive/internal/resolver"
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

	// 1b. Build a type index once so per-route lookups are O(1) instead of
	// O(N×deps) via repeated packages.Visit calls.
	typeIdx := buildTypeIndex(pkgs)

	// 2. Detect router.
	routerKind := detector.DetectRouter(pkgs)
	if routerKind == detector.RouterKindUnknown {
		return nil, fmt.Errorf("no supported router detected (expected chi, gin, gorilla/mux, echo, fiber, or net/http stdlib)")
	}

	// 3. Choose and run the appropriate extractor.
	var ext extractor.Extractor
	switch routerKind {
	case detector.RouterKindChi:
		ext = &extractor.ChiExtractor{}
	case detector.RouterKindGin:
		ext = &extractor.GinExtractor{}
	case detector.RouterKindStdlib:
		ext = &extractor.StdlibExtractor{}
	case detector.RouterKindGorilla:
		ext = &extractor.GorillaExtractor{}
	case detector.RouterKindEcho:
		ext = &extractor.EchoExtractor{}
	case detector.RouterKindFiber:
		ext = &extractor.FiberExtractor{}
	}

	routes, err := ext.Extract(pkgs)
	if err != nil {
		return nil, fmt.Errorf("extracting routes: %w", err)
	}

	// 4-8. Process each route into a full EndpointDef.
	var endpoints []model.EndpointDef
	for _, route := range routes {
		ep, err := processRoute(route, pkgs, typeIdx)
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
func processRoute(route extractor.RawRoute, pkgs []*packages.Package, typeIdx map[string]map[string]types.Type) (model.EndpointDef, error) {
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

	var deprecated bool

	// The handler may live in a different package than the route registration.
	// Use the TypesInfo from the handler's own package so that type lookups on
	// the handler's AST nodes (param types, local vars) resolve correctly.
	handlerInfo := info
	if funcDecl != nil {
		if hi := findInfoForFuncDecl(funcDecl, pkgs); hi != nil {
			handlerInfo = hi
		}
	}

	if funcDecl != nil {
		handlerNode = funcDecl
		paramNames = resolver.ResolveHandlerParams(funcDecl.Type, handlerInfo)
		handlerName = funcDecl.Name.Name
		handlerFile, handlerLine = posToFileLine(funcDecl.Pos(), pkgs)
		// Check for // Deprecated: comment on the handler.
		if funcDecl.Doc != nil {
			for _, comment := range funcDecl.Doc.List {
				if strings.Contains(comment.Text, "Deprecated:") {
					deprecated = true
					break
				}
			}
		}
	} else if funcLit != nil {
		handlerNode = funcLit
		paramNames = resolver.ResolveHandlerParams(funcLit.Type, handlerInfo)
		handlerName = "anonymous"
		handlerFile, handlerLine = posToFileLine(funcLit.Pos(), pkgs)
	}

	// Resolve package from the function's object if possible.
	if funcDecl != nil {
		if obj, ok := handlerInfo.Defs[funcDecl.Name]; ok && obj != nil {
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
	req, responses, unresolved := contract.ExtractContract(route, handlerNode, handlerInfo, paramNames, pkgs)

	// 6. Map body types using the struct mapper.
	pkg := findPackageForRoute(route, pkgs)
	if req.Body != nil {
		mapped := resolveAndMapType(req.Body, info, pkg, typeIdx)
		if mapped != nil {
			req.Body = mapped
		}
	}
	for i, resp := range responses {
		if resp.Body != nil {
			mapped := resolveAndMapType(resp.Body, info, pkg, typeIdx)
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
		// Fall back to the first meaningful path segment as tag.
		tags[0] = tagFromPath(route.Path)
	}
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
		Deprecated:  deprecated,
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
// typeIdx is the pre-built index from buildTypeIndex for O(1) lookups.
func resolveAndMapType(td *model.TypeDef, info *types.Info, pkg *packages.Package, typeIdx map[string]map[string]types.Type) *model.TypeDef {
	if td == nil || pkg == nil {
		return nil
	}

	t := lookupType(td.Name, td.Package, typeIdx)
	if t == nil {
		return nil
	}

	mapped := mapper.MapType(t, pkg)
	return &mapped
}

// buildTypeIndex builds a two-level map (pkgPath → typeName → types.Type) by
// visiting every package in the graph exactly once. Callers can then resolve
// any named type in O(1) rather than re-traversing the full package graph.
func buildTypeIndex(pkgs []*packages.Package) map[string]map[string]types.Type {
	idx := make(map[string]map[string]types.Type)
	packages.Visit(pkgs, func(p *packages.Package) bool {
		if p.Types == nil {
			return true
		}
		path := p.Types.Path()
		if _, exists := idx[path]; exists {
			return true // already indexed (diamond dependency)
		}
		scope := p.Types.Scope()
		names := scope.Names()
		m := make(map[string]types.Type, len(names))
		for _, name := range names {
			m[name] = scope.Lookup(name).Type()
		}
		idx[path] = m
		return true
	}, nil)
	return idx
}

// lookupType finds a types.Type by name and package path using the pre-built index.
func lookupType(name, pkgPath string, idx map[string]map[string]types.Type) types.Type {
	if pkgPath != "" {
		if m, ok := idx[pkgPath]; ok {
			return m[name] // may be nil if name not in that package
		}
		// pkgPath not found — fall through to name-only scan.
	}
	// Scan all packages when no pkgPath is given (or pkgPath wasn't indexed).
	for _, m := range idx {
		if t, ok := m[name]; ok {
			return t
		}
	}
	return nil
}

// tagFromPath extracts the first meaningful path segment as a fallback tag.
// e.g. "/api/users/{id}" → "users", "/health" → "health".
func tagFromPath(path string) string {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for _, seg := range segments {
		if seg == "" || seg == "api" || seg == "v1" || seg == "v2" || seg == "v3" {
			continue
		}
		if strings.HasPrefix(seg, "{") {
			continue
		}
		return seg
	}
	return ""
}

// findInfoForFuncDecl returns the TypesInfo for the package that contains fd by
// searching for it by pointer equality across all loaded packages. This is
// necessary when a handler is declared in a different package than the route
// registration: the route's info has no type data for the handler's AST nodes.
func findInfoForFuncDecl(fd *ast.FuncDecl, pkgs []*packages.Package) *types.Info {
	var result *types.Info
	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		if result != nil || pkg.TypesInfo == nil {
			return result == nil
		}
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				if decl == fd {
					result = pkg.TypesInfo
					return false
				}
			}
		}
		return true
	}, nil)
	return result
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
