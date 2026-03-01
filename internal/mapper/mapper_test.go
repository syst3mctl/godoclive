package mapper_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/syst3mctl/godoclive/internal/loader"
	"github.com/syst3mctl/godoclive/internal/mapper"
	"github.com/syst3mctl/godoclive/internal/model"
	"golang.org/x/tools/go/packages"
)

func testdataDir(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", name)
}

func loadType(t *testing.T, dir, typeName string) (*packages.Package, *model.TypeDef) {
	t.Helper()
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages: %v", err)
	}
	pkg := pkgs[0]
	obj := pkg.Types.Scope().Lookup(typeName)
	if obj == nil {
		t.Fatalf("type %q not found", typeName)
	}
	td := mapper.MapType(obj.Type(), pkg)
	return pkg, &td
}

func findField(td *model.TypeDef, jsonName string) *model.FieldDef {
	for i := range td.Fields {
		if td.Fields[i].JSONName == jsonName {
			return &td.Fields[i]
		}
	}
	return nil
}

func TestMapType_StructFields(t *testing.T) {
	_, td := loadType(t, testdataDir("chi-basic"), "CreateUserRequest")

	if td.Kind != model.KindStruct {
		t.Fatalf("expected KindStruct, got %s", td.Kind)
	}
	if td.Name != "CreateUserRequest" {
		t.Errorf("expected name CreateUserRequest, got %s", td.Name)
	}
	if len(td.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(td.Fields))
	}
}

func TestMapType_JSONTags(t *testing.T) {
	_, td := loadType(t, testdataDir("chi-basic"), "CreateUserRequest")

	name := findField(td, "name")
	if name == nil {
		t.Fatal("field 'name' not found")
	}
	if name.OmitEmpty {
		t.Error("name should not be omitempty")
	}

	email := findField(td, "email")
	if email == nil {
		t.Fatal("field 'email' not found")
	}

	_, td2 := loadType(t, testdataDir("chi-basic"), "ErrorResponse")
	details := findField(td2, "details")
	if details == nil {
		t.Fatal("field 'details' not found")
	}
	if !details.OmitEmpty {
		t.Error("details should be omitempty")
	}
}

func TestMapType_Required(t *testing.T) {
	_, td := loadType(t, testdataDir("chi-basic"), "CreateUserRequest")

	name := findField(td, "name")
	if !name.Required {
		t.Error("name should be required (validate:'required')")
	}

	email := findField(td, "email")
	if !email.Required {
		t.Error("email should be required (validate:'required')")
	}

	age := findField(td, "age")
	if age.Required {
		t.Error("age should not be required")
	}
}

func TestMapType_PrimitiveFieldType(t *testing.T) {
	_, td := loadType(t, testdataDir("chi-basic"), "UserResponse")

	idField := findField(td, "id")
	if idField == nil {
		t.Fatal("field 'id' not found")
	}
	if idField.Type.Kind != model.KindPrimitive {
		t.Errorf("expected KindPrimitive, got %s", idField.Type.Kind)
	}
	if idField.Type.Name != "string" {
		t.Errorf("expected type 'string', got %s", idField.Type.Name)
	}
}

func TestExampleGeneration_NameHints(t *testing.T) {
	_, td := loadType(t, testdataDir("chi-basic"), "UserResponse")

	tests := []struct {
		jsonName string
		expected interface{}
	}{
		{"id", "f47ac10b-58cc-4372-a567-0e02b2c3d479"},
		{"email", "user@example.com"},
		{"name", "John Doe"},
	}

	for _, tt := range tests {
		f := findField(td, tt.jsonName)
		if f == nil {
			t.Errorf("field %q not found", tt.jsonName)
			continue
		}
		if f.Example != tt.expected {
			t.Errorf("field %q: expected example %v, got %v", tt.jsonName, tt.expected, f.Example)
		}
	}
}

func TestExampleGeneration_TypeFallback(t *testing.T) {
	_, td := loadType(t, testdataDir("chi-basic"), "ErrorResponse")

	// "code" is an int field without a name hint → fallback to 0
	codeField := findField(td, "code")
	if codeField == nil {
		t.Fatal("field 'code' not found")
	}
	if codeField.Example != 0 {
		t.Errorf("expected int fallback 0, got %v", codeField.Example)
	}
}

func TestExampleGeneration_SuffixMatch(t *testing.T) {
	_, td := loadType(t, testdataDir("chi-basic"), "UserResponse")

	// The "age" field is an int — should get type fallback 0.
	ageField := findField(td, "age")
	if ageField == nil {
		t.Fatal("field 'age' not found")
	}
	if ageField.Example != 0 {
		t.Errorf("expected int fallback 0, got %v (%T)", ageField.Example, ageField.Example)
	}
}
