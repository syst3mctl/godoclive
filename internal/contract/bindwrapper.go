package contract

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"

	"github.com/syst3mctl/godoclive/internal/resolver"
)

// maxBindWrapperDepth bounds how many call levels the request-binding chain is
// followed. The common RealWorld-style idiom needs two —
//
//	validator.Bind(c) → common.Bind(c, validator) → c.ShouldBindWith(obj, b)
//
// — and a third leaves room for one more layer of indirection without letting
// an arbitrarily deep (or mutually recursive) chain run away.
const maxBindWrapperDepth = 3

// ginBodyBindMethods are the gin context methods that bind the REQUEST BODY
// into a destination value. Query- and header-only binders are deliberately
// absent: they are promoted to parameters, not to a body schema.
var ginBodyBindMethods = map[string]bool{
	"ShouldBindJSON":     true,
	"BindJSON":           true,
	"ShouldBindWith":     true,
	"MustBindWith":       true,
	"BindWith":           true,
	"ShouldBindBodyWith": true,
	"ShouldBindXML":      true,
	"BindXML":            true,
	"ShouldBindYAML":     true,
	"ShouldBind":         true,
	"Bind":               true,
}

// bindWrapperResult is what a traced binding chain yields.
type bindWrapperResult struct {
	Type        types.Type // resolved request body type, nil when unresolved
	ContentType string
	Detected    bool   // a binding wrapper was recognized
	Wrapper     string // the wrapper's name, for diagnostics
	Reason      string // why an detected wrapper stayed unresolved
}

// traceGinBindWrapper follows a handler-level call that hands the gin context
// to another function — `validator.Bind(c)`, `common.Bind(c, &req)`,
// `h.decode(c, &req)` — down to the gin bind call that actually reads the body,
// and reports the request schema in the CALLER's terms.
//
// The destination is almost never a concrete type at the point of the bind:
// the shared binder takes `obj interface{}`, and the validator passes itself.
// So rather than resolving a type at the innermost level, each level classifies
// the destination as "my parameter #i", "my receiver", or "this concrete type",
// and the classification is translated one level outward at a time until it
// lands on an expression the caller wrote.
func traceGinBindWrapper(call *ast.CallExpr, info *types.Info, pn resolver.HandlerParamNames, pkgs []*packages.Package) bindWrapperResult {
	if pn.GinCtx == "" || pkgs == nil {
		return bindWrapperResult{}
	}
	if !passesIdent(call, pn.GinCtx) {
		return bindWrapperResult{}
	}

	decl, declInfo, fnObj := resolveCallee(call, info, pkgs)
	if decl == nil || decl.Body == nil {
		return bindWrapperResult{}
	}

	dest, ok := findBindDest(decl, declInfo, pkgs, 0)
	if !ok {
		return bindWrapperResult{}
	}

	name := decl.Name.Name
	if fnObj != nil {
		name = fnObj.Name()
	}
	result := bindWrapperResult{
		Detected:    true,
		Wrapper:     name,
		ContentType: dest.contentType,
	}
	if dest.exhausted {
		result.Reason = "binding chain deeper than " + fmt.Sprint(maxBindWrapperDepth) + " levels"
		return result
	}

	t, ok := dest.resolveAgainst(call, info)
	if !ok || t == nil {
		result.Reason = "the value it binds into could not be traced back to a concrete type"
		return result
	}
	if isInterfaceType(t) {
		result.Reason = "it binds into an interface value, which documents nothing"
		return result
	}
	result.Type = t
	return result
}

// bindDest names where, in one function's own terms, the request body is bound.
type bindDest struct {
	paramIdx    int        // >= 0: the function's parameter at this argument position
	receiver    bool       // the function's receiver
	concrete    types.Type // a type resolved inside the function itself
	contentType string
	exhausted   bool // the chain was cut short by the depth bound
}

// resolveAgainst translates a destination expressed in the callee's terms into
// a concrete type, using the call site that invoked it.
func (d bindDest) resolveAgainst(call *ast.CallExpr, info *types.Info) (types.Type, bool) {
	switch {
	case d.concrete != nil:
		return d.concrete, true
	case d.receiver:
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil, false
		}
		return derefType(info.TypeOf(sel.X)), true
	case d.paramIdx >= 0 && d.paramIdx < len(call.Args):
		return extractArgType(call.Args[d.paramIdx], info)
	}
	return nil, false
}

