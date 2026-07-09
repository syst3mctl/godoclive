// Package main exercises request-body extraction through SHARED DECODE HELPERS —
// the production idiom where no decode pattern appears in the handler itself:
//
//	var req createItemRequest
//	if !decodeJSON(w, r, &req) { return }
//
// Body extraction must trace the helper (one level), find the decode inside it,
// and map the `dst any` parameter back to the caller's concrete &req argument.
// Two helper shapes are covered: json.NewDecoder(r.Body).Decode(dst) and the
// read-then-unmarshal form (io.ReadAll + json.Unmarshal(raw, dst)).
package main

import (
	"encoding/json"
	"io"
	"net/http"
)

// createItemRequest is the POST /items body, decoded via the Decoder-style helper.
type createItemRequest struct {
	Name string `json:"name"`
	Qty  int    `json:"qty"`
}

// renameItemRequest is the POST /items/rename body, decoded via the
// read-then-unmarshal helper (the MaxBytesReader idiom).
type renameItemRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// decodeJSON is a Decoder-style shared helper: the decode target is the `dst`
// parameter, so the schema type lives at the CALL SITE, not here.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		http.Error(w, `{"error":"bad json"}`, http.StatusBadRequest)
		return false
	}
	return true
}

// bindJSON is a read-then-unmarshal shared helper (size-capped body), the other
// common production shape.
func bindJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, `{"error":"body too large"}`, http.StatusBadRequest)
		return false
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		http.Error(w, `{"error":"bad json"}`, http.StatusBadRequest)
		return false
	}
	return true
}

func handleCreateItem(w http.ResponseWriter, r *http.Request) {
	var req createItemRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(req)
}

func handleRenameItem(w http.ResponseWriter, r *http.Request) {
	var req renameItemRequest
	if !bindJSON(w, r, &req) {
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(req)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /items", handleCreateItem)
	mux.HandleFunc("POST /items/rename", handleRenameItem)
	_ = http.ListenAndServe(":8080", mux)
}
