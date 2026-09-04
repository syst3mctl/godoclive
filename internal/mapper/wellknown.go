package mapper

import (
	"go/types"

	"github.com/syst3mctl/godoclive/internal/model"
)

// wellKnownTypes are the types whose JSON form is not their struct shape.
//
// time.Time is the one that matters most, and getting it wrong is loud: it
// marshals to an RFC 3339 string, but expanding its fields documents wall, ext
// and loc — Go runtime internals — and drags Location, zone and zoneTrans into
// the spec alongside them. Nearly every API has a timestamp, so nearly every
// spec carried the mess.
//
// Keyed by package path and type name, because the name alone is ambiguous:
// plenty of projects declare their own Time or Decimal.
var wellKnownTypes = map[string]model.TypeDef{
	"time.Time":                             {Kind: model.KindPrimitive, Name: "string", Format: "date-time"},
	"github.com/google/uuid.UUID":           {Kind: model.KindPrimitive, Name: "string", Format: "uuid"},
	"github.com/gofrs/uuid.UUID":            {Kind: model.KindPrimitive, Name: "string", Format: "uuid"},
	"github.com/gofrs/uuid/v5.UUID":         {Kind: model.KindPrimitive, Name: "string", Format: "uuid"},
	"github.com/shopspring/decimal.Decimal": {Kind: model.KindPrimitive, Name: "string", Format: "decimal"},
	"math/big.Int":                          {Kind: model.KindPrimitive, Name: "string"},
	"math/big.Float":                        {Kind: model.KindPrimitive, Name: "string"},
	"net.IP":                                {Kind: model.KindPrimitive, Name: "string"},
	// json.RawMessage is whatever JSON was put in it.
	"encoding/json.RawMessage": {Kind: model.KindInterface, Name: "json.RawMessage"},
}

// wellKnownType returns the documented form of a type whose JSON shape is not
// its Go shape.
func wellKnownType(t types.Type) (model.TypeDef, bool) {
	named, ok := t.(*types.Named)
	if !ok {
		return model.TypeDef{}, false
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return model.TypeDef{}, false
	}
	def, ok := wellKnownTypes[obj.Pkg().Path()+"."+obj.Name()]
	return def, ok
}

// marshalsItsOwnJSON reports whether a type defines MarshalJSON, on either a
// value or a pointer receiver.
//
// Such a type does not marshal as its fields, and nothing in the declaration
// says what it does marshal as — MarshalJSON's body could write anything. The
// fields are the one answer known to be wrong, so they are not published;
// the type is documented as unconstrained JSON instead.
//
// The check is structural, like the ResponseWriter test elsewhere in the
// analyzer, so it needs no encoding/json in the loaded package set.
func marshalsItsOwnJSON(t types.Type) bool {
	if t == nil {
		return false
	}
	if hasMarshalJSON(t) {
		return true
	}
	if _, isPtr := t.(*types.Pointer); !isPtr {
		return hasMarshalJSON(types.NewPointer(t))
	}
	return false
}

// hasMarshalJSON reports whether t's method set contains
// MarshalJSON() ([]byte, error).
func hasMarshalJSON(t types.Type) bool {
	ms := types.NewMethodSet(t)
	for i := 0; i < ms.Len(); i++ {
		fn, ok := ms.At(i).Obj().(*types.Func)
		if !ok || fn.Name() != "MarshalJSON" {
			continue
		}
		sig, ok := fn.Type().(*types.Signature)
		if !ok || sig.Params().Len() != 0 || sig.Results().Len() != 2 {
			continue
		}
		slice, ok := sig.Results().At(0).Type().(*types.Slice)
		if !ok {
			continue
		}
		basic, ok := slice.Elem().(*types.Basic)
		if !ok || basic.Kind() != types.Byte {
			continue
		}
		if named, ok := sig.Results().At(1).Type().(*types.Named); ok && named.Obj().Name() == "error" {
			return true
		}
	}
	return false
}