// findBindDest locates the request-body bind inside fn and expresses its
// destination in fn's own terms, recursing through nested wrapper calls that
// pass the context along.
func findBindDest(fn *ast.FuncDecl, info *types.Info, pkgs []*packages.Package, depth int) (bindDest, bool) {
	if fn.Body == nil || info == nil {
		return bindDest{}, false
	}
	pn := resolver.ResolveHandlerParams(fn.Type, info)
	ctxName := pn.GinCtx
	if ctxName == "" {
		ctxName = ginCtxParamName(fn, info)
	}
	if ctxName == "" {
		return bindDest{}, false
	}

	var found bindDest
	ok := false

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if ok {
			return false
		}
		call, isCall := n.(*ast.CallExpr)
		if !isCall {
			return true
		}

		// A direct bind on this function's own context parameter.
		if sel, isSel := call.Fun.(*ast.SelectorExpr); isSel && ginBodyBindMethods[sel.Sel.Name] {
			if recv, isIdent := sel.X.(*ast.Ident); isIdent && recv.Name == ctxName && len(call.Args) >= 1 {
				found = classifyBindDest(call.Args[0], fn, info)
				found.contentType = bindContentType(sel.Sel.Name, call)
				ok = true
				return false
			}
		}

		// Otherwise: another wrapper this function delegates to.
		if !passesIdent(call, ctxName) {
			return true
		}
		if depth+1 >= maxBindWrapperDepth {
			// Something is clearly delegating, but we stop here rather than
			// following an unbounded chain.
			if nestedDecl, _, _ := resolveCallee(call, info, pkgs); nestedDecl != nil {
				found = bindDest{paramIdx: -1, exhausted: true}
				ok = true
				return false
			}
			return true
		}
		nestedDecl, nestedInfo, _ := resolveCallee(call, info, pkgs)
		if nestedDecl == nil {
			return true
		}
		inner, innerOK := findBindDest(nestedDecl, nestedInfo, pkgs, depth+1)
		if !innerOK {
			return true
		}
		found = translateDest(inner, call, fn, info)
		ok = true
		return false
	})

	return found, ok
}

// translateDest rewrites a destination expressed in a nested callee's terms
// into the calling function's terms, by classifying the expression that the
// calling function passed for it.
func translateDest(inner bindDest, call *ast.CallExpr, fn *ast.FuncDecl, info *types.Info) bindDest {
	out := bindDest{paramIdx: -1, contentType: inner.contentType, exhausted: inner.exhausted}
	if inner.exhausted {
		return out
	}
	switch {
	case inner.concrete != nil:
		out.concrete = inner.concrete
	case inner.receiver:
		if sel, isSel := call.Fun.(*ast.SelectorExpr); isSel {
			d := classifyBindDest(sel.X, fn, info)
			d.contentType = inner.contentType
			return d
		}
		out.exhausted = true
	case inner.paramIdx >= 0 && inner.paramIdx < len(call.Args):
		d := classifyBindDest(call.Args[inner.paramIdx], fn, info)
		d.contentType = inner.contentType
		return d
	default:
		out.exhausted = true
	}
	return out
}

// classifyBindDest decides whether an expression names one of fn's parameters,
// fn's receiver, or a value whose type is already concrete here.
func classifyBindDest(expr ast.Expr, fn *ast.FuncDecl, info *types.Info) bindDest {
	if unary, isUnary := expr.(*ast.UnaryExpr); isUnary {
		expr = unary.X
	}
	if ident, isIdent := expr.(*ast.Ident); isIdent {
		if idx, isParam := funcParamIndex(fn)[ident.Name]; isParam {
			return bindDest{paramIdx: idx}
		}
		if isReceiverName(fn, ident.Name) {
			return bindDest{paramIdx: -1, receiver: true}
		}
	}
	if t, resolved := extractArgType(expr, info); resolved && !isInterfaceType(t) {
		return bindDest{paramIdx: -1, concrete: t}
	}
	return bindDest{paramIdx: -1, exhausted: true}
}

