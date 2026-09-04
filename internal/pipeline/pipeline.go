package pipeline

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

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

	// 1b. Build a type index once so per-route lookups are O(1) instead of
	// O(N×deps) via repeated packages.Visit calls.
	typeIdx := buildTypeIndex(pkgs)

	// 2. Detect every router framework the project registers routes on. A
	// service may use more than one — a gin API beside a stdlib ServeMux for
	// health checks, or a chi tree that still mounts a legacy gorilla subtree.
	routerKinds := detector.DetectRouters(pkgs)
	if len(routerKinds) == 0 {
		return nil, fmt.Errorf("no supported router detected (expected chi, gin, gorilla/mux, echo, fiber, or net/http stdlib)")
	}

	// 3. Run the extractor for each detected framework and take the union.
	// Every extractor type-checks the receiver it registers on, so one running
	// against a project that only partly uses its framework yields nothing
	// rather than false routes.
	var routes []RawRouteSource
	for _, kind := range routerKinds {
		ext := extractorFor(kind)
		if ext == nil {
			continue
		}
		found, err := ext.Extract(pkgs)
		if err != nil {
			return nil, fmt.Errorf("extracting %s routes: %w", kind, err)
		}
		for _, r := range found {
			routes = append(routes, RawRouteSource{Route: r, Router: kind})
		}
	}
	routes = dedupeRoutes(routes)

	// 4-8. Process each route into a full EndpointDef.
	var endpoints []model.EndpointDef
	for _, src := range routes {
		route := src.Route
		ep, err := processRoute(route, pkgs, typeIdx)
		if err != nil {
			// Record the error as unresolved rather than failing the whole pipeline.
			endpoints = append(endpoints, model.EndpointDef{
				Method:     route.Method,
				Path:       route.Path,
				Router:     string(src.Router),
				File:       route.File,
				Line:       route.Line,
				Unresolved: []string{err.Error()},
			})
			continue
		}
		ep.Router = string(src.Router)
		endpoints = append(endpoints, ep)
	}

	// 9. Record the gaps only visible across the whole endpoint set.
	endpoints = annotateAnalysisGaps(endpoints)

	// 10. Apply config exclusions and overrides.
	if cfg != nil {
		endpoints = config.ApplyExclusions(endpoints, cfg.Exclude)
		endpoints = config.ApplyOverrides(endpoints, cfg.Overrides)
	}

	return endpoints, nil
}

