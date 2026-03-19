package contract

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/dicki/godoclive/internal/model"
	"github.com/dicki/godoclive/internal/resolver"
	"golang.org/x/tools/go/packages"
)

// Known packages whose functions should NOT be traced into.
var skipPackages = map[string]bool{
	"net/http":                   true,
	"encoding/json":             true,
	"github.com/go-chi/chi/v5":  true,
	"github.com/gin-gonic/gin":  true,
}

// traceHelper checks if a call expression is a helper function that takes a
// ResponseWriter (or *gin.Context) as its first argument and is not from a
// known stdlib/router package. If so, it enters the helper body ONE LEVEL ONLY
// and collects response events from it.
//
// Returns nil if the call is not a traceable helper.
func traceHelper(call *ast.CallExpr, info *types.Info, pn resolver.HandlerParamNames, pkgs []*packages.Package) ([]responseEvent, []string) {
	if len(call.Args) == 0 {
		return nil, nil
	}

	// Check if the first argument is the ResponseWriter or GinCtx.
	firstArgName := ""
	if ident, ok := call.Args[0].(*ast.Ident); ok {
		firstArgName = ident.Name
	}
	if firstArgName == "" {
		return nil, nil
	}

	isWriter := pn.Writer != "" && firstArgName == pn.Writer
	isGinCtx := pn.GinCtx != "" && firstArgName == pn.GinCtx
	if !isWriter && !isGinCtx {
		return nil, nil
	}

	// Resolve the called function.
	var obj types.Object

	switch fn := call.Fun.(type) {
	case *ast.Ident:
		obj = info.Uses[fn]
	case *ast.SelectorExpr:
		obj = info.Uses[fn.Sel]
	default:
		return nil, nil
	}

	if obj == nil {
		return nil, nil
	}

	fnObj, ok := obj.(*types.Func)
	if !ok {
		return nil, nil
	}

	// Skip stdlib and router packages.
	if fnObj.Pkg() == nil {
		return nil, nil
	}
	if skipPackages[fnObj.Pkg().Path()] {
		return nil, nil
	}

	// Find the FuncDecl in the loaded packages.
	fd := findHelperFuncDecl(fnObj, pkgs)
	if fd == nil || fd.Body == nil {
		return nil, []string{
			fmt.Sprintf("response: helper function %s() could not be traced — body type unknown", fnObj.Name()),
		}
	}

	// Resolve the helper's own parameter names.
	helperPN := resolver.ResolveHandlerParams(fd.Type, info)

	// Collect response events from the helper body (isHelper=true prevents further recursion).
	var unresolved []string
	events := collectResponseEvents(fd.Body, info, helperPN, pkgs, &unresolved, true)

	// If the helper has calls that look like they could be further helpers,
	// check if any events are empty and add unresolved messages.
	if len(events) == 0 {
		unresolved = append(unresolved,
			fmt.Sprintf("response: helper function %s() could not be traced — body type unknown", fnObj.Name()),
		)
	}

	// Post-process: resolve status codes that are -1 (unresolvable) by mapping
	// helper parameters back to caller arguments. For example, if the helper has
	// w.WriteHeader(code) where code is the 3rd parameter, resolve the caller's
	// 3rd argument (http.StatusBadRequest → 400).
	resolveHelperStatusCodes(events, fd, call, info)

	// Tag events as coming from a helper and fix their positions.
	// Helper events have positions within the helper function body — these
	// need to be remapped to the call site position so the branch-aware
	// pairing algorithm groups them correctly with the calling function's
	// return boundaries.
	callPos := call.Pos()
	for i := range events {
		if events[i].kind != "combined" {
			events[i].kind = "helper"
		}
		events[i].pos = callPos
	}

	return events, unresolved
}

// traceGinHelper checks if a call is to a helper function that takes *gin.Context
// as its first argument. If so, enters the helper body (one level only) and
// collects gin response events from it.
func traceGinHelper(call *ast.CallExpr, info *types.Info, pn resolver.HandlerParamNames, pkgs []*packages.Package) ([]model.ResponseDef, []string) {
	if len(call.Args) == 0 || pn.GinCtx == "" {
		return nil, nil
	}

	// Check if the first argument is the gin context.
	firstArgIdent, ok := call.Args[0].(*ast.Ident)
	if !ok || firstArgIdent.Name != pn.GinCtx {
		return nil, nil
	}

	// Resolve the called function.
	var obj types.Object
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		obj = info.Uses[fn]
	case *ast.SelectorExpr:
		obj = info.Uses[fn.Sel]
	default:
		return nil, nil
	}

	if obj == nil {
		return nil, nil
	}

	fnObj, ok := obj.(*types.Func)
	if !ok {
		return nil, nil
	}

	if fnObj.Pkg() == nil || skipPackages[fnObj.Pkg().Path()] {
		return nil, nil
	}

	fd := findHelperFuncDecl(fnObj, pkgs)
	if fd == nil || fd.Body == nil {
		return nil, []string{
			fmt.Sprintf("response: helper function %s() could not be traced — body type unknown", fnObj.Name()),
		}
	}

	// Resolve the helper's own parameter names.
	helperPN := resolver.ResolveHandlerParams(fd.Type, info)

	// Recursively extract gin responses from helper body (pass nil pkgs to prevent further recursion).
	responses, unresolved := extractGinResponsesFromHelper(fd.Body, info, helperPN, call, fd, pn)

	// Mark all responses as coming from a helper.
	for i := range responses {
		responses[i].Source = "helper"
	}

	return responses, unresolved
}

