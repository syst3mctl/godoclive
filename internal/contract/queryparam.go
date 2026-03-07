package contract

import (
	"go/ast"
	"go/types"
	"strings"

	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/resolver"
)

// ExtractQueryParams walks a handler function body and detects query parameter
// access patterns for both net/http and gin.
func ExtractQueryParams(body *ast.BlockStmt, info *types.Info, paramNames resolver.HandlerParamNames) []model.ParamDef {
	if body == nil || info == nil {
		return nil
	}

	var params []model.ParamDef
	seen := make(map[string]bool)

	for i, stmt := range body.List {
		// Walk all expressions in this statement looking for query access patterns.
		ast.Inspect(stmt, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// --- net/http patterns ---

			// r.URL.Query().Get("key")
			if name, ok := matchURLQueryGet(call, paramNames.Request); ok && !seen[name] {
				p := model.ParamDef{Name: name, In: "query", Type: "string"}
				if isRequiredGuard(body.List, i, name) {
					p.Required = true
				}
				params = append(params, p)
				seen[name] = true
				return false
			}

			// r.FormValue("key")
			if name, ok := matchFormValue(call, paramNames.Request); ok && !seen[name] {
				p := model.ParamDef{Name: name, In: "query", Type: "string"}
				params = append(params, p)
				seen[name] = true
				return false
			}

			// --- gin patterns ---

			// c.Query("key")
			if name, ok := matchGinSimple(call, paramNames.GinCtx, "Query"); ok && !seen[name] {
				params = append(params, model.ParamDef{Name: name, In: "query", Type: "string"})
				seen[name] = true
				return false
			}

			// c.DefaultQuery("key", "default")
			if name, def, ok := matchGinDefaultQuery(call, paramNames.GinCtx); ok && !seen[name] {
				p := model.ParamDef{Name: name, In: "query", Type: "string", Default: &def}
				params = append(params, p)
				seen[name] = true
				return false
			}

			// c.QueryArray("key")
			if name, ok := matchGinSimple(call, paramNames.GinCtx, "QueryArray"); ok && !seen[name] {
				params = append(params, model.ParamDef{Name: name, In: "query", Type: "string", Multi: true})
				seen[name] = true
				return false
			}

			// c.QueryMap("key")
			if name, ok := matchGinSimple(call, paramNames.GinCtx, "QueryMap"); ok && !seen[name] {
				params = append(params, model.ParamDef{Name: name, In: "query", Type: "string", Multi: true})
				seen[name] = true
				return false
			}

			// --- echo patterns ---

			// c.QueryParam("key")
			if name, ok := matchGinSimple(call, paramNames.EchoCtx, "QueryParam"); ok && !seen[name] {
				params = append(params, model.ParamDef{Name: name, In: "query", Type: "string"})
				seen[name] = true
				return false
			}

			return true
		})
	}

	// Also check for r.URL.Query()["key"] index expressions.
	ast.Inspect(body, func(n ast.Node) bool {
		idx, ok := n.(*ast.IndexExpr)
		if !ok {
			return true
		}
		if name, ok := matchURLQueryIndex(idx, paramNames.Request); ok && !seen[name] {
			params = append(params, model.ParamDef{Name: name, In: "query", Type: "string", Multi: true})
			seen[name] = true
		}
		return true
	})

	return params
}

