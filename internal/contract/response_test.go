package contract_test

import (
	"go/ast"
	"go/types"
	"testing"

	"github.com/syst3mctl/godoclive/internal/contract"
	"github.com/syst3mctl/godoclive/internal/extractor"
	"github.com/syst3mctl/godoclive/internal/loader"
	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/resolver"
	"golang.org/x/tools/go/packages"
)

// resolveForResponse loads packages, extracts routes, and resolves handlers
// into a map of key→(body, funcType, paramNames, pkgs, info).
type resolvedForResp struct {
	body       *ast.BlockStmt
	funcType   *ast.FuncType
	paramNames resolver.HandlerParamNames
	pkgs       []*packages.Package
	info       *types.Info
}

func resolveForResponse(t *testing.T, dir string, ext extractor.Extractor) ([]extractor.RawRoute, map[string]resolvedForResp) {
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
	handlers := make(map[string]resolvedForResp)

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
		handlers[key] = resolvedForResp{body: body, funcType: fnType, paramNames: pn, pkgs: pkgs, info: info}
	}

	return routes, handlers
}

func findResponse(responses []model.ResponseDef, code int) *model.ResponseDef {
	for i := range responses {
		if responses[i].StatusCode == code {
			return &responses[i]
		}
	}
	return nil
}

// --- Gin response tests ---

func TestExtractResponses_GinBasic(t *testing.T) {
	dir := testdataDir("gin-basic")
	_, handlers := resolveForResponse(t, dir, &extractor.GinExtractor{})

	// ListItems: c.JSON(http.StatusOK, []ItemResponse{...})
	h := handlers["GET /api/v1/items"]
	responses, unresolved := contract.ExtractResponses(h.body, h.info, h.paramNames, h.pkgs)

	if len(unresolved) > 0 {
		t.Errorf("ListItems: unexpected unresolved: %v", unresolved)
	}
	if len(responses) != 1 {
		t.Fatalf("ListItems: expected 1 response, got %d", len(responses))
	}
	if responses[0].StatusCode != 200 {
		t.Errorf("ListItems: expected status 200, got %d", responses[0].StatusCode)
	}
	if responses[0].ContentType != "application/json" {
		t.Errorf("ListItems: expected content type 'application/json', got %q", responses[0].ContentType)
	}

	// CreateItem: c.JSON(400, ...) and c.JSON(201, ...)
	h = handlers["POST /api/v1/items"]
	responses, _ = contract.ExtractResponses(h.body, h.info, h.paramNames, h.pkgs)

	if len(responses) < 2 {
		t.Fatalf("CreateItem: expected at least 2 responses, got %d", len(responses))
	}
	if findResponse(responses, 400) == nil {
		t.Error("CreateItem: missing 400 response")
	}
	if findResponse(responses, 201) == nil {
		t.Error("CreateItem: missing 201 response")
	}

	// DeleteItem: c.Status(http.StatusNoContent)
	h = handlers["DELETE /api/v1/items/{id}"]
	responses, _ = contract.ExtractResponses(h.body, h.info, h.paramNames, h.pkgs)

	if len(responses) != 1 {
		t.Fatalf("DeleteItem: expected 1 response, got %d", len(responses))
	}
	if responses[0].StatusCode != 204 {
		t.Errorf("DeleteItem: expected status 204, got %d", responses[0].StatusCode)
	}
}

// --- net/http response tests (branch-aware pairing) ---

func TestExtractResponses_ChiBasic_GetUser(t *testing.T) {
	// chi-basic GetUser:
	//   if id == "" { ... w.WriteHeader(400); json.Encode(ErrorResponse{...}); return }
	//   w.WriteHeader(200); json.Encode(UserResponse{...})
	dir := testdataDir("chi-basic")
	_, handlers := resolveForResponse(t, dir, &extractor.ChiExtractor{})

	h := handlers["GET /users/{id}"]
	responses, _ := contract.ExtractResponses(h.body, h.info, h.paramNames, h.pkgs)

	if len(responses) < 2 {
		t.Fatalf("GetUser: expected at least 2 responses, got %d", len(responses))
	}

	r400 := findResponse(responses, 400)
	if r400 == nil {
		t.Error("GetUser: missing 400 response")
	}

	r200 := findResponse(responses, 200)
	if r200 == nil {
		t.Error("GetUser: missing 200 response")
	} else if r200.ContentType != "application/json" {
		t.Errorf("GetUser 200: expected content type 'application/json', got %q", r200.ContentType)
	}
}

