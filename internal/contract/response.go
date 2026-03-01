package contract

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"github.com/syst3mctl/godoclive/internal/model"
	"github.com/syst3mctl/godoclive/internal/resolver"
	"golang.org/x/tools/go/packages"
)

// responseEvent represents a single response-related action in a handler body.
type responseEvent struct {
	kind       string     // "status" | "body" | "combined" | "helper"
	statusCode int        // for "status" and "combined" events
	bodyType   types.Type // for "body" and "combined" events (nil for status-only)
	contentType string    // content type if determinable
	pos        token.Pos  // source position for ordering
}

// httpStatusConstants maps net/http status constant names to their integer values.
var httpStatusConstants = map[string]int{
	"StatusContinue":           100,
	"StatusOK":                 200,
	"StatusCreated":            201,
	"StatusAccepted":           202,
	"StatusNoContent":          204,
	"StatusMovedPermanently":   301,
	"StatusFound":              302,
	"StatusBadRequest":         400,
	"StatusUnauthorized":       401,
	"StatusForbidden":          403,
	"StatusNotFound":           404,
	"StatusMethodNotAllowed":   405,
	"StatusConflict":           409,
	"StatusUnprocessableEntity": 422,
	"StatusTooManyRequests":    429,
	"StatusInternalServerError": 500,
	"StatusBadGateway":         502,
	"StatusServiceUnavailable": 503,
}

// ExtractResponses walks a handler function body and extracts all response
// definitions. For gin handlers, responses are co-located (easy). For net/http
// handlers, branch-aware pairing is used.
func ExtractResponses(body *ast.BlockStmt, info *types.Info, paramNames resolver.HandlerParamNames, pkgs []*packages.Package) ([]model.ResponseDef, []string) {
	if body == nil || info == nil {
		return nil, nil
	}

	// Determine which extraction strategy to use based on which param names are set.
	if paramNames.GinCtx != "" {
		return extractGinResponses(body, info, paramNames, pkgs)
	}
	return extractHTTPResponses(body, info, paramNames, pkgs)
}

// --- Gin response extraction (easy case) ---

func extractGinResponses(body *ast.BlockStmt, info *types.Info, pn resolver.HandlerParamNames, pkgs []*packages.Package) ([]model.ResponseDef, []string) {
	var responses []model.ResponseDef
	var unresolved []string
	seen := make(map[int]bool) // deduplicate by status code

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			// Check for helper function calls: someFunc(c, ...) where c is the gin context.
			if pkgs != nil {
				helperResps, helperUnresolved := traceGinHelper(call, info, pn, pkgs)
				if helperResps != nil {
					for _, r := range helperResps {
						if !seen[r.StatusCode] {
							seen[r.StatusCode] = true
							responses = append(responses, r)
						}
					}
					unresolved = append(unresolved, helperUnresolved...)
					return false
				}
			}
			return true
		}

		recv, ok := sel.X.(*ast.Ident)
		if !ok || recv.Name != pn.GinCtx {
			// Could be a helper method call.
			return true
		}

		switch sel.Sel.Name {
		case "JSON", "AbortWithStatusJSON":
			if len(call.Args) >= 2 {
				code := ResolveStatusCode(call.Args[0], info)
				if code == -1 {
					unresolved = append(unresolved, unresolvedStatusMsg(call, info))
					return true
				}
				if seen[code] {
					return true
				}
				seen[code] = true
				resp := model.ResponseDef{
					StatusCode:  code,
					ContentType: "application/json",
					Source:      "explicit",
					Description: descriptionForStatus(code),
				}
				bodyType := resolveBodyType(call.Args[1], info)
				if bodyType != nil {
					resp.Body = typeRefDef(bodyType)
				}
				responses = append(responses, resp)
			}

		case "Status":
			if len(call.Args) >= 1 {
				code := ResolveStatusCode(call.Args[0], info)
				if code == -1 {
					unresolved = append(unresolved, unresolvedStatusMsg(call, info))
					return true
				}
				if !seen[code] {
					seen[code] = true
					responses = append(responses, model.ResponseDef{
						StatusCode:  code,
						ContentType: "",
						Source:      "explicit",
						Description: descriptionForStatus(code),
					})
				}
			}

		case "File":
			if !seen[200] {
				seen[200] = true
				responses = append(responses, model.ResponseDef{
					StatusCode:  200,
					ContentType: "application/octet-stream",
					Source:      "explicit",
					Description: "OK",
				})
			}
		}

		return true
	})

	return responses, unresolved
}