// bindContentType maps a bind method (and its binding argument, when it has
// one) to the content type the endpoint accepts. gin's binding.Default picks
// between JSON and form encoding by the request's own Content-Type, so both are
// reported: the schema is fully known, only the encoding is the caller's choice.
func bindContentType(method string, call *ast.CallExpr) string {
	switch method {
	case "ShouldBindJSON", "BindJSON", "ShouldBindBodyWith":
		return "application/json"
	case "ShouldBindXML", "BindXML":
		return "application/xml"
	case "ShouldBindYAML":
		return "application/yaml"
	case "ShouldBindWith", "MustBindWith", "BindWith":
		if len(call.Args) >= 2 {
			if sel, isSel := call.Args[1].(*ast.SelectorExpr); isSel {
				switch sel.Sel.Name {
				case "JSON":
					return "application/json"
				case "XML":
					return "application/xml"
				case "YAML":
					return "application/yaml"
				case "Form", "FormPost":
					return "application/x-www-form-urlencoded"
				case "FormMultipart":
					return "multipart/form-data"
				}
			}
		}
	}
	return "application/json | application/x-www-form-urlencoded"
}

// --- small helpers ---

// passesIdent reports whether a call hands the named identifier to the callee.
func passesIdent(call *ast.CallExpr, name string) bool {
	for _, arg := range call.Args {
		if ident, ok := arg.(*ast.Ident); ok && ident.Name == name {
			return true
		}
	}
	return false
}

// resolveCallee resolves a call to the callee's declaration, the TypesInfo of
// the package declaring it, and its function object. Framework and stdlib
// callees are refused: their bodies are not part of the analyzed source.
func resolveCallee(call *ast.CallExpr, info *types.Info, pkgs []*packages.Package) (*ast.FuncDecl, *types.Info, *types.Func) {
	var obj types.Object
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		obj = info.Uses[fn]
	case *ast.SelectorExpr:
		obj = info.Uses[fn.Sel]
		if obj == nil {
			if sel, ok := info.Selections[fn]; ok {
				obj = sel.Obj()
			}
		}
	default:
		return nil, nil, nil
	}
	fnObj, ok := obj.(*types.Func)
	if !ok || fnObj.Pkg() == nil || skipPackages[fnObj.Pkg().Path()] {
		return nil, nil, nil
	}
	decl := findHelperFuncDecl(fnObj, pkgs)
	if decl == nil {
		return nil, nil, fnObj
	}
	declInfo := helperFuncInfo(fnObj, pkgs)
	if declInfo == nil {
		declInfo = info
	}
	return decl, declInfo, fnObj
}

// funcParamIndex maps each parameter name to its argument position.
func funcParamIndex(fn *ast.FuncDecl) map[string]int {
	idx := make(map[string]int)
	if fn.Type == nil || fn.Type.Params == nil {
		return idx
	}
	pos := 0
	for _, field := range fn.Type.Params.List {
		if len(field.Names) == 0 {
			pos++
			continue
		}
		for _, name := range field.Names {
			idx[name.Name] = pos
			pos++
		}
	}
	return idx
}

// isReceiverName reports whether name is fn's method receiver.
func isReceiverName(fn *ast.FuncDecl, name string) bool {
	if fn.Recv == nil {
		return false
	}
	for _, field := range fn.Recv.List {
		for _, ident := range field.Names {
			if ident.Name == name {
				return true
			}
		}
	}
	return false
}

// ginCtxParamName finds a *gin.Context parameter by type when the generic
// param resolver did not classify one.
func ginCtxParamName(fn *ast.FuncDecl, info *types.Info) string {
	if fn.Type == nil || fn.Type.Params == nil || info == nil {
		return ""
	}
	for _, field := range fn.Type.Params.List {
		t := info.TypeOf(field.Type)
		if t == nil {
			continue
		}
		if ptr, ok := derefOnce(t).(*types.Named); ok {
			if ptr.Obj().Pkg() != nil &&
				ptr.Obj().Pkg().Path() == "github.com/gin-gonic/gin" &&
				ptr.Obj().Name() == "Context" && len(field.Names) > 0 {
				return field.Names[0].Name
			}
		}
	}
	return ""
}

// derefOnce removes a single pointer indirection.
func derefOnce(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

// derefType removes a pointer indirection, returning nil unchanged.
func derefType(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	return derefOnce(t)
}
