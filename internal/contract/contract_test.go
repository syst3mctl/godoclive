package contract_test

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/syst3mctl/godoclive/internal/contract"
	"github.com/syst3mctl/godoclive/internal/extractor"
	"github.com/syst3mctl/godoclive/internal/loader"
	"github.com/syst3mctl/godoclive/internal/resolver"
)

func testdataDir(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
}

// resolveRoute is a test helper that resolves a route to its handler FuncDecl/FuncLit
// and returns the function body, type info, and param names.
type resolvedHandler struct {
	body       *ast.BlockStmt
	funcType   *ast.FuncType
	paramNames resolver.HandlerParamNames
}

func resolveRoutes(t *testing.T, dir string, ext extractor.Extractor) ([]extractor.RawRoute, map[string]resolvedHandler) {
	t.Helper()
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info := pkgs[0].TypesInfo
	handlers := make(map[string]resolvedHandler)

	for _, route := range routes {
		key := route.Method + " " + route.Path
		fd, fl, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Logf("ResolveHandler(%s) failed: %v", key, err)
			continue
		}

		var body *ast.BlockStmt
		var fnType *ast.FuncType
		if fd != nil {
			body = fd.Body
			fnType = fd.Type
		} else if fl != nil {
			body = fl.Body
			fnType = fl.Type
		}

		pn := resolver.ResolveHandlerParams(fnType, info)
		handlers[key] = resolvedHandler{body: body, funcType: fnType, paramNames: pn}
	}

	return routes, handlers
}

// --- Path Parameter Tests ---

func TestExtractPathParams_ChiBasic(t *testing.T) {
	dir := testdataDir("chi-basic")
	routes, handlers := resolveRoutes(t, dir, &extractor.ChiExtractor{})

	for _, route := range routes {
		key := route.Method + " " + route.Path
		h := handlers[key]
		params := contract.ExtractPathParams(route.Path, h.body, nil, h.paramNames)

		switch route.Path {
		case "/users":
			if len(params) != 0 {
				t.Errorf("%s: expected 0 path params, got %d", key, len(params))
			}
		case "/users/{id}":
			if len(params) != 1 {
				t.Errorf("%s: expected 1 path param, got %d", key, len(params))
				continue
			}
			p := params[0]
			if p.Name != "id" {
				t.Errorf("%s: expected name 'id', got %q", key, p.Name)
			}
			if !p.Required {
				t.Errorf("%s: path param should be required", key)
			}
			if p.Type != "uuid" {
				t.Errorf("%s: expected type 'uuid' (from name heuristic), got %q", key, p.Type)
			}
			if p.In != "path" {
				t.Errorf("%s: expected In='path', got %q", key, p.In)
			}
		}
	}
}

func TestExtractPathParams_TypeUpgrade(t *testing.T) {
	// chi-basic's GetUser uses chi.URLParam(r, "id") without strconv
	// so type stays as "uuid" from heuristic.
	// ListUsers uses strconv.Atoi on "page" query param — but that's a query param, not path.
	// We test the type inference directly with pattern-only extraction.
	params := contract.ExtractPathParams("/items/{itemId}/reviews/{page}", nil, nil, resolver.HandlerParamNames{})

	if len(params) != 2 {
		t.Fatalf("expected 2 path params, got %d", len(params))
	}

	// itemId should be "uuid" (ends with Id)
	if params[0].Name != "itemId" || params[0].Type != "uuid" {
		t.Errorf("param 0: expected itemId/uuid, got %s/%s", params[0].Name, params[0].Type)
	}

	// page should be "integer"
	if params[1].Name != "page" || params[1].Type != "integer" {
		t.Errorf("param 1: expected page/integer, got %s/%s", params[1].Name, params[1].Type)
	}
}

func TestExtractPathParams_Examples(t *testing.T) {
	params := contract.ExtractPathParams("/users/{id}/posts/{slug}", nil, nil, resolver.HandlerParamNames{})
	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(params))
	}

	if params[0].Example == "" {
		t.Error("id should have an example")
	}
	if params[1].Example != "my-resource" {
		t.Errorf("slug example: expected 'my-resource', got %q", params[1].Example)
	}
}