// matchURLQueryGet matches r.URL.Query().Get("key").
func matchURLQueryGet(call *ast.CallExpr, reqName string) (string, bool) {
	if reqName == "" {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Get" {
		return "", false
	}

	inner, ok := sel.X.(*ast.CallExpr)
	if !ok {
		return "", false
	}

	innerSel, ok := inner.Fun.(*ast.SelectorExpr)
	if !ok || innerSel.Sel.Name != "Query" {
		return "", false
	}

	urlSel, ok := innerSel.X.(*ast.SelectorExpr)
	if !ok || urlSel.Sel.Name != "URL" {
		return "", false
	}

	recv, ok := urlSel.X.(*ast.Ident)
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

// matchURLQueryIndex matches r.URL.Query()["key"].
func matchURLQueryIndex(idx *ast.IndexExpr, reqName string) (string, bool) {
	if reqName == "" {
		return "", false
	}
	// idx.X should be a CallExpr for r.URL.Query()
	inner, ok := idx.X.(*ast.CallExpr)
	if !ok {
		return "", false
	}

	innerSel, ok := inner.Fun.(*ast.SelectorExpr)
	if !ok || innerSel.Sel.Name != "Query" {
		return "", false
	}

	urlSel, ok := innerSel.X.(*ast.SelectorExpr)
	if !ok || urlSel.Sel.Name != "URL" {
		return "", false
	}

	recv, ok := urlSel.X.(*ast.Ident)
	if !ok || recv.Name != reqName {
		return "", false
	}

	if v := extractStringLit(idx.Index); v != "" {
		return v, true
	}
	return "", false
}

// matchFormValue matches r.FormValue("key").
func matchFormValue(call *ast.CallExpr, reqName string) (string, bool) {
	if reqName == "" {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "FormValue" {
		return "", false
	}
	recv, ok := sel.X.(*ast.Ident)
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

// matchGinSimple matches c.<method>("key") for Query, QueryArray, QueryMap.
func matchGinSimple(call *ast.CallExpr, ginCtx, method string) (string, bool) {
	if ginCtx == "" {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != method {
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

// matchGinDefaultQuery matches c.DefaultQuery("key", "default").
func matchGinDefaultQuery(call *ast.CallExpr, ginCtx string) (string, string, bool) {
	if ginCtx == "" {
		return "", "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "DefaultQuery" {
		return "", "", false
	}
	recv, ok := sel.X.(*ast.Ident)
	if !ok || recv.Name != ginCtx {
		return "", "", false
	}
	if len(call.Args) == 2 {
		name := extractStringLit(call.Args[0])
		def := extractStringLit(call.Args[1])
		if name != "" {
			return name, def, true
		}
	}
	return "", "", false
}

// isRequiredGuard checks if the statement immediately following the assignment
// at stmtIdx is an if-guard that checks val == "" and returns. This detects:
//
//	page := r.URL.Query().Get("page")
//	if page == "" { ... return }
func isRequiredGuard(stmts []ast.Stmt, stmtIdx int, paramName string) bool {
	// Find the variable name assigned at stmtIdx.
	varName := assignedVarName(stmts[stmtIdx], paramName)
	if varName == "" {
		return false
	}

	// Check the next statement for an if-guard.
	if stmtIdx+1 >= len(stmts) {
		return false
	}

	ifStmt, ok := stmts[stmtIdx+1].(*ast.IfStmt)
	if !ok {
		return false
	}

	// Check if the condition is varName == "".
	binExpr, ok := ifStmt.Cond.(*ast.BinaryExpr)
	if !ok {
		return false
	}

	ident, ok := binExpr.X.(*ast.Ident)
	if !ok || ident.Name != varName {
		return false
	}

	lit, ok := binExpr.Y.(*ast.BasicLit)
	if !ok || strings.Trim(lit.Value, `"`) != "" {
		return false
	}

	// Check if the if-body contains a return statement.
	return blockHasReturn(ifStmt.Body)
}

// assignedVarName extracts the variable name from an assignment/short-var-decl
// statement, but only if the RHS contains a query access call for the given
// param name. This prevents false-positive required-guard detection when the
// preceding statement assigns a different variable.
func assignedVarName(stmt ast.Stmt, paramName string) string {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || len(assign.Lhs) == 0 {
		return ""
	}
	ident, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return ""
	}

	// Verify the RHS actually contains a reference to paramName.
	found := false
	for _, rhs := range assign.Rhs {
		ast.Inspect(rhs, func(n ast.Node) bool {
			if found {
				return false
			}
			lit, ok := n.(*ast.BasicLit)
			if ok && strings.Trim(lit.Value, `"`) == paramName {
				found = true
				return false
			}
			return true
		})
	}
	if !found {
		return ""
	}
	return ident.Name
}

// blockHasReturn checks if a block statement contains a return statement.
func blockHasReturn(block *ast.BlockStmt) bool {
	if block == nil {
		return false
	}
	for _, stmt := range block.List {
		if _, ok := stmt.(*ast.ReturnStmt); ok {
			return true
		}
	}
	return false
}