// extractGinResponsesFromHelper extracts gin responses from a helper function body.
// It's similar to extractGinResponses but uses the helper's parameter names and
// resolves dynamic status codes from the caller's arguments.
func extractGinResponsesFromHelper(body *ast.BlockStmt, info *types.Info, helperPN resolver.HandlerParamNames, callerCall *ast.CallExpr, helperDecl *ast.FuncDecl, callerPN resolver.HandlerParamNames) ([]model.ResponseDef, []string) {
	var responses []model.ResponseDef
	var unresolved []string
	seen := make(map[int]bool)

	// Build param-to-caller-arg mapping for status code resolution.
	paramToCallerIdx := make(map[string]int)
	idx := 0
	if helperDecl.Type.Params != nil {
		for _, field := range helperDecl.Type.Params.List {
			for _, name := range field.Names {
				paramToCallerIdx[name.Name] = idx
				idx++
			}
		}
	}

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		recv, ok := sel.X.(*ast.Ident)
		if !ok || recv.Name != helperPN.GinCtx {
			return true
		}

		switch sel.Sel.Name {
		case "JSON", "AbortWithStatusJSON":
			if len(call.Args) >= 2 {
				code := ResolveStatusCode(call.Args[0], info)
				// If unresolvable, try resolving from caller's arguments.
				if code == -1 {
					code = resolveFromCallerArgs(call.Args[0], paramToCallerIdx, callerCall, info)
				}
				if code == -1 {
					unresolved = append(unresolved, unresolvedStatusMsg(call, info))
					return true
				}
				if seen[code] {
					return true
				}
				seen[code] = true
				resp := model.ResponseDef{
					StatusCode:  code,
					ContentType: "application/json",
					Description: descriptionForStatus(code),
				}
				bodyType := resolveBodyType(call.Args[1], info)
				if bodyType != nil {
					resp.Body = typeRefDef(bodyType)
				}
				responses = append(responses, resp)
			}

		case "Status":
			if len(call.Args) >= 1 {
				code := ResolveStatusCode(call.Args[0], info)
				if code == -1 {
					code = resolveFromCallerArgs(call.Args[0], paramToCallerIdx, callerCall, info)
				}
				if code == -1 {
					unresolved = append(unresolved, unresolvedStatusMsg(call, info))
					return true
				}
				if !seen[code] {
					seen[code] = true
					responses = append(responses, model.ResponseDef{
						StatusCode:  code,
						Description: descriptionForStatus(code),
					})
				}
			}
		}

		return true
	})

	return responses, unresolved
}

// resolveFromCallerArgs resolves a status code by mapping a helper's parameter
// variable back to the caller's argument.
func resolveFromCallerArgs(expr ast.Expr, paramToCallerIdx map[string]int, callerCall *ast.CallExpr, info *types.Info) int {
	argIdent, ok := expr.(*ast.Ident)
	if !ok {
		return -1
	}
	callerIdx, found := paramToCallerIdx[argIdent.Name]
	if !found || callerIdx >= len(callerCall.Args) {
		return -1
	}
	return ResolveStatusCode(callerCall.Args[callerIdx], info)
}

// resolveHelperStatusCodes tries to fix unresolvable status codes (-1) in
// helper events by mapping the helper's parameter positions back to the
// caller's argument expressions. For example, sendError(w, msg, http.StatusBadRequest)
// calls a helper with code at position 2 — we resolve the caller's arg[2].
func resolveHelperStatusCodes(events []responseEvent, helperDecl *ast.FuncDecl, callerCall *ast.CallExpr, info *types.Info) {
	// Build a mapping of helper parameter names to caller argument indices.
	// Helper params are indexed sequentially across all fields.
	paramToCallerIdx := make(map[string]int)
	idx := 0
	if helperDecl.Type.Params != nil {
		for _, field := range helperDecl.Type.Params.List {
			for _, name := range field.Names {
				paramToCallerIdx[name.Name] = idx
				idx++
			}
		}
	}

	for i := range events {
		if events[i].statusCode != -1 && events[i].statusCode != 0 {
			continue
		}

		// We need to find which parameter was used for the status code.
		// Walk the helper body to find WriteHeader calls with parameter variables.
		ast.Inspect(helperDecl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name != "WriteHeader" || len(call.Args) != 1 {
				return true
			}
			// Check if the arg is a parameter identifier.
			argIdent, ok := call.Args[0].(*ast.Ident)
			if !ok {
				return true
			}
			callerIdx, found := paramToCallerIdx[argIdent.Name]
			if !found || callerIdx >= len(callerCall.Args) {
				return true
			}
			// Resolve the status code from the caller's argument.
			code := ResolveStatusCode(callerCall.Args[callerIdx], info)
			if code != -1 {
				events[i].statusCode = code
			}
			return false
		})
	}
}

// findHelperFuncDecl searches all packages for the FuncDecl of a types.Func.
func findHelperFuncDecl(fn *types.Func, pkgs []*packages.Package) *ast.FuncDecl {
	fnPkg := fn.Pkg()
	if fnPkg == nil {
		return nil
	}

	var targetPkg *packages.Package
	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		if pkg.Types == fnPkg {
			targetPkg = pkg
			return false
		}
		return true
	}, nil)

	if targetPkg == nil {
		return nil
	}

	fnPos := fn.Pos()
	for _, file := range targetPkg.Syntax {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fd.Name.Pos() == fnPos {
				return fd
			}
		}
	}

	return nil
}