func TestExtractPathParams_GorillaBasic(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}
	info := pkgs[0].TypesInfo

	ext := &extractor.GorillaExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Find GET /users/{id} handler
	for _, route := range routes {
		if route.Method != "GET" || route.Path != "/users/{id}" {
			continue
		}

		fd, fl, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler failed: %v", err)
		}
		var body *ast.BlockStmt
		var fnType *ast.FuncType
		if fd != nil {
			body = fd.Body
			fnType = fd.Type
		} else if fl != nil {
			body = fl.Body
			fnType = fl.Type
		}
		pn := resolver.ResolveHandlerParams(fnType, info)

		params := contract.ExtractPathParams("/users/{id}", body, info, pn)
		if len(params) != 1 {
			t.Fatalf("expected 1 path param, got %d", len(params))
		}
		if params[0].Name != "id" {
			t.Errorf("expected param name 'id', got %q", params[0].Name)
		}
		return
	}
	t.Fatal("route GET /users/{id} not found")
}

func TestExtractPathParams_GorillaRegexParam(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}
	info := pkgs[0].TypesInfo

	ext := &extractor.GorillaExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Find GET /api/v1/items/{id} handler
	for _, route := range routes {
		if route.Method != "GET" || route.Path != "/api/v1/items/{id}" {
			continue
		}

		fd, fl, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler failed: %v", err)
		}
		var body *ast.BlockStmt
		var fnType *ast.FuncType
		if fd != nil {
			body = fd.Body
			fnType = fd.Type
		} else if fl != nil {
			body = fl.Body
			fnType = fl.Type
		}
		pn := resolver.ResolveHandlerParams(fnType, info)

		params := contract.ExtractPathParams("/api/v1/items/{id}", body, info, pn)
		if len(params) != 1 {
			t.Fatalf("expected 1 path param, got %d", len(params))
		}
		if params[0].Name != "id" {
			t.Errorf("expected param name 'id', got %q", params[0].Name)
		}
		// With types.Info, strconv.Atoi upgrade resolves mux.Vars indirection
		if params[0].Type != "integer" {
			t.Errorf("expected type 'integer' (strconv.Atoi upgrade), got %q", params[0].Type)
		}
		return
	}
	t.Fatal("route GET /api/v1/items/{id} not found")
}

func TestExtractPathParams_GorillaTypeUpgrade(t *testing.T) {
	dir := testdataDir("gorilla-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}
	info := pkgs[0].TypesInfo

	ext := &extractor.GorillaExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Find DELETE /users/{id} — uses mux.Vars(r)["id"]
	for _, route := range routes {
		if route.Method != "DELETE" || route.Path != "/users/{id}" {
			continue
		}

		fd, fl, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler failed: %v", err)
		}
		var body *ast.BlockStmt
		var fnType *ast.FuncType
		if fd != nil {
			body = fd.Body
			fnType = fd.Type
		} else if fl != nil {
			body = fl.Body
			fnType = fl.Type
		}
		pn := resolver.ResolveHandlerParams(fnType, info)

		params := contract.ExtractPathParams("/users/{id}", body, info, pn)
		if len(params) != 1 {
			t.Fatalf("expected 1 path param, got %d", len(params))
		}
		if params[0].Name != "id" {
			t.Errorf("expected param name 'id', got %q", params[0].Name)
		}
		return
	}
	t.Fatal("route DELETE /users/{id} not found")
}

// --- Query Parameter Tests ---

func TestExtractQueryParams_ChiBasic(t *testing.T) {
	dir := testdataDir("chi-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info := pkgs[0].TypesInfo

	for _, route := range routes {
		key := route.Method + " " + route.Path
		if key != "GET /users" {
			continue
		}

		fd, _, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler(%s) failed: %v", key, err)
		}
		pn := resolver.ResolveHandlerParams(fd.Type, info)
		params := contract.ExtractQueryParams(fd.Body, info, pn)

		if len(params) != 2 {
			t.Fatalf("ListUsers: expected 2 query params, got %d", len(params))
		}

		// Find page and limit params.
		paramMap := make(map[string]int)
		for i, p := range params {
			paramMap[p.Name] = i
		}

		pageIdx, ok := paramMap["page"]
		if !ok {
			t.Fatal("missing 'page' query param")
		}
		if !params[pageIdx].Required {
			t.Error("page should be required (has guard block)")
		}
		if params[pageIdx].In != "query" {
			t.Errorf("page In: expected 'query', got %q", params[pageIdx].In)
		}

		limitIdx, ok := paramMap["limit"]
		if !ok {
			t.Fatal("missing 'limit' query param")
		}
		if params[limitIdx].Required {
			t.Error("limit should not be required")
		}
	}
}