// processRoute converts a single RawRoute into a fully-resolved EndpointDef.
func processRoute(route extractor.RawRoute, pkgs []*packages.Package, typeIdx typeIndex) (model.EndpointDef, error) {
	// Find the TypesInfo from the package that contains this route's file.
	// A route expanded through a house router wrapper carries the type info its
	// handler expression belongs to, which is the call site's package rather
	// than the wrapper's.
	info := route.HandlerInfo
	if info == nil {
		info = findInfoForRoute(route, pkgs)
	}
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
	var handlerDoc model.HandlerDoc

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
		// The handler's own doc comment is the best description of what it
		// does; a "Deprecated:" paragraph in it is also how Go marks the
		// endpoint as going away.
		handlerDoc = model.ParseHandlerDoc(funcDecl.Doc, funcDecl.Name.Name)
		deprecated = handlerDoc.Deprecated
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

	// 7. Detect auth from middleware chain. Middleware that could not be read
	// is reported rather than silently treated as "no auth".
	authDef, authNotes := auth.DetectAuth(route.Middlewares, info, pkgs)
	unresolved = append(unresolved, authNotes...)

	// 8. Take the summary from the handler's doc comment, falling back to
	// splitting its name when there is no comment to read.
	summary := handlerDoc.Summary
	if summary == "" {
		summary = model.InferSummary(handlerName)
	}
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

	// Caveats recorded during extraction (unknown group origin, empty path)
	// belong to the endpoint too.
	unresolved = append(route.Unresolved, unresolved...)

	ep := model.EndpointDef{
		Method:      route.Method,
		Path:        route.Path,
		Summary:     summary,
		Description: handlerDoc.Description,
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

// resolveAndMapType looks up the types.Type for a TypeDef reference and maps it
// fully. typeIdx is the pre-built index from buildTypeIndex for O(1) lookups.
//
// A reference is not always a bare named type: contract extraction also emits
// composed shapes — a slice of a reference, or the synthetic struct standing in
// for a gin.H literal whose values are themselves references — so the mapping
// walks into those and resolves each named type it finds.
func resolveAndMapType(td *model.TypeDef, info *types.Info, pkg *packages.Package, typeIdx typeIndex) *model.TypeDef {
	if td == nil || pkg == nil {
		return nil
	}

	// Synthetic struct (a map literal turned into a schema): map each field.
	if td.Kind == model.KindStruct && td.Name == "" && len(td.Fields) > 0 {
		out := *td
		out.Fields = make([]model.FieldDef, len(td.Fields))
		copy(out.Fields, td.Fields)
		for i := range out.Fields {
			if mapped := resolveAndMapType(&out.Fields[i].Type, info, pkg, typeIdx); mapped != nil {
				out.Fields[i].Type = *mapped
			}
		}
		return &out
	}

	if td.Kind == model.KindSlice && td.Elem != nil {
		elem := resolveAndMapType(td.Elem, info, pkg, typeIdx)
		if elem == nil {
			return nil
		}
		return &model.TypeDef{Kind: model.KindSlice, Elem: elem, IsPointer: td.IsPointer}
	}

	// Already-concrete leaves carry no name to look up.
	if td.Kind == model.KindPrimitive || td.Kind == model.KindInterface || td.Kind == model.KindMap {
		return td
	}

	t := typeIdx.lookup(td.Name, td.Package)
	if t == nil {
		return nil
	}

	mapped := mapper.MapType(t, pkg)
	mapped.IsPointer = mapped.IsPointer || td.IsPointer
	return &mapped
}

// typeIndex resolves a named type from the type-checked package graph.
//
// It holds package pointers rather than a pre-expanded name→type map: a scope
// lookup is already O(1), so enumerating every name of every package up front
// only spends time and memory materializing names nothing asks for. A service
// on gin and gorm reaches a few hundred packages, most of them irrelevant.
//
// The graph is walked through types.Package.Imports rather than
// packages.Package.Imports, which is what lets dependency types resolve
// without their source being parsed: export data gives the type checker
// complete scopes for everything the analyzed code refers to.
type typeIndex struct {
	byPath map[string]*types.Package
}

// buildTypeIndex collects every package reachable from the analyzed packages.
func buildTypeIndex(pkgs []*packages.Package) typeIndex {
	idx := typeIndex{byPath: make(map[string]*types.Package)}
	packages.Visit(pkgs, func(p *packages.Package) bool {
		idx.add(p.Types)
		return true
	}, nil)
	return idx
}

// add records tp and everything it imports, transitively.
func (idx typeIndex) add(tp *types.Package) {
	if tp == nil {
		return
	}
	if _, seen := idx.byPath[tp.Path()]; seen {
		return // already recorded (diamond dependency)
	}
	idx.byPath[tp.Path()] = tp
	for _, imp := range tp.Imports() {
		idx.add(imp)
	}
}

// lookup finds a types.Type by name and package path. A type reference carries
// the defining package whenever one is known; the name-only scan is the
// fallback for references that do not.
func (idx typeIndex) lookup(name, pkgPath string) types.Type {
	if name == "" {
		return nil
	}
	if pkgPath != "" {
		if tp, ok := idx.byPath[pkgPath]; ok {
			return scopeType(tp, name)
		}
		// pkgPath not found — fall through to the name-only scan.
	}
	for _, tp := range idx.byPath {
		if t := scopeType(tp, name); t != nil {
			return t
		}
	}
	return nil
}

// scopeType looks one name up in a package scope.
func scopeType(tp *types.Package, name string) types.Type {
	if tp == nil || tp.Scope() == nil {
		return nil
	}
	obj := tp.Scope().Lookup(name)
	if obj == nil {
		return nil
	}
	return obj.Type()
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

// annotateAnalysisGaps records the caveats that only become visible once every
// endpoint is known, so that coverage reporting reflects them.
//
// Two endpoints are an OpenAPI collision when they share a method and a path
// that differ only in the *names* of their template parameters: the
// specification treats /articles/{slug} and /articles/{id} as the same path,
// so one operation silently replaces the other in the generated document. A
// path that is empty or not rooted at "/" is likewise reported rather than
// emitted as if it were a real route.
func annotateAnalysisGaps(endpoints []model.EndpointDef) []model.EndpointDef {
	byKey := make(map[string][]int, len(endpoints))
	for i, ep := range endpoints {
		key := ep.Method + " " + normalizeTemplatedPath(ep.Path)
		byKey[key] = append(byKey[key], i)
	}

	for _, idxs := range byKey {
		if len(idxs) < 2 {
			continue
		}
		for _, i := range idxs {
			var others []string
			for _, j := range idxs {
				if j == i {
					continue
				}
				others = append(others, fmt.Sprintf("%s %s (%s:%d)",
					endpoints[j].Method, endpoints[j].Path, endpoints[j].File, endpoints[j].Line))
			}
			endpoints[i].Unresolved = append(endpoints[i].Unresolved, fmt.Sprintf(
				"openapi collision: %s %s collides with %s — paths differing only in parameter names are the same path in OpenAPI, so only one operation survives",
				endpoints[i].Method, endpoints[i].Path, strings.Join(others, ", ")))
		}
	}

	for i, ep := range endpoints {
		if ep.Path != "" && strings.HasPrefix(ep.Path, "/") {
			continue
		}
		if hasNotePrefix(ep.Unresolved, "empty route path") {
			continue // the extractor already explained this one, with context
		}
		if ep.Path == "" {
			endpoints[i].Unresolved = append(endpoints[i].Unresolved,
				"empty route path: no path could be resolved for this registration")
			continue
		}
		endpoints[i].Unresolved = append(endpoints[i].Unresolved, fmt.Sprintf(
			"malformed route path %q: a route path must start with %q", ep.Path, "/"))
	}

	return endpoints
}

// normalizeTemplatedPath replaces every {param} with a placeholder so that
// paths differing only in parameter names compare equal, matching OpenAPI's
// own definition of path identity.
func normalizeTemplatedPath(p string) string {
	segments := strings.Split(p, "/")
	for i, seg := range segments {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			segments[i] = "{}"
		}
	}
	return strings.Join(segments, "/")
}

// hasNotePrefix reports whether any note starts with prefix.
func hasNotePrefix(notes []string, prefix string) bool {
	for _, n := range notes {
		if strings.HasPrefix(n, prefix) {
			return true
		}
	}
	return false
}
