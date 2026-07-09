package contract

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"

	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/resolver"
)

// BodyResult holds the outcome of request body extraction.
type BodyResult struct {
	BodyType       types.Type // The resolved type of the request body struct (nil if none)
	ContentType    string     // "application/json", "multipart/form-data", or combo
	IsMultipart    bool
	FileParams     []model.ParamDef // File params from FormFile calls
	BindQueryType  types.Type       // Struct type from ShouldBindQuery — promotes fields to QueryParams
	BindHeaderType types.Type       // Struct type from ShouldBindHeader — promotes fields to Headers
	Unresolved     []string         // Anything that couldn't be determined
}

// ExtractBody walks a handler function body and detects request body patterns
// for both net/http (json.NewDecoder/Unmarshal) and gin (ShouldBindJSON, etc.).
//
// When pkgs is non-nil, net/http handlers additionally get ONE level of helper
// tracing (mirroring response extraction): a call like decodeJSON(w, r, &req)
// whose callee decodes the request body into one of its parameters resolves the
// body schema from the CALLER's concrete argument. Pass nil to disable tracing.
func ExtractBody(body *ast.BlockStmt, info *types.Info, paramNames resolver.HandlerParamNames, pkgs []*packages.Package) BodyResult {
	result := BodyResult{}
	if body == nil || info == nil {
		return result
	}

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// --- echo patterns ---

		// c.Bind(&req)
		if t, ok := matchGinBindMethod(call, info, paramNames.EchoCtx, "Bind"); ok {
			result.BodyType = t
			result.ContentType = "application/json"
			return false
		}

		// --- fiber patterns ---

		// c.BodyParser(&req)
		if t, ok := matchGinBindMethod(call, info, paramNames.FiberCtx, "BodyParser"); ok {
			result.BodyType = t
			result.ContentType = "application/json"
			return false
		}

		// c.QueryParser(&q) — promotes struct fields to QueryParams
		if t, ok := matchGinBindMethod(call, info, paramNames.FiberCtx, "QueryParser"); ok {
			result.BindQueryType = t
			return false
		}

		// --- net/http JSON patterns ---

		// json.NewDecoder(r.Body).Decode(&req)
		if t, ok := matchJSONDecoderDecode(call, info, paramNames.Request); ok {
			result.BodyType = t
			result.ContentType = "application/json"
			return false
		}

		// json.Unmarshal(body, &req)
		if t, ok := matchJSONUnmarshal(call, info); ok {
			result.BodyType = t
			result.ContentType = "application/json"
			return false
		}

		// --- gin JSON patterns ---

		// c.ShouldBindJSON(&req) / c.BindJSON(&req)
		if t, ok := matchGinBindJSON(call, info, paramNames.GinCtx); ok {
			result.BodyType = t
			result.ContentType = "application/json"
			return false
		}

		// c.ShouldBindQuery(&q) — promotes struct fields to QueryParams
		if t, ok := matchGinBindMethod(call, info, paramNames.GinCtx, "ShouldBindQuery"); ok {
			result.BindQueryType = t
			return false
		}

		// c.ShouldBindHeader(&q) — promotes struct fields to Headers
		if t, ok := matchGinBindMethod(call, info, paramNames.GinCtx, "ShouldBindHeader"); ok {
			result.BindHeaderType = t
			return false
		}

		// c.ShouldBind(&req) — ambiguous content type
		if t, ok := matchGinShouldBind(call, info, paramNames.GinCtx); ok {
			result.BodyType = t
			result.ContentType = "application/json | multipart/form-data"
			result.Unresolved = append(result.Unresolved, "ShouldBind content type is ambiguous")
			return false
		}

		// --- Multipart patterns ---

		// r.FormFile("name") or c.FormFile("name")
		if name, ok := matchFormFile(call, paramNames); ok {
			result.IsMultipart = true
			result.ContentType = "multipart/form-data"
			result.FileParams = append(result.FileParams, model.ParamDef{
				Name: name,
				In:   "body",
				Type: "file",
			})
			return false
		}

		// r.ParseMultipartForm(...)
		if matchParseMultipartForm(call, paramNames.Request) {
			result.IsMultipart = true
			result.ContentType = "multipart/form-data"
			return false
		}

		// c.MultipartForm()
		if matchGinMultipartForm(call, paramNames.GinCtx) {
			result.IsMultipart = true
			result.ContentType = "multipart/form-data"
			return false
		}

		// --- Helper tracing (net/http, one level only) ---

		// decodeJSON(w, r, &req)-style helpers: the decode lives in a shared
		// function, so none of the direct matchers above fire in the handler.
		if pkgs != nil && result.BodyType == nil {
			if t, ok := traceBodyHelper(call, info, paramNames, pkgs); ok {
				result.BodyType = t
				result.ContentType = "application/json"
				return false
			}
		}

		return true
	})

	return result
}