func TestExtractQueryParams_GinBasic(t *testing.T) {
	dir := testdataDir("gin-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.GinExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info := pkgs[0].TypesInfo

	for _, route := range routes {
		key := route.Method + " " + route.Path
		if key != "GET /api/v1/items" {
			continue
		}

		fd, _, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler(%s) failed: %v", key, err)
		}
		pn := resolver.ResolveHandlerParams(fd.Type, info)
		params := contract.ExtractQueryParams(fd.Body, info, pn)

		if len(params) != 2 {
			t.Fatalf("ListItems: expected 2 query params, got %d", len(params))
		}

		paramMap := make(map[string]int)
		for i, p := range params {
			paramMap[p.Name] = i
		}

		// search via c.Query("search")
		searchIdx, ok := paramMap["search"]
		if !ok {
			t.Fatal("missing 'search' query param")
		}
		if params[searchIdx].Default != nil {
			t.Error("search should not have a default")
		}

		// limit via c.DefaultQuery("limit", "20")
		limitIdx, ok := paramMap["limit"]
		if !ok {
			t.Fatal("missing 'limit' query param")
		}
		if params[limitIdx].Default == nil {
			t.Fatal("limit should have a default")
		}
		if *params[limitIdx].Default != "20" {
			t.Errorf("limit default: expected '20', got %q", *params[limitIdx].Default)
		}
	}
}

// --- Header Tests ---

func TestExtractHeaders_ChiBasic(t *testing.T) {
	// chi-basic handlers don't have explicit header access (just Content-Type
	// which is set via w.Header().Set, not r.Header.Get). So we expect 0 headers.
	dir := testdataDir("chi-basic")
	_, handlers := resolveRoutes(t, dir, &extractor.ChiExtractor{})

	pkgs, _ := loader.LoadPackages(dir, "./...")
	info := pkgs[0].TypesInfo

	for key, h := range handlers {
		headers := contract.ExtractHeaders(h.body, info, h.paramNames)
		if len(headers) != 0 {
			t.Errorf("%s: expected 0 headers, got %d", key, len(headers))
		}
	}
}

func TestExtractHeaders_AuthorizationExcluded(t *testing.T) {
	// The JWTAuth middleware in chi-basic uses r.Header.Get("Authorization").
	// This should be excluded. We test this by loading chi-basic and checking
	// that none of the handlers report Authorization as a header param.
	dir := testdataDir("chi-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	info := pkgs[0].TypesInfo

	// Walk all function decls and check for headers.
	for _, file := range pkgs[0].Syntax {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			pn := resolver.ResolveHandlerParams(fd.Type, info)
			headers := contract.ExtractHeaders(fd.Body, info, pn)
			for _, h := range headers {
				if h.Name == "Authorization" {
					t.Errorf("func %s: Authorization header should be excluded", fd.Name.Name)
				}
			}
		}
	}
}

// --- Body Tests ---

func TestExtractBody_ChiBasicJSON(t *testing.T) {
	dir := testdataDir("chi-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info := pkgs[0].TypesInfo

	for _, route := range routes {
		key := route.Method + " " + route.Path
		if key != "POST /users" {
			continue
		}

		fd, _, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler(%s) failed: %v", key, err)
		}
		pn := resolver.ResolveHandlerParams(fd.Type, info)
		result := contract.ExtractBody(fd.Body, info, pn)

		if result.BodyType == nil {
			t.Fatal("CreateUser: expected body type, got nil")
		}
		if result.ContentType != "application/json" {
			t.Errorf("CreateUser: expected content type 'application/json', got %q", result.ContentType)
		}
		if result.IsMultipart {
			t.Error("CreateUser: should not be multipart")
		}

		// Verify the resolved type name.
		named, ok := result.BodyType.(*types.Named)
		if !ok {
			t.Fatalf("CreateUser: expected named type, got %T", result.BodyType)
		}
		if named.Obj().Name() != "CreateUserRequest" {
			t.Errorf("CreateUser: expected type name 'CreateUserRequest', got %q", named.Obj().Name())
		}
	}
}