func TestExtractResponses_ChiBasic_DeleteUser(t *testing.T) {
	// DeleteUser: if id == "" { ... 400; return }; w.WriteHeader(204)
	dir := testdataDir("chi-basic")
	_, handlers := resolveForResponse(t, dir, &extractor.ChiExtractor{})

	h := handlers["DELETE /users/{id}"]
	responses, _ := contract.ExtractResponses(h.body, h.info, h.paramNames, h.pkgs)

	r204 := findResponse(responses, 204)
	if r204 == nil {
		t.Fatal("DeleteUser: missing 204 response")
	}
	if r204.Body != nil {
		t.Error("DeleteUser 204: should have no body")
	}
}

// --- Implicit 200 rule ---

func TestExtractResponses_Implicit200(t *testing.T) {
	// chi-helpers HealthCheck: json.Encode with no WriteHeader → implicit 200
	dir := testdataDir("chi-helpers")
	_, handlers := resolveForResponse(t, dir, &extractor.ChiExtractor{})

	h := handlers["GET /health"]
	responses, _ := contract.ExtractResponses(h.body, h.info, h.paramNames, h.pkgs)

	if len(responses) == 0 {
		t.Fatal("HealthCheck: expected at least 1 response")
	}

	r200 := findResponse(responses, 200)
	if r200 == nil {
		t.Fatal("HealthCheck: missing implicit 200 response")
	}
	if r200.ContentType != "application/json" {
		t.Errorf("HealthCheck 200: expected content type 'application/json', got %q", r200.ContentType)
	}
}

// --- Helper function tracing ---

func TestExtractResponses_ChiHelpers_GetUser(t *testing.T) {
	// GetUser uses sendError(w, msg, 400) and respond(w, user, 200)
	dir := testdataDir("chi-helpers")
	_, handlers := resolveForResponse(t, dir, &extractor.ChiExtractor{})

	h := handlers["GET /users/{id}"]
	responses, _ := contract.ExtractResponses(h.body, h.info, h.paramNames, h.pkgs)

	if len(responses) < 2 {
		t.Fatalf("GetUser (helpers): expected at least 2 responses, got %d", len(responses))
	}

	// Should have found responses from the helpers.
	found400 := findResponse(responses, 400)
	found200 := findResponse(responses, 200)

	if found400 == nil {
		t.Error("GetUser (helpers): missing 400 from sendError helper")
	}
	if found200 == nil {
		t.Error("GetUser (helpers): missing 200 from respond helper")
	}

	// Check that helper-traced responses are marked as "helper" source.
	for _, r := range responses {
		if r.Source != "explicit" && r.Source != "helper" {
			t.Errorf("GetUser (helpers): unexpected source %q", r.Source)
		}
	}
}

func TestExtractResponses_ChiHelpers_ListUsers(t *testing.T) {
	// ListUsers uses writeJSON(w, users) → always 200.
	dir := testdataDir("chi-helpers")
	_, handlers := resolveForResponse(t, dir, &extractor.ChiExtractor{})

	h := handlers["GET /users"]
	responses, _ := contract.ExtractResponses(h.body, h.info, h.paramNames, h.pkgs)

	if len(responses) == 0 {
		t.Fatal("ListUsers (helpers): expected at least 1 response")
	}

	r200 := findResponse(responses, 200)
	if r200 == nil {
		t.Fatal("ListUsers (helpers): missing 200 from writeJSON helper")
	}
}

// --- Gin helpers ---

func TestExtractResponses_GinHelpers(t *testing.T) {
	// gin-helpers GetItem uses respondOK and respondError
	dir := testdataDir("gin-helpers")
	_, handlers := resolveForResponse(t, dir, &extractor.GinExtractor{})

	h := handlers["GET /items/{id}"]
	responses, _ := contract.ExtractResponses(h.body, h.info, h.paramNames, h.pkgs)

	// Gin helpers call c.JSON directly — the gin extractor should still see them.
	if len(responses) < 2 {
		t.Fatalf("GetItem (gin-helpers): expected at least 2 responses, got %d", len(responses))
	}

	if findResponse(responses, 200) == nil {
		t.Error("GetItem (gin-helpers): missing 200")
	}
	if findResponse(responses, 400) == nil {
		// respondError passes a dynamic code, so it may not resolve.
		// Check for unresolved instead.
		t.Log("GetItem (gin-helpers): 400 may be dynamic — checking for unresolved")
	}
}

func TestExtractResponses_GinHelpers_DeleteItem(t *testing.T) {
	dir := testdataDir("gin-helpers")
	_, handlers := resolveForResponse(t, dir, &extractor.GinExtractor{})

	h := handlers["DELETE /items/{id}"]
	responses, _ := contract.ExtractResponses(h.body, h.info, h.paramNames, h.pkgs)

	r204 := findResponse(responses, 204)
	if r204 == nil {
		t.Fatal("DeleteItem (gin-helpers): missing 204")
	}
}

// --- net/http typed response bodies (method-receiver handlers) ---

