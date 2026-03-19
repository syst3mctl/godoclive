package contract

import (
	"go/ast"
	"go/types"

	"github.com/dicki/godoclive/internal/extractor"
	"github.com/dicki/godoclive/internal/model"
	"github.com/dicki/godoclive/internal/resolver"
	"golang.org/x/tools/go/packages"
)

// ExtractContract is the top-level orchestrator that calls all sub-extractors
// in order and assembles the complete request and response contract for a
// single endpoint.
//
// fn should be either *ast.FuncDecl or *ast.FuncLit.
func ExtractContract(
	route extractor.RawRoute,
	fn ast.Node,
	info *types.Info,
	paramNames resolver.HandlerParamNames,
	pkgs []*packages.Package,
) (model.RequestDef, []model.ResponseDef, []string) {

	var body *ast.BlockStmt
	var fnType *ast.FuncType

	switch f := fn.(type) {
	case *ast.FuncDecl:
		body = f.Body
		fnType = f.Type
	case *ast.FuncLit:
		body = f.Body
		fnType = f.Type
	}

	// If we couldn't get param names from the passed-in value, resolve them now.
	if paramNames == (resolver.HandlerParamNames{}) && fnType != nil {
		paramNames = resolver.ResolveHandlerParams(fnType, info)
	}

	var unresolved []string

	// 1. Path parameters.
	pathParams := ExtractPathParams(route.Path, body, info, paramNames)

	// 2. Query parameters.
	queryParams := ExtractQueryParams(body, info, paramNames)

	// 3. Headers.
	headers := ExtractHeaders(body, info, paramNames)

	// 4. Request body.
	bodyResult := ExtractBody(body, info, paramNames)
	unresolved = append(unresolved, bodyResult.Unresolved...)

	// Promote ShouldBindQuery fields to QueryParams.
	if bodyResult.BindQueryType != nil {
		promoted := promoteStructFields(bodyResult.BindQueryType, "form", "query")
		queryParams = append(queryParams, promoted...)
	}

	// Promote ShouldBindHeader fields to Headers.
	if bodyResult.BindHeaderType != nil {
		promoted := promoteStructFields(bodyResult.BindHeaderType, "header", "header")
		headers = append(headers, promoted...)
	}

	// Deduplicate: remove any query param whose name matches a path param.
	queryParams = deduplicateParams(pathParams, queryParams)

	req := model.RequestDef{
		PathParams:  pathParams,
		QueryParams: queryParams,
		Headers:     headers,
		ContentType: bodyResult.ContentType,
		IsMultipart: bodyResult.IsMultipart,
	}

	// If body extraction found file params, add them as path params might
	// contain file uploads.
	if len(bodyResult.FileParams) > 0 {
		// File params are special — they go alongside the body info.
		// For multipart endpoints, we don't set Body (it's the files).
		if bodyResult.BodyType == nil {
			// Pure file upload — no JSON body struct.
			req.Body = nil
		}
	}

	// Body type will be mapped to a TypeDef by the struct mapper later.
	// For now, store a reference if we have one.
	if bodyResult.BodyType != nil {
		req.Body = typeRefDef(bodyResult.BodyType)
	}

	// 5. Responses.
	responses, respUnresolved := ExtractResponses(body, info, paramNames, pkgs)
	unresolved = append(unresolved, respUnresolved...)

	return req, responses, unresolved
}

// deduplicateParams removes query params whose name matches a path param.
// This avoids double-reporting when r.FormValue() is used for a path param.
func deduplicateParams(pathParams, queryParams []model.ParamDef) []model.ParamDef {
	if len(pathParams) == 0 || len(queryParams) == 0 {
		return queryParams
	}
	pathNames := make(map[string]bool, len(pathParams))
	for _, p := range pathParams {
		pathNames[p.Name] = true
	}
	var filtered []model.ParamDef
	for _, q := range queryParams {
		if !pathNames[q.Name] {
			filtered = append(filtered, q)
		}
	}
	return filtered
}