// --- net/http response extraction (hard case) ---

func extractHTTPResponses(body *ast.BlockStmt, info *types.Info, pn resolver.HandlerParamNames, pkgs []*packages.Package) ([]model.ResponseDef, []string) {
	var unresolved []string

	// Step 1: Collect all response events.
	events := collectResponseEvents(body, info, pn, pkgs, &unresolved, false)

	// Step 2: Collect all return statement positions.
	returns := collectReturnPositions(body)

	// Step 3: Pair events using return boundaries.
	responses := pairEvents(events, returns, info)

	return responses, unresolved
}

// collectResponseEvents walks a block statement and finds all response-related calls.
func collectResponseEvents(body *ast.BlockStmt, info *types.Info, pn resolver.HandlerParamNames, pkgs []*packages.Package, unresolved *[]string, isHelper bool) []responseEvent {
	var events []responseEvent

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// http.Error(w, msg, code) → combined event
		if ev, ok := matchHTTPError(call, info, pn.Writer); ok {
			events = append(events, ev)
			return false
		}

		// w.WriteHeader(code) → status event
		if ev, ok := matchWriteHeader(call, info, pn.Writer); ok {
			events = append(events, ev)
			return false
		}

		// json.NewEncoder(w).Encode(val) → body event
		if ev, ok := matchJSONEncode(call, info, pn.Writer); ok {
			events = append(events, ev)
			return false
		}

		// w.Write(data) → body event
		if ev, ok := matchWriteCall(call, pn.Writer); ok {
			events = append(events, ev)
			return false
		}

		// Helper function tracing (one level only, not inside helpers).
		if !isHelper && pkgs != nil {
			if helperEvents, unresolvedMsgs := traceHelper(call, info, pn, pkgs); helperEvents != nil {
				events = append(events, helperEvents...)
				if unresolvedMsgs != nil {
					*unresolved = append(*unresolved, unresolvedMsgs...)
				}
				return false
			}
		}

		return true
	})

	return events
}

// collectReturnPositions finds all return statement positions in a block.
func collectReturnPositions(body *ast.BlockStmt) []token.Pos {
	var returns []token.Pos
	ast.Inspect(body, func(n ast.Node) bool {
		// Don't descend into nested function literals.
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if ret, ok := n.(*ast.ReturnStmt); ok {
			returns = append(returns, ret.Pos())
		}
		return true
	})
	return returns
}

// pairEvents groups response events by return boundaries and pairs status+body.
func pairEvents(events []responseEvent, returns []token.Pos, info *types.Info) []model.ResponseDef {
	if len(events) == 0 {
		return nil
	}

	// Add a sentinel return at the end (for the final implicit return).
	returns = append(returns, token.Pos(^uint(0)>>1)) // max Pos

	var responses []model.ResponseDef
	seen := make(map[int]bool)

	prevBoundary := token.Pos(0)

	for _, ret := range returns {
		// Find events in this branch: prevBoundary < event.pos <= ret.
		var branchEvents []responseEvent
		for _, ev := range events {
			if ev.pos > prevBoundary && ev.pos <= ret {
				branchEvents = append(branchEvents, ev)
			}
		}

		if len(branchEvents) > 0 {
			resp := pairBranchEvents(branchEvents)
			if resp != nil && !seen[resp.StatusCode] {
				seen[resp.StatusCode] = true
				responses = append(responses, *resp)
			}
		}

		prevBoundary = ret
	}

	return responses
}