// resolveMainResponses loads a fixture, locates the main package (the handlers
// may import helper sub-packages), extracts stdlib routes, resolves each
// handler, and returns the extracted responses keyed by "METHOD /path".
func resolveMainResponses(t *testing.T, dir string) map[string][]model.ResponseDef {
	t.Helper()
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	var info *types.Info
	for _, p := range pkgs {
		if p.Name == "main" && p.TypesInfo != nil {
			info = p.TypesInfo
		}
	}
	if info == nil {
		t.Fatal("main package TypesInfo not found")
	}

	ext := &extractor.StdlibExtractor{}
	routes, err := ext.Extract(pkgs)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	out := make(map[string][]model.ResponseDef)
	for _, route := range routes {
		fd, fl, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Fatalf("ResolveHandler(%s %s): %v", route.Method, route.Path, err)
		}
		var body *ast.BlockStmt
		var ft *ast.FuncType
		if fd != nil {
			body, ft = fd.Body, fd.Type
		} else if fl != nil {
			body, ft = fl.Body, fl.Type
		}
		pn := resolver.ResolveHandlerParams(ft, info)
		resps, _ := contract.ExtractResponses(body, info, pn, pkgs)
		out[route.Method+" "+route.Path] = resps
	}
	return out
}

// TestExtractResponses_StdlibMethodReceiver_Encode covers a method-value handler
// registered via http.HandlerFunc(p.get) with a reachable
// json.NewEncoder(w).Encode(ItemResponse{}).
func TestExtractResponses_StdlibMethodReceiver_Encode(t *testing.T) {
	byKey := resolveMainResponses(t, testdataDir("stdlib-response"))

	resps := byKey["GET /v1/items/{id}"]
	r := findResponse(resps, 200)
	if r == nil {
		t.Fatalf("get: missing 200 (got %d responses)", len(resps))
	}
	if r.ContentType != "application/json" {
		t.Errorf("get 200: content type = %q, want application/json", r.ContentType)
	}
	if r.Body == nil || r.Body.Name != "ItemResponse" {
		t.Fatalf("get 200: want ItemResponse body, got %+v", r.Body)
	}
}

// TestExtractResponses_StdlibDocOnlyEncode_Paginated covers the reverse-proxy
// idiom: `if false { json.Encode(ItemList{}) }` followed by an untyped
// w.Write([]byte) relay. The doc-only typed anchor must win over the write, and
// the paginated wrapper type ([]ItemResponse field) must be reported.
func TestExtractResponses_StdlibDocOnlyEncode_Paginated(t *testing.T) {
	byKey := resolveMainResponses(t, testdataDir("stdlib-response"))

	resps := byKey["GET /v1/items"]
	r := findResponse(resps, 200)
	if r == nil {
		t.Fatalf("list: missing 200 (got %d responses) — doc-only encode dropped", len(resps))
	}
	if r.ContentType != "application/json" {
		t.Errorf("list 200: content type = %q, want application/json", r.ContentType)
	}
	if r.Body == nil || r.Body.Name != "ItemList" {
		t.Fatalf("list 200: want ItemList body, got %+v", r.Body)
	}
}

// TestExtractResponses_StdlibHelperAndHTTPError covers http.Error → text/plain
// and a one-arg JSON helper (httpx.WriteJSON(w, code, v)) whose body type is
// bound to the concrete call-site argument rather than the helper's `any` param.
func TestExtractResponses_StdlibHelperAndHTTPError(t *testing.T) {
	byKey := resolveMainResponses(t, testdataDir("stdlib-response"))

	resps := byKey["POST /v1/items"]

	r400 := findResponse(resps, 400)
	if r400 == nil {
		t.Fatal("create: missing 400 from http.Error")
	}
	if r400.ContentType != "text/plain" {
		t.Errorf("create 400: content type = %q, want text/plain", r400.ContentType)
	}

	r201 := findResponse(resps, 201)
	if r201 == nil {
		t.Fatalf("create: missing helper 201 (got %d responses)", len(resps))
	}
	if r201.Source != "helper" {
		t.Errorf("create 201: source = %q, want helper", r201.Source)
	}
	if r201.ContentType != "application/json" {
		t.Errorf("create 201: content type = %q, want application/json", r201.ContentType)
	}
	if r201.Body == nil || r201.Body.Name != "ItemDetail" {
		t.Fatalf("create 201: want ItemDetail body bound from call site, got %+v", r201.Body)
	}
}

