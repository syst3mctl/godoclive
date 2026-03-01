package resolver

import (
	"go/ast"
	"go/types"
)

// HandlerParamNames holds the local variable names for the HTTP primitives
// inside a specific handler function body.
type HandlerParamNames struct {
	Writer  string // name of the http.ResponseWriter param
	Request string // name of the *http.Request param
	GinCtx  string // name of the *gin.Context param
}

// ResolveHandlerParams inspects a function's parameter list and determines
// which parameter names correspond to http.ResponseWriter, *http.Request,
// and *gin.Context using the type checker — never hardcoded names.
func ResolveHandlerParams(fn *ast.FuncType, info *types.Info) HandlerParamNames {
	names := HandlerParamNames{}
	if fn == nil || fn.Params == nil || info == nil {
		return names
	}

	for _, field := range fn.Params.List {
		t := info.TypeOf(field.Type)
		if t == nil {
			continue
		}

		// Handle unnamed parameters (no names in field.Names).
		if len(field.Names) == 0 {
			continue
		}

		for _, name := range field.Names {
			if name.Name == "_" {
				continue
			}
			switch {
			case implementsResponseWriter(t):
				names.Writer = name.Name
			case isHTTPRequest(t):
				names.Request = name.Name
			case isGinContext(t):
				names.GinCtx = name.Name
			}
		}
	}

	return names
}

// implementsResponseWriter checks whether t implements net/http.ResponseWriter.
// http.ResponseWriter is an interface with Header(), Write(), and WriteHeader().
func implementsResponseWriter(t types.Type) bool {
	// http.ResponseWriter is an interface. Check if the type's underlying
	// interface matches by checking the named type path.
	if named, ok := t.(*types.Named); ok {
		obj := named.Obj()
		if obj.Pkg() != nil && obj.Pkg().Path() == "net/http" && obj.Name() == "ResponseWriter" {
			return true
		}
	}

	// Also check the interface itself — the parameter type might be the
	// interface type directly.
	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return false
	}

	// Check that the interface has the three methods of http.ResponseWriter.
	needed := map[string]bool{
		"Header":      false,
		"Write":       false,
		"WriteHeader": false,
	}
	for i := 0; i < iface.NumMethods(); i++ {
		m := iface.Method(i)
		if _, ok := needed[m.Name()]; ok {
			needed[m.Name()] = true
		}
	}
	for _, found := range needed {
		if !found {
			return false
		}
	}
	return true
}

// isHTTPRequest checks whether t is *net/http.Request.
func isHTTPRequest(t types.Type) bool {
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := ptr.Elem().(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "net/http" && obj.Name() == "Request"
}

// isGinContext checks whether t is *gin.Context.
func isGinContext(t types.Type) bool {
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := ptr.Elem().(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "github.com/gin-gonic/gin" && obj.Name() == "Context"
}