// traceBodyHelper resolves a handler-level call that passes the *http.Request
// along (decodeJSON(w, r, &req), bind(r, &req), …) to its function declaration
// and scans that body — ONE level, no deeper recursion — for the net/http JSON
// decode patterns. When the helper decodes into one of its own parameters
// (typically `dst any`), the body type is mapped back to the CALLER's concrete
// argument (&req → req's struct type), the same param-index mapping response
// tracing uses; a helper that decodes into a concrete local type contributes
// that type directly. Interface-typed results are rejected — `any` documents
// nothing.
func traceBodyHelper(call *ast.CallExpr, info *types.Info, pn resolver.HandlerParamNames, pkgs []*packages.Package) (types.Type, bool) {
	if pn.Request == "" || len(call.Args) == 0 {
		return nil, false
	}

	// The helper must receive the request to be able to read its body.
	carriesRequest := false
	for _, arg := range call.Args {
		if id, ok := arg.(*ast.Ident); ok && id.Name == pn.Request {
			carriesRequest = true
			break
		}
	}
	if !carriesRequest {
		return nil, false
	}

	// Resolve the callee, skipping stdlib/router packages.
	var obj types.Object
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		obj = info.Uses[fn]
	case *ast.SelectorExpr:
		obj = info.Uses[fn.Sel]
	default:
		return nil, false
	}
	fnObj, ok := obj.(*types.Func)
	if !ok || fnObj.Pkg() == nil || skipPackages[fnObj.Pkg().Path()] {
		return nil, false
	}

	fd := findHelperFuncDecl(fnObj, pkgs)
	if fd == nil || fd.Body == nil {
		return nil, false
	}

	// The helper body's AST is type-checked in the helper package's TypesInfo
	// (see helperFuncInfo) — the caller's info yields nothing cross-package.
	helperInfo := helperFuncInfo(fnObj, pkgs)
	if helperInfo == nil {
		helperInfo = info
	}
	helperPN := resolver.ResolveHandlerParams(fd.Type, helperInfo)

	dest := bodyDecodeDest(fd.Body, helperPN.Request)
	if dest == nil {
		return nil, false
	}

	// Strip a leading & so `&dst` and `dst` map the same way.
	if unary, ok := dest.(*ast.UnaryExpr); ok {
		dest = unary.X
	}

	// Preferred path: the decode target names a helper PARAMETER — resolve the
	// caller's matching argument for the concrete schema type.
	if id, ok := dest.(*ast.Ident); ok {
		if callerIdx, found := helperParamIndex(fd)[id.Name]; found && callerIdx < len(call.Args) {
			if t, ok := extractArgType(call.Args[callerIdx], info); ok && !isInterfaceType(t) {
				return t, true
			}
			return nil, false
		}
	}

	// Fallback: the helper decodes into a concrete local — use its own type.
	if t, ok := extractArgType(dest, helperInfo); ok && !isInterfaceType(t) {
		return t, true
	}
	return nil, false
}

// bodyDecodeDest finds the destination expression of the first net/http JSON
// decode inside a helper body: json.NewDecoder(<req>.Body).Decode(dst) (when the
// helper has a request param) or json.Unmarshal(raw, dst). The Unmarshal form is
// accepted ONLY when the helper body also touches <req>.Body somewhere (the
// read-then-unmarshal idiom reads it via io.ReadAll/MaxBytesReader) — otherwise a
// helper that unmarshals something unrelated (an upstream response, a config
// blob) would fabricate a request schema. Nested function literals are scanned
// (helpers often wrap the decode in a closure), but no further function CALLS
// are followed — one level only.
func bodyDecodeDest(body *ast.BlockStmt, reqName string) ast.Expr {
	// The read-then-unmarshal guard: does this helper read the request body at all?
	readsBody := reqName != "" && mentionsRequestBody(body, reqName)

	var dest ast.Expr
	ast.Inspect(body, func(n ast.Node) bool {
		if dest != nil {
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
		case "Decode":
			// json.NewDecoder(<reqName>.Body).Decode(dst) — the decoder argument may
			// also be a MaxBytesReader-wrapped body; accept any argument expression
			// that mentions <reqName>.Body.
			newDecCall, ok := sel.X.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			newDecSel, ok := newDecCall.Fun.(*ast.SelectorExpr)
			if !ok || newDecSel.Sel.Name != "NewDecoder" {
				return true
			}
			if pkg, ok := newDecSel.X.(*ast.Ident); !ok || pkg.Name != "json" {
				return true
			}
			if reqName == "" || len(newDecCall.Args) != 1 || !mentionsRequestBody(newDecCall.Args[0], reqName) {
				return true
			}
			dest = call.Args[0]
			return false

		case "Unmarshal":
			if !readsBody {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "json" && len(call.Args) == 2 {
				dest = call.Args[1]
				return false
			}
		}
		return true
	})
	return dest
}

// mentionsRequestBody reports whether node contains <reqName>.Body anywhere —
// covering both a bare r.Body and wrapped forms like
// http.MaxBytesReader(w, r.Body, n) or io.LimitReader(r.Body, n).
func mentionsRequestBody(node ast.Node, reqName string) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Body" {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == reqName {
			found = true
			return false
		}
		return true
	})
	return found
}