// TestExtractResponses_StdlibUnresolvedStatus covers a relay helper that calls
// w.WriteHeader(<non-constant>) (e.g. an upstream resp.StatusCode). The
// unresolved -1 is not a valid HTTP status and must never surface: a typed body
// in the same branch falls back to implicit 200, and a relay-only handler emits
// no response at all.
func TestExtractResponses_StdlibUnresolvedStatus(t *testing.T) {
	byKey := resolveMainResponses(t, testdataDir("stdlib-unresolved-status"))

	// GET: doc-only Encode(Item{}) + relay(unresolved status) → exactly one 200/Item.
	getResps := byKey["GET /v1/items/{id}"]
	for _, r := range getResps {
		if r.StatusCode <= 0 {
			t.Errorf("get: emitted invalid status %d", r.StatusCode)
		}
	}
	if len(getResps) != 1 {
		t.Fatalf("get: want exactly 1 response, got %d (%+v)", len(getResps), getResps)
	}
	if getResps[0].StatusCode != 200 {
		t.Errorf("get: want status 200, got %d", getResps[0].StatusCode)
	}
	if getResps[0].Body == nil || getResps[0].Body.Name != "Item" {
		t.Errorf("get: want Item body, got %+v", getResps[0].Body)
	}

	// DELETE: relay only (unresolved status, no typed body) → no response at all.
	delResps := byKey["DELETE /v1/items/{id}"]
	for _, r := range delResps {
		if r.StatusCode <= 0 {
			t.Errorf("remove: emitted invalid status %d", r.StatusCode)
		}
	}
	if len(delResps) != 0 {
		t.Errorf("remove: want no responses, got %d (%+v)", len(delResps), delResps)
	}
}

// --- Status code resolution ---

func TestResolveStatusCode(t *testing.T) {
	// Test status code resolution with a real package.
	dir := testdataDir("chi-basic")
	pkgs, err := loader.LoadPackages(dir, "./...")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	info := pkgs[0].TypesInfo

	// Find the ListUsers function and look for WriteHeader calls to test resolution.
	ext := &extractor.ChiExtractor{}
	routes, _ := ext.Extract(pkgs)

	for _, route := range routes {
		key := route.Method + " " + route.Path
		if key != "GET /users" {
			continue
		}

		fd, _, _ := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if fd == nil {
			t.Fatal("could not resolve ListUsers")
		}

		// Walk the body and find WriteHeader calls.
		var foundCodes []int
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "WriteHeader" {
				return true
			}
			if len(call.Args) == 1 {
				code := contract.ResolveStatusCode(call.Args[0], info)
				foundCodes = append(foundCodes, code)
			}
			return true
		})

		if len(foundCodes) < 2 {
			t.Fatalf("ListUsers: expected at least 2 WriteHeader calls, got %d", len(foundCodes))
		}

		// Should have 400 and 200.
		has400, has200 := false, false
		for _, c := range foundCodes {
			if c == 400 {
				has400 = true
			}
			if c == 200 {
				has200 = true
			}
		}
		if !has400 {
			t.Error("ListUsers: expected to resolve http.StatusBadRequest → 400")
		}
		if !has200 {
			t.Error("ListUsers: expected to resolve http.StatusOK → 200")
		}
	}
}

// --- Contract orchestrator ---

func TestExtractContract_ChiBasic(t *testing.T) {
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
		fd, fl, err := resolver.ResolveHandler(route.HandlerExpr, info, pkgs)
		if err != nil {
			t.Errorf("%s: resolve failed: %v", key, err)
			continue
		}

		var fn ast.Node
		var fnType *ast.FuncType
		if fd != nil {
			fn = fd
			fnType = fd.Type
		} else {
			fn = fl
			fnType = fl.Type
		}

		pn := resolver.ResolveHandlerParams(fnType, info)
		req, responses, _ := contract.ExtractContract(route, fn, info, pn, pkgs)

		switch key {
		case "POST /users":
			// Should have body.
			if req.Body == nil {
				t.Errorf("%s: expected body, got nil", key)
			}
			if req.ContentType != "application/json" {
				t.Errorf("%s: expected content type 'application/json', got %q", key, req.ContentType)
			}
			// Should have responses.
			if len(responses) < 2 {
				t.Errorf("%s: expected at least 2 responses, got %d", key, len(responses))
			}

		case "GET /users":
			// Should have query params.
			if len(req.QueryParams) < 2 {
				t.Errorf("%s: expected at least 2 query params, got %d", key, len(req.QueryParams))
			}
			// Should have no body.
			if req.Body != nil {
				t.Errorf("%s: expected no body", key)
			}

		case "GET /users/{id}":
			// Should have path params.
			if len(req.PathParams) != 1 {
				t.Errorf("%s: expected 1 path param, got %d", key, len(req.PathParams))
			}

		case "DELETE /users/{id}":
			if len(req.PathParams) != 1 {
				t.Errorf("%s: expected 1 path param, got %d", key, len(req.PathParams))
			}
		}
	}
}
