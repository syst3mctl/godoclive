package contract

import (
	"go/ast"
	"go/types"
	"regexp"
	"strings"

	"github.com/dicki/godoclive/internal/model"
	"github.com/dicki/godoclive/internal/resolver"
)

var pathParamPattern = regexp.MustCompile(`\{([^}]+)\}`)

// ExtractPathParams parses {param} segments from a route pattern and produces
// an initial ParamDef list. If fnBody and info are provided, the handler body
// is scanned for type upgrade evidence (strconv.Atoi → integer, uuid.Parse → uuid).
func ExtractPathParams(pattern string, fnBody *ast.BlockStmt, info *types.Info, paramNames resolver.HandlerParamNames) []model.ParamDef {
	matches := pathParamPattern.FindAllStringSubmatch(pattern, -1)
	if len(matches) == 0 {
		return nil
	}

	params := make([]model.ParamDef, 0, len(matches))
	for _, m := range matches {
		name := m[1]
		params = append(params, model.ParamDef{
			Name:     name,
			In:       "path",
			Required: true,
			Type:     inferPathParamType(name),
			Example:  exampleForPathParam(name),
		})
	}

	if fnBody != nil && info != nil {
		upgradePathParamTypes(params, fnBody, paramNames)
	}

	return params
}

// inferPathParamType guesses a type from the parameter name convention.
func inferPathParamType(name string) string {
	lower := strings.ToLower(name)
	if lower == "id" || strings.HasSuffix(lower, "id") || strings.HasSuffix(lower, "_id") {
		return "uuid"
	}
	if lower == "page" || lower == "limit" || lower == "offset" || lower == "count" {
		return "integer"
	}
	return "string"
}

// exampleForPathParam produces an example value based on name heuristics.
func exampleForPathParam(name string) string {
	lower := strings.ToLower(name)
	if lower == "id" || strings.HasSuffix(lower, "id") || strings.HasSuffix(lower, "_id") {
		return "123e4567-e89b-12d3-a456-426614174000"
	}
	if lower == "slug" {
		return "my-resource"
	}
	if lower == "username" {
		return "johndoe"
	}
	if lower == "page" {
		return "1"
	}
	if lower == "limit" || lower == "offset" || lower == "count" {
		return "10"
	}
	return "example"
}

