package mapper

import (
	"go/constant"
	"go/types"
	"math"
	"sort"
	"strconv"

	"github.com/syst3mctl/godoclive/internal/model"
)

// namedConstValues returns the values of the constants declared with a named
// type — the Go spelling of an enumeration:
//
//	type Status string
//
//	const (
//	    StatusDraft     Status = "draft"
//	    StatusPublished Status = "published"
//	)
//
// A field of that type may only hold one of those values, and a schema that
// says so tells a client what to send. Nothing here needs the declaring
// package's source: constants and their values come from go/types, which is
// populated from export data, so the enum resolves for a type defined in a
// dependency as readily as one next door.
//
// Values are returned in declaration order. Sorting them would put "archived"
// before "draft", losing an order the author chose and, for an integer enum,
// scrambling the sequence entirely.
func namedConstValues(named *types.Named) []string {
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return nil
	}
	// Only strings and integers enumerate. A named float or bool is not an
	// enumeration in any idiom worth guessing at.
	basic, ok := named.Underlying().(*types.Basic)
	if !ok || basic.Info()&(types.IsString|types.IsInteger) == 0 {
		return nil
	}

	scope := obj.Pkg().Scope()
	type entry struct {
		pos   int
		value string
	}
	var found []entry
	for _, name := range scope.Names() {
		c, ok := scope.Lookup(name).(*types.Const)
		if !ok || !types.Identical(c.Type(), named) {
			continue
		}
		var value string
		switch v := c.Val(); v.Kind() {
		case constant.String:
			value = constant.StringVal(v)
		case constant.Int, constant.Float:
			value = v.String()
		default:
			continue // an unknown or unrepresentable value documents nothing
		}
		found = append(found, entry{pos: int(c.Pos()), value: value})
	}
	if len(found) == 0 {
		return nil
	}

	// scope.Names() is sorted alphabetically; recover declaration order from
	// the source positions, which run in file order within a package.
	sort.Slice(found, func(i, j int) bool { return found[i].pos < found[j].pos })

	values := make([]string, 0, len(found))
	for _, e := range found {
		values = append(values, e.value)
	}
	return values
}

// enumExample renders an enum member as an example value of the field's own
// type. Members are carried as strings; a numeric field needs a number.
func enumExample(value string, td model.TypeDef) interface{} {
	if isNumericType(td) {
		if n, err := strconv.ParseFloat(value, 64); err == nil {
			if n == math.Trunc(n) {
				return int64(n)
			}
			return n
		}
	}
	return value
}