// pairBranchEvents takes events within a single return-delimited branch and
// pairs them into a ResponseDef.
func pairBranchEvents(events []responseEvent) *model.ResponseDef {
	var statusCode int
	var bodyType types.Type
	var contentType string
	hasStatus := false
	source := "explicit"

	for _, ev := range events {
		switch ev.kind {
		case "combined":
			// Self-paired — return directly.
			resp := &model.ResponseDef{
				StatusCode:  ev.statusCode,
				ContentType: ev.contentType,
				Source:      source,
				Description: descriptionForStatus(ev.statusCode),
			}
			if ev.bodyType != nil {
				resp.Body = typeRefDef(ev.bodyType)
			}
			return resp

		case "status":
			statusCode = ev.statusCode
			hasStatus = true

		case "body":
			bodyType = ev.bodyType
			if ev.contentType != "" {
				contentType = ev.contentType
			}

		case "helper":
			statusCode = ev.statusCode
			bodyType = ev.bodyType
			hasStatus = ev.statusCode != 0
			if ev.contentType != "" {
				contentType = ev.contentType
			}
			source = "helper"
		}
	}

	// If we have nothing, skip.
	if !hasStatus && bodyType == nil {
		return nil
	}

	// Implicit 200 rule: body with no explicit status.
	if !hasStatus {
		statusCode = 200
	}

	if contentType == "" && bodyType != nil {
		contentType = "application/json"
	}

	resp := &model.ResponseDef{
		StatusCode:  statusCode,
		ContentType: contentType,
		Source:      source,
		Description: descriptionForStatus(statusCode),
	}
	if bodyType != nil {
		resp.Body = typeRefDef(bodyType)
	}
	return resp
}

// --- Pattern matchers ---

