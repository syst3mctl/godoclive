package mapper

import (
	"go/ast"
	"go/types"
	"reflect"
	"strings"

	"github.com/syst3mctl/godoclive/internal/model"
	"golang.org/x/tools/go/packages"
)

// mapStruct walks all fields of a struct and builds a model.TypeDef with
// FieldDef entries. It handles JSON tags, binding/validate tags for required
// detection, embedded struct inlining, and field doc comments.
func mapStruct(named *types.Named, st *types.Struct, pkg *packages.Package, visited map[*types.Named]bool) model.TypeDef {
	def := model.TypeDef{
		Kind: model.KindStruct,
	}
	if named != nil {
		def.Name = named.Obj().Name()
		if named.Obj().Pkg() != nil {
			def.Package = named.Obj().Pkg().Path()
		}
	}

	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		tagStr := st.Tag(i)
		tag := reflect.StructTag(tagStr)

		jsonTag := tag.Get("json")
		jsonName, opts := parseJSONTag(jsonTag)

		if jsonName == "-" {
			continue
		}
		if jsonName == "" {
			jsonName = field.Name()
		}

		fd := model.FieldDef{
			Name:      field.Name(),
			JSONName:  jsonName,
			OmitEmpty: strings.Contains(opts, "omitempty"),
			Nullable:  isPointer(field.Type()),
			Required:  isRequired(tag),
			Doc:       fieldDocComment(named, field, pkg),
		}
		fd.Type = mapType(field.Type(), pkg, visited)
		fd.Example = generateExample(field.Type(), jsonName)

		// Embedded struct: inline fields (matches Go's JSON encoding behavior).
		if field.Embedded() && fd.Type.Kind == model.KindStruct {
			def.Fields = append(def.Fields, fd.Type.Fields...)
			continue
		}

		def.Fields = append(def.Fields, fd)
	}
	return def
}

// parseJSONTag splits a json tag value into the name and remaining options.
// e.g. "email,omitempty" → ("email", "omitempty")
func parseJSONTag(tag string) (string, string) {
	if tag == "" {
		return "", ""
	}
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], tag[idx+1:]
	}
	return tag, ""
}

// isPointer returns true if the given type is a pointer type.
func isPointer(t types.Type) bool {
	_, ok := t.(*types.Pointer)
	return ok
}

// isRequired checks binding and validate struct tags for "required".
func isRequired(tag reflect.StructTag) bool {
	binding := tag.Get("binding")
	validate := tag.Get("validate")
	return strings.Contains(binding, "required") || strings.Contains(validate, "required")
}

// fieldDocComment attempts to extract the doc comment for a struct field
// by finding the field's AST node in the package syntax.
func fieldDocComment(named *types.Named, field *types.Var, pkg *packages.Package) string {
	if named == nil || pkg == nil {
		return ""
	}

	fieldPos := field.Pos()
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, f := range st.Fields.List {
					for _, name := range f.Names {
						if name.Pos() == fieldPos && f.Doc != nil {
							return strings.TrimSpace(f.Doc.Text())
						}
					}
				}
			}
		}
	}
	return ""
}
