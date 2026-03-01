package contract

import (
	"go/ast"
	"go/types"
	"strings"

	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/resolver"
)

// Headers to skip — handled by auth detection and content-type detection.
var skipHeaders = map[string]bool{
	"authorization": true,
	"content-type":  true,
}

// ExtractHeaders walks a handler function body and detects header access
// patterns for both net/http and gin.
func ExtractHeaders(body *ast.BlockStmt, info *types.Info, paramNames resolver.HandlerParamNames) []model.ParamDef {
	if body == nil || info == nil {
		return nil
	}

	var params []model.ParamDef
	seen := make(map[string]bool)

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// r.Header.Get("X-Key")
		if name, ok := matchHeaderGet(call, paramNames.Request); ok {
			if !skipHeaders[strings.ToLower(name)] && !seen[name] {
				params = append(params, model.ParamDef{Name: name, In: "header", Type: "string"})
				seen[name] = true
			}
			return false
		}

		// c.GetHeader("X-Key")
		if name, ok := matchGinGetHeader(call, paramNames.GinCtx); ok {
			if !skipHeaders[strings.ToLower(name)] && !seen[name] {
				params = append(params, model.ParamDef{Name: name, In: "header", Type: "string"})
				seen[name] = true
			}
			return false
		}

		// c.Request.Header.Get("X-Key")
		if name, ok := matchGinRequestHeaderGet(call, paramNames.GinCtx); ok {
			if !skipHeaders[strings.ToLower(name)] && !seen[name] {
				params = append(params, model.ParamDef{Name: name, In: "header", Type: "string"})
				seen[name] = true
			}
			return false
		}

		return true
	})

	return params
}

// matchHeaderGet matches r.Header.Get("X-Key").
func matchHeaderGet(call *ast.CallExpr, reqName string) (string, bool) {
	if reqName == "" {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Get" {
		return "", false
	}

	headerSel, ok := sel.X.(*ast.SelectorExpr)
	if !ok || headerSel.Sel.Name != "Header" {
		return "", false
	}

	recv, ok := headerSel.X.(*ast.Ident)
	if !ok || recv.Name != reqName {
		return "", false
	}

	if len(call.Args) == 1 {
		if v := extractStringLit(call.Args[0]); v != "" {
			return v, true
		}
	}
	return "", false
}

// matchGinGetHeader matches c.GetHeader("X-Key").
func matchGinGetHeader(call *ast.CallExpr, ginCtx string) (string, bool) {
	if ginCtx == "" {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "GetHeader" {
		return "", false
	}
	recv, ok := sel.X.(*ast.Ident)
	if !ok || recv.Name != ginCtx {
		return "", false
	}
	if len(call.Args) == 1 {
		if v := extractStringLit(call.Args[0]); v != "" {
			return v, true
		}
	}
	return "", false
}

// matchGinRequestHeaderGet matches c.Request.Header.Get("X-Key").
func matchGinRequestHeaderGet(call *ast.CallExpr, ginCtx string) (string, bool) {
	if ginCtx == "" {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Get" {
		return "", false
	}

	headerSel, ok := sel.X.(*ast.SelectorExpr)
	if !ok || headerSel.Sel.Name != "Header" {
		return "", false
	}

	reqSel, ok := headerSel.X.(*ast.SelectorExpr)
	if !ok || reqSel.Sel.Name != "Request" {
		return "", false
	}

	recv, ok := reqSel.X.(*ast.Ident)
	if !ok || recv.Name != ginCtx {
		return "", false
	}

	if len(call.Args) == 1 {
		if v := extractStringLit(call.Args[0]); v != "" {
			return v, true
		}
	}
	return "", false
}
