package contract

import (
	"go/ast"
	"go/types"

	"github.com/syst3mctl/godoclive/internal/extractor"
	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/resolver"
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