func TestExtractBody_GinBasicJSON(t *testing.T) {
	dir := testdataDir("gin-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.GinExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info := pkgs[0].TypesInfo

	for _, route := range routes {
		key := route.Method + " " + route.Path
		if key != "POST /api/v1/items" {
			continue
		}

		fd, _, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler(%s) failed: %v", key, err)
		}
		pn := resolver.ResolveHandlerParams(fd.Type, info)
		result := contract.ExtractBody(fd.Body, info, pn)

		if result.BodyType == nil {
			t.Fatal("CreateItem: expected body type, got nil")
		}
		if result.ContentType != "application/json" {
			t.Errorf("CreateItem: expected content type 'application/json', got %q", result.ContentType)
		}

		named, ok := result.BodyType.(*types.Named)
		if !ok {
			t.Fatalf("CreateItem: expected named type, got %T", result.BodyType)
		}
		if named.Obj().Name() != "CreateItemRequest" {
			t.Errorf("CreateItem: expected type name 'CreateItemRequest', got %q", named.Obj().Name())
		}
	}
}

func TestExtractBody_Multipart(t *testing.T) {
	dir := testdataDir("multipart")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info := pkgs[0].TypesInfo

	for _, route := range routes {
		key := route.Method + " " + route.Path

		fd, _, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler(%s) failed: %v", key, err)
		}
		pn := resolver.ResolveHandlerParams(fd.Type, info)
		result := contract.ExtractBody(fd.Body, info, pn)

		switch key {
		case "POST /users/{id}/avatar":
			if !result.IsMultipart {
				t.Errorf("%s: expected IsMultipart=true", key)
			}
			if result.ContentType != "multipart/form-data" {
				t.Errorf("%s: expected content type 'multipart/form-data', got %q", key, result.ContentType)
			}
			if len(result.FileParams) != 1 {
				t.Errorf("%s: expected 1 file param, got %d", key, len(result.FileParams))
			} else if result.FileParams[0].Name != "avatar" {
				t.Errorf("%s: expected file param 'avatar', got %q", key, result.FileParams[0].Name)
			}

		case "PUT /users/{id}/profile":
			if result.IsMultipart {
				t.Errorf("%s: should not be multipart", key)
			}
			if result.BodyType == nil {
				t.Errorf("%s: expected body type, got nil", key)
			}
			if result.ContentType != "application/json" {
				t.Errorf("%s: expected content type 'application/json', got %q", key, result.ContentType)
			}

		case "POST /documents":
			if !result.IsMultipart {
				t.Errorf("%s: expected IsMultipart=true", key)
			}
			if len(result.FileParams) != 1 {
				t.Errorf("%s: expected 1 file param, got %d", key, len(result.FileParams))
			} else if result.FileParams[0].Name != "document" {
				t.Errorf("%s: expected file param 'document', got %q", key, result.FileParams[0].Name)
			}
		}
	}
}

func TestExtractBody_NoBody(t *testing.T) {
	// GET /users in chi-basic has no body — should return empty result.
	dir := testdataDir("chi-basic")
	_, handlers := resolveRoutes(t, dir, &extractor.ChiExtractor{})

	pkgs, _ := loader.LoadPackages(dir, "./...")
	info := pkgs[0].TypesInfo

	h := handlers["GET /users"]
	result := contract.ExtractBody(h.body, info, h.paramNames)
	if result.BodyType != nil {
		t.Error("GET /users should not have a body type")
	}
	if result.IsMultipart {
		t.Error("GET /users should not be multipart")
	}
}

// --- Multipart with query param (FormValue) ---

func TestExtractQueryParams_FormValue(t *testing.T) {
	dir := testdataDir("multipart")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	ext := &extractor.ChiExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	info := pkgs[0].TypesInfo

	for _, route := range routes {
		key := route.Method + " " + route.Path
		if key != "POST /documents" {
			continue
		}

		fd, _, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler(%s) failed: %v", key, err)
		}
		pn := resolver.ResolveHandlerParams(fd.Type, info)
		params := contract.ExtractQueryParams(fd.Body, info, pn)

		if len(params) != 1 {
			t.Fatalf("UploadDocument: expected 1 query param, got %d", len(params))
		}
		if params[0].Name != "title" {
			t.Errorf("expected param name 'title', got %q", params[0].Name)
		}
	}
}
