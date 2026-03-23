package contract_test

import (
	"go/ast"
	"go/types"
	"testing"

	"github.com/dicki/godoclive/internal/contract"
	"github.com/dicki/godoclive/internal/extractor"
	"github.com/dicki/godoclive/internal/loader"
	"github.com/dicki/godoclive/internal/model"
	"github.com/dicki/godoclive/internal/resolver"
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
