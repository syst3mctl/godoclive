package generator

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/syst3mctl/godoclive/internal/openapi"
	"github.com/syst3mctl/godoclive/internal/pipeline"
)

func e2eTestdataDir(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
}

// TestResponseSchema_EndToEnd verifies that typed net/http response bodies land
// in BOTH the OpenAPI document (components/schemas + responses.200.schema.$ref)
// and the HTML window.API_DATA payload (endpoints[].responses[].body.fields),
// including embedded-struct inlining — for method-receiver handlers registered
// via http.HandlerFunc, a doc-only `if false` encode, and a JSON helper.
func TestResponseSchema_EndToEnd(t *testing.T) {
	dir := e2eTestdataDir("stdlib-response")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// --- OpenAPI side ---
	doc := openapi.Generate(eps, openapi.Config{Title: "stdlib-response", Version: "1.0.0"})

	if doc.Components == nil || doc.Components.Schemas == nil {
		t.Fatal("openapi: expected components/schemas to be populated")
	}
	for _, name := range []string{"ItemList", "ItemResponse", "ItemDetail"} {
		if _, ok := doc.Components.Schemas[name]; !ok {
			t.Errorf("openapi: components/schemas missing %q", name)
		}
	}

	// GET /v1/items → 200 → $ref ItemList (doc-only encode, method receiver).
	getItems := doc.Paths["/v1/items"]
	if getItems == nil || getItems.Get == nil {
		t.Fatal("openapi: missing GET /v1/items operation")
	}
	if ref := schemaRef(getItems.Get.Responses["200"]); ref != "#/components/schemas/ItemList" {
		t.Errorf("openapi: GET /v1/items 200 schema $ref = %q, want ItemList", ref)
	}

	// ItemList must carry its fields (items is an array of $ref ItemResponse).
	itemList := doc.Components.Schemas["ItemList"]
	if itemList.Properties["items"] == nil || itemList.Properties["items"].Type != "array" {
		t.Errorf("openapi: ItemList.items should be an array, got %+v", itemList.Properties["items"])
	}
	if itemList.Properties["next_cursor"] == nil {
		t.Error("openapi: ItemList.next_cursor missing")
	}

	// POST /v1/items → 201 → $ref ItemDetail (helper-bound concrete type).
	if getItems.Post == nil {
		t.Fatal("openapi: missing POST /v1/items operation")
	}
	if ref := schemaRef(getItems.Post.Responses["201"]); ref != "#/components/schemas/ItemDetail" {
		t.Errorf("openapi: POST /v1/items 201 schema $ref = %q, want ItemDetail", ref)
	}

	// Embedded Meta must be inlined into ItemDetail (Go JSON semantics).
	itemDetail := doc.Components.Schemas["ItemDetail"]
	if itemDetail.Properties["request_id"] == nil {
		t.Error("openapi: ItemDetail must inline embedded Meta.request_id")
	}
	if itemDetail.Properties["item"] == nil || itemDetail.Properties["extra"] == nil {
		t.Error("openapi: ItemDetail missing item/extra fields")
	}

	// --- HTML window.API_DATA side ---
	apiJSON, err := json.Marshal(buildAPIData(eps, GeneratorConfig{}))
	if err != nil {
		t.Fatalf("marshal API_DATA: %v", err)
	}
	var ad struct {
		Endpoints []struct {
			Method    string `json:"method"`
			Path      string `json:"path"`
			Responses []struct {
				Status int `json:"status"`
				Body   *struct {
					TypeName string `json:"typeName"`
					Fields   []struct {
						JSONName string `json:"jsonName"`
					} `json:"fields"`
				} `json:"body"`
			} `json:"responses"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal(apiJSON, &ad); err != nil {
		t.Fatalf("unmarshal API_DATA: %v", err)
	}

	// responseFields returns the JSON field names of a given endpoint/status
	// response body from the API_DATA payload.
	responseFields := func(method, path string, status int) []string {
		for _, ep := range ad.Endpoints {
			if ep.Method != method || ep.Path != path {
				continue
			}
			for _, r := range ep.Responses {
				if r.Status != status || r.Body == nil {
					continue
				}
				var names []string
				for _, f := range r.Body.Fields {
					names = append(names, f.JSONName)
				}
				return names
			}
		}
		return nil
	}

	listFields := responseFields("GET", "/v1/items", 200)
	if !hasField(listFields, "items") || !hasField(listFields, "next_cursor") {
		t.Errorf("API_DATA: GET /v1/items 200 body.fields = %v, want items+next_cursor", listFields)
	}

	detailFields := responseFields("POST", "/v1/items", 201)
	// request_id proves the embedded Meta is inlined on the response side too.
	for _, want := range []string{"request_id", "item", "extra"} {
		if !hasField(detailFields, want) {
			t.Errorf("API_DATA: POST /v1/items 201 body.fields missing %q (got %v)", want, detailFields)
		}
	}
}

// TestResponseSchema_ProxyFixture_EndToEnd asserts every EXPECT note in the
// reverse-proxy fixture (testdata/stdlib-proxy): typed response $refs for
// method-receiver handlers registered via http.HandlerFunc, embedded-struct
// inlining, a relay helper that must NOT downgrade the typed body's content
// type, a one-arg JSON helper binding the concrete arg type, and a status-only
// 204 with no body — across both OpenAPI and window.API_DATA.
func TestResponseSchema_ProxyFixture_EndToEnd(t *testing.T) {
	dir := e2eTestdataDir("stdlib-proxy")
	eps, err := pipeline.RunPipeline(dir, "./...", nil)
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}

	// --- OpenAPI side ---
	doc := openapi.Generate(eps, openapi.Config{Title: "stdlib-proxy", Version: "1.0.0"})

	if doc.Components == nil || doc.Components.Schemas == nil {
		t.Fatal("openapi: expected components/schemas")
	}
	for _, name := range []string{"Item", "ItemList", "ItemDetail", "CreateItemRequest"} {
		if _, ok := doc.Components.Schemas[name]; !ok {
			t.Errorf("openapi: components/schemas missing %q", name)
		}
	}

	// Every GET response must carry a $ref schema, not a bare 200.
	wantRef := map[[3]string]string{
		{"/v1/items", "get", "200"}:        "#/components/schemas/ItemList",
		{"/v1/items/{id}", "get", "200"}:   "#/components/schemas/ItemDetail",
		{"/v1/items", "post", "201"}:       "#/components/schemas/Item",
		{"/v1/items/helper", "get", "200"}: "#/components/schemas/Item",
	}
	for key, want := range wantRef {
		op := operation(doc, key[0], key[1])
		if op == nil {
			t.Errorf("openapi: missing %s %s", key[1], key[0])
			continue
		}
		if got := schemaRef(op.Responses[key[2]]); got != want {
			t.Errorf("openapi: %s %s %s schema $ref = %q, want %q", key[1], key[0], key[2], got, want)
		}
	}

	// POST request body → CreateItemRequest.
	post := operation(doc, "/v1/items", "post")
	if post.RequestBody == nil || post.RequestBody.Content["application/json"] == nil ||
		post.RequestBody.Content["application/json"].Schema.Ref != "#/components/schemas/CreateItemRequest" {
		t.Errorf("openapi: POST /v1/items request body should $ref CreateItemRequest, got %+v", post.RequestBody)
	}

	// DELETE → 204 No Content, no body.
	del := operation(doc, "/v1/items/{id}", "delete")
	if del == nil || del.Responses["204"] == nil {
		t.Fatal("openapi: missing DELETE 204")
	}
	if del.Responses["204"].Content != nil {
		t.Errorf("openapi: DELETE 204 should have no content, got %+v", del.Responses["204"].Content)
	}

	// Embedded Item inlined into ItemDetail (id + name + tags).
	detail := doc.Components.Schemas["ItemDetail"]
	for _, p := range []string{"id", "name", "tags"} {
		if detail.Properties[p] == nil {
			t.Errorf("openapi: ItemDetail missing inlined property %q", p)
		}
	}
	if detail.Properties["tags"] != nil && detail.Properties["tags"].Type != "array" {
		t.Errorf("openapi: ItemDetail.tags should be array, got %+v", detail.Properties["tags"])
	}
	// ItemList.items → array of $ref Item.
	items := doc.Components.Schemas["ItemList"].Properties["items"]
	if items == nil || items.Type != "array" || items.Items == nil || items.Items.Ref != "#/components/schemas/Item" {
		t.Errorf("openapi: ItemList.items should be array of $ref Item, got %+v", items)
	}

	// --- window.API_DATA side ---
	apiJSON, err := json.Marshal(buildAPIData(eps, GeneratorConfig{}))
	if err != nil {
		t.Fatalf("marshal API_DATA: %v", err)
	}
	var ad struct {
		Endpoints []struct {
			Method    string `json:"method"`
			Path      string `json:"path"`
			Responses []struct {
				Status      int    `json:"status"`
				ContentType string `json:"contentType"`
				Body        *struct {
					TypeName string `json:"typeName"`
					Fields   []struct {
						JSONName string `json:"jsonName"`
					} `json:"fields"`
				} `json:"body"`
			} `json:"responses"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal(apiJSON, &ad); err != nil {
		t.Fatalf("unmarshal API_DATA: %v", err)
	}

	type respView struct {
		ct       string
		typeName string
		fields   []string
		bodyNil  bool
	}
	find := func(method, path string, status int) (respView, bool) {
		for _, ep := range ad.Endpoints {
			if ep.Method != method || ep.Path != path {
				continue
			}
			for _, r := range ep.Responses {
				if r.Status != status {
					continue
				}
				if r.Body == nil {
					return respView{ct: r.ContentType, bodyNil: true}, true
				}
				var names []string
				for _, f := range r.Body.Fields {
					names = append(names, f.JSONName)
				}
				return respView{ct: r.ContentType, typeName: r.Body.TypeName, fields: names}, true
			}
		}
		return respView{}, false
	}

	cases := []struct {
		method, path string
		status       int
		wantType     string
		wantFields   []string
		wantBodyNil  bool
		wantContent  string
	}{
		{"GET", "/v1/items", 200, "ItemList", []string{"items", "next_cursor"}, false, "application/json"},
		{"GET", "/v1/items/{id}", 200, "ItemDetail", []string{"id", "name", "tags"}, false, "application/json"},
		{"POST", "/v1/items", 201, "Item", []string{"id", "name"}, false, "application/json"},
		{"GET", "/v1/items/helper", 200, "Item", []string{"id", "name"}, false, "application/json"},
		{"DELETE", "/v1/items/{id}", 204, "", nil, true, ""},
	}
	for _, c := range cases {
		rv, ok := find(c.method, c.path, c.status)
		if !ok {
			t.Errorf("API_DATA: missing %s %s %d", c.method, c.path, c.status)
			continue
		}
		if c.wantBodyNil {
			if !rv.bodyNil {
				t.Errorf("API_DATA: %s %s %d body should be null", c.method, c.path, c.status)
			}
			continue
		}
		if rv.typeName != c.wantType {
			t.Errorf("API_DATA: %s %s %d typeName = %q, want %q", c.method, c.path, c.status, rv.typeName, c.wantType)
		}
		if rv.ct != c.wantContent {
			t.Errorf("API_DATA: %s %s %d contentType = %q, want %q", c.method, c.path, c.status, rv.ct, c.wantContent)
		}
		for _, f := range c.wantFields {
			if !hasField(rv.fields, f) {
				t.Errorf("API_DATA: %s %s %d body.fields missing %q (got %v)", c.method, c.path, c.status, f, rv.fields)
			}
		}
	}
}

// operation returns the *Operation for a path+lowercase-method, or nil.
func operation(doc *openapi.Document, path, method string) *openapi.Operation {
	pi := doc.Paths[path]
	if pi == nil {
		return nil
	}
	switch method {
	case "get":
		return pi.Get
	case "post":
		return pi.Post
	case "put":
		return pi.Put
	case "delete":
		return pi.Delete
	case "patch":
		return pi.Patch
	default:
		return nil
	}
}

func schemaRef(r *openapi.Response) string {
	if r == nil || r.Content == nil {
		return ""
	}
	mt := r.Content["application/json"]
	if mt == nil || mt.Schema == nil {
		return ""
	}
	return mt.Schema.Ref
}

func hasField(fields []string, name string) bool {
	return slices.Contains(fields, name)
}
