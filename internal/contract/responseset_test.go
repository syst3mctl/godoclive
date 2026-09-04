package contract

import (
	"testing"

	"github.com/syst3mctl/godoclive/internal/model"
)

func typeRef(name string) *model.TypeDef {
	return &model.TypeDef{Kind: model.KindStruct, Name: name, Package: "example.com/api"}
}

func TestResponseSet(t *testing.T) {
	t.Run("two payloads under one status both survive", func(t *testing.T) {
		s := newResponseSet()
		s.add(model.ResponseDef{StatusCode: 200, Body: typeRef("Article")})
		s.add(model.ResponseDef{StatusCode: 200, Body: typeRef("ArticleSummary")})

		got := s.all()
		if len(got) != 2 {
			t.Fatalf("got %d responses, want 2: %+v", len(got), got)
		}
		if got[0].Body.Name != "Article" || got[1].Body.Name != "ArticleSummary" {
			t.Errorf("order lost: %q then %q", got[0].Body.Name, got[1].Body.Name)
		}
	})

	t.Run("the same payload twice collapses", func(t *testing.T) {
		s := newResponseSet()
		s.add(model.ResponseDef{StatusCode: 400, Body: typeRef("Error")})
		s.add(model.ResponseDef{StatusCode: 400, Body: typeRef("Error")})

		if len(s.all()) != 1 {
			t.Errorf("got %d responses, want 1: %+v", len(s.all()), s.all())
		}
	})

	t.Run("a body-less response does not follow a documented payload", func(t *testing.T) {
		s := newResponseSet()
		s.add(model.ResponseDef{StatusCode: 400, Body: typeRef("Error")})
		s.add(model.ResponseDef{StatusCode: 400})

		got := s.all()
		if len(got) != 1 {
			t.Fatalf("got %d responses, want 1: %+v", len(got), got)
		}
		if got[0].Body == nil {
			t.Error("the documented payload was replaced by the body-less response")
		}
	})

	t.Run("a payload supersedes a body-less response recorded first", func(t *testing.T) {
		s := newResponseSet()
		s.add(model.ResponseDef{StatusCode: 404})
		s.add(model.ResponseDef{StatusCode: 404, Body: typeRef("Error")})

		got := s.all()
		if len(got) != 1 {
			t.Fatalf("got %d responses, want 1: %+v", len(got), got)
		}
		if got[0].Body == nil || got[0].Body.Name != "Error" {
			t.Errorf("got %+v, want the Error payload", got[0])
		}
	})

	t.Run("different statuses are independent", func(t *testing.T) {
		s := newResponseSet()
		s.add(model.ResponseDef{StatusCode: 200, Body: typeRef("Article")})
		s.add(model.ResponseDef{StatusCode: 404})
		s.add(model.ResponseDef{StatusCode: 500, Body: typeRef("Error")})

		if len(s.all()) != 3 {
			t.Errorf("got %d responses, want 3: %+v", len(s.all()), s.all())
		}
	})

	t.Run("has reports a recorded status", func(t *testing.T) {
		s := newResponseSet()
		s.add(model.ResponseDef{StatusCode: 204})
		if !s.has(204) {
			t.Error("has(204) = false after adding 204")
		}
		if s.has(200) {
			t.Error("has(200) = true without adding it")
		}
	})
}

// Two slices of different element types are different payloads; two of the same
// are one.
func TestBodyKeyDistinguishesShapes(t *testing.T) {
	users := &model.TypeDef{Kind: model.KindSlice, Elem: typeRef("User")}
	admins := &model.TypeDef{Kind: model.KindSlice, Elem: typeRef("Admin")}
	usersAgain := &model.TypeDef{Kind: model.KindSlice, Elem: typeRef("User")}

	if bodyKey(users) == bodyKey(admins) {
		t.Error("[]User and []Admin share a key")
	}
	if bodyKey(users) != bodyKey(usersAgain) {
		t.Error("two []User do not share a key")
	}
	if bodyKey(nil) != "" {
		t.Errorf("bodyKey(nil) = %q, want empty", bodyKey(nil))
	}
	if bodyKey(typeRef("User")) == bodyKey(users) {
		t.Error("User and []User share a key")
	}
}

// A recursive type must not send the key builder into an endless walk.
func TestBodyKeyTerminatesOnRecursiveTypes(t *testing.T) {
	node := &model.TypeDef{Kind: model.KindStruct, Name: "", Package: ""}
	node.Fields = []model.FieldDef{{JSONName: "next", Type: *node}}
	self := &model.TypeDef{Kind: model.KindSlice}
	self.Elem = self

	done := make(chan string, 2)
	go func() { done <- bodyKey(node) }()
	go func() { done <- bodyKey(self) }()
	for i := 0; i < 2; i++ {
		if k := <-done; k == "" {
			t.Error("recursive type produced an empty key")
		}
	}
}