// matchJSONDecoderDecode matches json.NewDecoder(r.Body).Decode(&req).
func matchJSONDecoderDecode(call *ast.CallExpr, info *types.Info, reqName string) (types.Type, bool) {
	if reqName == "" {
		return nil, false
	}
	// call.Fun should be a SelectorExpr: <expr>.Decode
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Decode" {
		return nil, false
	}

	// sel.X should be json.NewDecoder(r.Body)
	newDecCall, ok := sel.X.(*ast.CallExpr)
	if !ok {
		return nil, false
	}

	newDecSel, ok := newDecCall.Fun.(*ast.SelectorExpr)
	if !ok || newDecSel.Sel.Name != "NewDecoder" {
		return nil, false
	}

	// Check it's the json package.
	ident, ok := newDecSel.X.(*ast.Ident)
	if !ok || ident.Name != "json" {
		return nil, false
	}

	// Check the argument is r.Body.
	if len(newDecCall.Args) != 1 {
		return nil, false
	}
	bodySel, ok := newDecCall.Args[0].(*ast.SelectorExpr)
	if !ok || bodySel.Sel.Name != "Body" {
		return nil, false
	}
	bodyRecv, ok := bodySel.X.(*ast.Ident)
	if !ok || bodyRecv.Name != reqName {
		return nil, false
	}

	// Extract the type of Decode's argument.
	if len(call.Args) == 1 {
		return extractArgType(call.Args[0], info)
	}
	return nil, false
}

// matchJSONUnmarshal matches json.Unmarshal(body, &req).
func matchJSONUnmarshal(call *ast.CallExpr, info *types.Info) (types.Type, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Unmarshal" {
		return nil, false
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != "json" {
		return nil, false
	}

	if len(call.Args) == 2 {
		return extractArgType(call.Args[1], info)
	}
	return nil, false
}

// matchGinBindMethod matches c.<method>(&req) for a specific gin bind method.
func matchGinBindMethod(call *ast.CallExpr, info *types.Info, ginCtx, method string) (types.Type, bool) {
	if ginCtx == "" {
		return nil, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != method {
		return nil, false
	}
	recv, ok := sel.X.(*ast.Ident)
	if !ok || recv.Name != ginCtx {
		return nil, false
	}
	if len(call.Args) == 1 {
		return extractArgType(call.Args[0], info)
	}
	return nil, false
}

// matchGinBindJSON matches c.ShouldBindJSON(&req) or c.BindJSON(&req).
func matchGinBindJSON(call *ast.CallExpr, info *types.Info, ginCtx string) (types.Type, bool) {
	if t, ok := matchGinBindMethod(call, info, ginCtx, "ShouldBindJSON"); ok {
		return t, true
	}
	return matchGinBindMethod(call, info, ginCtx, "BindJSON")
}

// matchGinShouldBind matches c.ShouldBind(&req).
func matchGinShouldBind(call *ast.CallExpr, info *types.Info, ginCtx string) (types.Type, bool) {
	if ginCtx == "" {
		return nil, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "ShouldBind" {
		return nil, false
	}
	recv, ok := sel.X.(*ast.Ident)
	if !ok || recv.Name != ginCtx {
		return nil, false
	}
	if len(call.Args) == 1 {
		return extractArgType(call.Args[0], info)
	}
	return nil, false
}

// matchFormFile matches r.FormFile("name") or c.FormFile("name").
func matchFormFile(call *ast.CallExpr, pn resolver.HandlerParamNames) (string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "FormFile" {
		return "", false
	}
	recv, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	if recv.Name != pn.Request && recv.Name != pn.GinCtx {
		return "", false
	}
	if len(call.Args) == 1 {
		if v := extractStringLit(call.Args[0]); v != "" {
			return v, true
		}
	}
	return "", false
}

// matchParseMultipartForm matches r.ParseMultipartForm(...).
func matchParseMultipartForm(call *ast.CallExpr, reqName string) bool {
	if reqName == "" {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "ParseMultipartForm" {
		return false
	}
	recv, ok := sel.X.(*ast.Ident)
	return ok && recv.Name == reqName
}

// matchGinMultipartForm matches c.MultipartForm().
func matchGinMultipartForm(call *ast.CallExpr, ginCtx string) bool {
	if ginCtx == "" {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "MultipartForm" {
		return false
	}
	recv, ok := sel.X.(*ast.Ident)
	return ok && recv.Name == ginCtx
}

// extractArgType gets the types.Type of an argument expression. If the
// expression is a unary & (address-of), it dereferences the pointer to get
// the underlying struct type.
func extractArgType(expr ast.Expr, info *types.Info) (types.Type, bool) {
	// Handle &req — unary address-of.
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		t := info.TypeOf(unary.X)
		if t != nil {
			return t, true
		}
	}
	// Fallback: just get the type directly.
	t := info.TypeOf(expr)
	if t == nil {
		return nil, false
	}
	// Dereference pointer if needed.
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem(), true
	}
	return t, true
}
