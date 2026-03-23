package contract

import (
	"go/ast"
	"go/types"

	"github.com/dicki/godoclive/internal/model"
	"github.com/dicki/godoclive/internal/resolver"
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
func ExtractBody(body *ast.BlockStmt, info *types.Info, paramNames resolver.HandlerParamNames) BodyResult {
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

		return true
	})

	return result
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