// upgradePathParamTypes scans the handler body for calls like
// strconv.Atoi(chi.URLParam(r, "id")) that reveal the actual type.
func upgradePathParamTypes(params []model.ParamDef, body *ast.BlockStmt, pn resolver.HandlerParamNames) {
	// Build a lookup for quick param access.
	paramIdx := make(map[string]int, len(params))
	for i, p := range params {
		paramIdx[p.Name] = i
	}

	// Track variables assigned from mux.Vars(r) for indirect access patterns:
	//   vars := mux.Vars(r)
	//   id := vars["id"]
	muxVarsIdents := make(map[string]bool)

	// Also track variable→param name mappings for vars["id"] → variable "id".
	varToParam := make(map[string]string)

	// First pass: find mux.Vars() assignments and vars["key"] assignments.
	for _, stmt := range body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) == 0 || len(assign.Rhs) == 0 {
			continue
		}
		lhs, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			continue
		}

		// vars := mux.Vars(r)
		if call, ok := assign.Rhs[0].(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if ident.Name == "mux" && sel.Sel.Name == "Vars" {
						muxVarsIdents[lhs.Name] = true
						continue
					}
				}
			}
		}

		// varName := c.Params("id") / c.Param("id") / chi.URLParam(r, "id") — direct assignment
		if rhs, ok := assign.Rhs[0].(*ast.CallExpr); ok {
			if name := extractPathParamName(rhs, pn); name != "" {
				varToParam[lhs.Name] = name
			}
		}

		// id := vars["key"] or id := mux.Vars(r)["key"]
		if idx, ok := assign.Rhs[0].(*ast.IndexExpr); ok {
			// Direct: mux.Vars(r)["key"]
			if name := extractGorillaPathParam(assign.Rhs[0]); name != "" {
				varToParam[lhs.Name] = name
				continue
			}
			// Indirect: vars["key"] where vars is a mux.Vars result
			if ident, ok := idx.X.(*ast.Ident); ok && muxVarsIdents[ident.Name] {
				if key := extractStringLit(idx.Index); key != "" {
					varToParam[lhs.Name] = key
				}
			}
		}
	}

	// Walk the body looking for strconv.Atoi / strconv.ParseInt / uuid.Parse
	// wrapping a path param accessor.
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		fnName := callFuncName(call)
		switch fnName {
		case "strconv.Atoi", "strconv.ParseInt", "strconv.ParseUint":
			if len(call.Args) >= 1 {
				if name := extractPathParamName(call.Args[0], pn); name != "" {
					if idx, ok := paramIdx[name]; ok {
						params[idx].Type = "integer"
					}
				}
				// Check gorilla mux.Vars(r)["key"] direct pattern.
				if name := extractGorillaPathParam(call.Args[0]); name != "" {
					if idx, ok := paramIdx[name]; ok {
						params[idx].Type = "integer"
					}
				}
				// Check indirect variable pattern: strconv.Atoi(id) where id came from vars["id"].
				if ident, ok := call.Args[0].(*ast.Ident); ok {
					if paramName, ok := varToParam[ident.Name]; ok {
						if idx, ok := paramIdx[paramName]; ok {
							params[idx].Type = "integer"
						}
					}
				}
			}
		case "uuid.Parse", "uuid.MustParse":
			if len(call.Args) >= 1 {
				if name := extractPathParamName(call.Args[0], pn); name != "" {
					if idx, ok := paramIdx[name]; ok {
						params[idx].Type = "uuid"
					}
				}
				if name := extractGorillaPathParam(call.Args[0]); name != "" {
					if idx, ok := paramIdx[name]; ok {
						params[idx].Type = "uuid"
					}
				}
				if ident, ok := call.Args[0].(*ast.Ident); ok {
					if paramName, ok := varToParam[ident.Name]; ok {
						if idx, ok := paramIdx[paramName]; ok {
							params[idx].Type = "uuid"
						}
					}
				}
			}
		}
		return true
	})
}

// extractPathParamName checks if expr is a call to chi.URLParam(r, "name")
// or c.Param("name") and returns the param name.
func extractPathParamName(expr ast.Expr, pn resolver.HandlerParamNames) string {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return ""
	}

	// chi.URLParam(r, "name")
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == "chi" && sel.Sel.Name == "URLParam" && len(call.Args) == 2 {
				return extractStringLit(call.Args[1])
			}
		}
	}

	// c.Param("name") — gin
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == pn.GinCtx && sel.Sel.Name == "Param" && len(call.Args) == 1 {
				return extractStringLit(call.Args[0])
			}
		}
	}

	// r.PathValue("name") — Go 1.22+ stdlib
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == pn.Request && sel.Sel.Name == "PathValue" && len(call.Args) == 1 {
				return extractStringLit(call.Args[0])
			}
		}
	}

	// c.Param("name") — echo
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == pn.EchoCtx && sel.Sel.Name == "Param" && len(call.Args) == 1 {
				return extractStringLit(call.Args[0])
			}
		}
	}

	// c.Params("name") — fiber (plural)
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == pn.FiberCtx && sel.Sel.Name == "Params" && len(call.Args) == 1 {
				return extractStringLit(call.Args[0])
			}
		}
	}

	return ""
}

// extractGorillaPathParam checks if expr is mux.Vars(r)["name"] (IndexExpr)
// and returns the param name.
func extractGorillaPathParam(expr ast.Expr) string {
	idx, ok := expr.(*ast.IndexExpr)
	if !ok {
		return ""
	}
	// Check that X is mux.Vars(r)
	call, ok := idx.X.(*ast.CallExpr)
	if !ok {
		return ""
	}
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == "mux" && sel.Sel.Name == "Vars" {
				return extractStringLit(idx.Index)
			}
		}
	}
	return ""
}

// callFuncName returns the qualified name of a call expression, e.g. "strconv.Atoi".
func callFuncName(call *ast.CallExpr) string {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			return ident.Name + "." + sel.Sel.Name
		}
	}
	return ""
}

// extractStringLit extracts a string value from a *ast.BasicLit.
func extractStringLit(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok {
		return ""
	}
	return strings.Trim(lit.Value, `"`)
}
