package contract

import (
	"go/ast"
	"go/types"

	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/resolver"
)

// ExtractCookies walks a handler body and detects the cookies it reads.
//
// A cookie is an input to the endpoint exactly as a header or a query
// parameter is — OpenAPI gives it its own `in: cookie` location — and a session
// or CSRF cookie a handler requires is often the difference between a request
// that works and one that 401s. Reading them out of the Authorization header's
// shadow is why they are extracted separately from auth detection.
//
// Each router spells the accessor differently but they agree on the shape: one
// string literal naming the cookie.
//
//	r.Cookie("session")        net/http, and so chi and gorilla/mux
//	c.Cookie("session")        gin and echo
//	c.Cookies("session")       fiber
//	c.Request.Cookie("session")  gin, reaching through to the raw request
func ExtractCookies(body *ast.BlockStmt, info *types.Info, paramNames resolver.HandlerParamNames) []model.ParamDef {
	if body == nil || info == nil {
		return nil
	}

	// receiver name → the method spelled on it
	accessors := []struct {
		recv   string
		method string
	}{
		{paramNames.Request, "Cookie"},
		{paramNames.GinCtx, "Cookie"},
		{paramNames.EchoCtx, "Cookie"},
		{paramNames.FiberCtx, "Cookies"},
	}

	var params []model.ParamDef
	seen := make(map[string]bool)

	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		params = append(params, model.ParamDef{
			Name: name,
			In:   "cookie",
			Type: "string",
			// A handler reads a cookie and decides what to do when it is
			// absent. Nothing in the read itself says the request is refused
			// without it, so requiredness is left unclaimed rather than guessed.
			Required: false,
		})
	}

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		for _, a := range accessors {
			if name, ok := matchCookieCall(call, a.recv, a.method); ok {
				add(name)
				return false
			}
		}
		// c.Request.Cookie("session")
		if name, ok := matchNestedRequestCookie(call, paramNames.GinCtx); ok {
			add(name)
			return false
		}
		return true
	})

	return params
}

// matchCookieCall matches recv.method("name") for a single string literal.
func matchCookieCall(call *ast.CallExpr, recvName, method string) (string, bool) {
	if recvName == "" {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != method {
		return "", false
	}
	recv, ok := sel.X.(*ast.Ident)
	if !ok || recv.Name != recvName {
		return "", false
	}
	if len(call.Args) != 1 {
		return "", false
	}
	return extractStringLit(call.Args[0]), true
}

// matchNestedRequestCookie matches c.Request.Cookie("name").
func matchNestedRequestCookie(call *ast.CallExpr, ctxName string) (string, bool) {
	if ctxName == "" {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Cookie" {
		return "", false
	}
	reqSel, ok := sel.X.(*ast.SelectorExpr)
	if !ok || reqSel.Sel.Name != "Request" {
		return "", false
	}
	recv, ok := reqSel.X.(*ast.Ident)
	if !ok || recv.Name != ctxName {
		return "", false
	}
	if len(call.Args) != 1 {
		return "", false
	}
	return extractStringLit(call.Args[0]), true
}