// matchHTTPError matches http.Error(w, msg, code).
func matchHTTPError(call *ast.CallExpr, info *types.Info, writerName string) (responseEvent, bool) {
	if writerName == "" {
		return responseEvent{}, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Error" {
		return responseEvent{}, false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != "http" {
		return responseEvent{}, false
	}
	if len(call.Args) != 3 {
		return responseEvent{}, false
	}
	// First arg should be the writer.
	wIdent, ok := call.Args[0].(*ast.Ident)
	if !ok || wIdent.Name != writerName {
		return responseEvent{}, false
	}

	code := ResolveStatusCode(call.Args[2], info)
	return responseEvent{
		kind:        "combined",
		statusCode:  code,
		contentType: "text/plain",
		pos:         call.Pos(),
	}, true
}

// matchWriteHeader matches w.WriteHeader(code).
func matchWriteHeader(call *ast.CallExpr, info *types.Info, writerName string) (responseEvent, bool) {
	if writerName == "" {
		return responseEvent{}, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "WriteHeader" {
		return responseEvent{}, false
	}
	recv, ok := sel.X.(*ast.Ident)
	if !ok || recv.Name != writerName {
		return responseEvent{}, false
	}
	if len(call.Args) != 1 {
		return responseEvent{}, false
	}

	code := ResolveStatusCode(call.Args[0], info)
	return responseEvent{
		kind:       "status",
		statusCode: code,
		pos:        call.Pos(),
	}, true
}

// matchJSONEncode matches json.NewEncoder(w).Encode(val).
func matchJSONEncode(call *ast.CallExpr, info *types.Info, writerName string) (responseEvent, bool) {
	if writerName == "" {
		return responseEvent{}, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Encode" {
		return responseEvent{}, false
	}

	// sel.X should be json.NewEncoder(w)
	newEncCall, ok := sel.X.(*ast.CallExpr)
	if !ok {
		return responseEvent{}, false
	}

	newEncSel, ok := newEncCall.Fun.(*ast.SelectorExpr)
	if !ok || newEncSel.Sel.Name != "NewEncoder" {
		return responseEvent{}, false
	}

	ident, ok := newEncSel.X.(*ast.Ident)
	if !ok || ident.Name != "json" {
		return responseEvent{}, false
	}

	// Check arg is the writer.
	if len(newEncCall.Args) != 1 {
		return responseEvent{}, false
	}
	wIdent, ok := newEncCall.Args[0].(*ast.Ident)
	if !ok || wIdent.Name != writerName {
		return responseEvent{}, false
	}

	var bodyType types.Type
	if len(call.Args) == 1 {
		bodyType = resolveBodyType(call.Args[0], info)
	}

	return responseEvent{
		kind:        "body",
		bodyType:    bodyType,
		contentType: "application/json",
		pos:         call.Pos(),
	}, true
}

// matchWriteCall matches w.Write(data).
func matchWriteCall(call *ast.CallExpr, writerName string) (responseEvent, bool) {
	if writerName == "" {
		return responseEvent{}, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Write" {
		return responseEvent{}, false
	}
	recv, ok := sel.X.(*ast.Ident)
	if !ok || recv.Name != writerName {
		return responseEvent{}, false
	}

	return responseEvent{
		kind:        "body",
		contentType: "text/plain",
		pos:         call.Pos(),
	}, true
}

// --- Status code resolution ---

// ResolveStatusCode resolves a status code expression to an integer.
// Uses go/constant for compile-time constants, falls back to known http.StatusXxx names.
// Returns -1 if unresolvable.
func ResolveStatusCode(expr ast.Expr, info *types.Info) int {
	// Primary: use the type checker's constant evaluation.
	if tv, ok := info.Types[expr]; ok && tv.Value != nil {
		if i, ok := constant.Int64Val(tv.Value); ok {
			return int(i)
		}
	}

	// Fallback: check for http.StatusXxx selector expressions.
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if code, found := httpStatusConstants[sel.Sel.Name]; found {
			return code
		}
	}

	// Fallback: check for integer literal.
	if lit, ok := expr.(*ast.BasicLit); ok {
		if v := extractIntLit(lit); v > 0 {
			return v
		}
	}

	return -1
}

// extractIntLit extracts an integer from a BasicLit.
func extractIntLit(lit *ast.BasicLit) int {
	if lit.Kind != token.INT {
		return 0
	}
	val := 0
	for _, c := range lit.Value {
		if c < '0' || c > '9' {
			return 0
		}
		val = val*10 + int(c-'0')
	}
	return val
}

// --- Helpers ---

// resolveBodyType extracts the types.Type of a body expression.
func resolveBodyType(expr ast.Expr, info *types.Info) types.Type {
	t := info.TypeOf(expr)
	if t == nil {
		return nil
	}
	// Dereference pointer.
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

// typeRefDef creates a minimal TypeDef as a type reference. The full struct
// mapping is done later by the struct mapper phase.
func typeRefDef(t types.Type) *model.TypeDef {
	if t == nil {
		return nil
	}

	name := t.String()
	pkg := ""

	// Try to extract the named type info.
	if named, ok := t.(*types.Named); ok {
		name = named.Obj().Name()
		if named.Obj().Pkg() != nil {
			pkg = named.Obj().Pkg().Path()
		}
	}

	return &model.TypeDef{
		Name:    name,
		Package: pkg,
	}
}

// descriptionForStatus returns a human-readable description for a status code.
func descriptionForStatus(code int) string {
	switch code {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 202:
		return "Accepted"
	case 204:
		return "No Content"
	case 301:
		return "Moved Permanently"
	case 302:
		return "Found"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 405:
		return "Method Not Allowed"
	case 409:
		return "Conflict"
	case 422:
		return "Unprocessable Entity"
	case 429:
		return "Too Many Requests"
	case 500:
		return "Internal Server Error"
	case 502:
		return "Bad Gateway"
	case 503:
		return "Service Unavailable"
	default:
		return ""
	}
}

// unresolvedStatusMsg generates an unresolved message for an unresolvable status code.
func unresolvedStatusMsg(call *ast.CallExpr, info *types.Info) string {
	return "status code: variable used — value not a compile-time constant"
}
