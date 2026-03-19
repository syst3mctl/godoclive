package contract

import (
	"go/types"
	"reflect"
	"strings"

	"github.com/dicki/godoclive/internal/model"
)

// promoteStructFields resolves a struct type and converts its fields into
// ParamDef entries. tagKey is the struct tag to use for the param name
// (e.g. "form" for ShouldBindQuery, "header" for ShouldBindHeader).
// paramIn is "query" or "header".
func promoteStructFields(t types.Type, tagKey, paramIn string) []model.ParamDef {
	// Dereference pointer.
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return nil
	}

	var params []model.ParamDef
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		tagStr := st.Tag(i)
		tag := reflect.StructTag(tagStr)

		// Use the tag key (form/header) for the parameter name.
		paramName := tag.Get(tagKey)
		if paramName == "" || paramName == "-" {
			paramName = strings.ToLower(field.Name())
		}
		// Strip options after comma.
		if idx := strings.Index(paramName, ","); idx != -1 {
			paramName = paramName[:idx]
		}
		if paramName == "-" {
			continue
		}

		p := model.ParamDef{
			Name:     paramName,
			In:       paramIn,
			Type:     goTypeToParamType(field.Type()),
			Required: isBindingRequired(tag),
		}
		params = append(params, p)
	}
	return params
}

func goTypeToParamType(t types.Type) string {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	basic, ok := t.(*types.Basic)
	if !ok {
		return "string"
	}
	switch basic.Kind() {
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
		types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
		return "integer"
	case types.Float32, types.Float64:
		return "number"
	case types.Bool:
		return "boolean"
	default:
		return "string"
	}
}

func isBindingRequired(tag reflect.StructTag) bool {
	binding := tag.Get("binding")
	validate := tag.Get("validate")
	return strings.Contains(binding, "required") || strings.Contains(validate, "required")
}
