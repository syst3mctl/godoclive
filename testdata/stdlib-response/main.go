// Package main is a net/http reverse-proxy-style service whose handlers are
// method values registered via http.HandlerFunc. The handlers relay upstream
// bytes verbatim but carry "doc-only" encode anchors so their response shapes
// can be extracted statically.
package main

import (
	"encoding/json"
	"net/http"

	"github.com/syst3mctl/godoclive/testdata/stdlib-response/httpx"
)

// proxy holds shared dependencies for the relay handlers.
type proxy struct {
	upstream string
}

// ItemResponse is a single item in the catalog.
type ItemResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ItemList is a paginated list of items.
type ItemList struct {
	Items      []ItemResponse `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

// Meta carries response envelope metadata that is inlined into detail responses.
type Meta struct {
	RequestID string `json:"request_id"`
}

// ItemDetail embeds Meta (Go JSON inlining) alongside the item fields.
type ItemDetail struct {
	Meta
	Item  ItemResponse `json:"item"`
	Extra string       `json:"extra,omitempty"`
}

// list relays the upstream list response verbatim. The if-false encode is a
// doc-only anchor describing the JSON shape clients receive.
func (p *proxy) list(w http.ResponseWriter, r *http.Request) {
	if false {
		_ = json.NewEncoder(w).Encode(ItemList{})
	}
	// Real handler relays upstream bytes.
	w.Write([]byte("relayed"))
}

// get returns a single item via a reachable encode.
func (p *proxy) get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ItemResponse{})
}

// create decodes a request body and responds with a detail envelope via a
// one-arg JSON helper. It returns text/plain on bad input via http.Error.
func (p *proxy) create(w http.ResponseWriter, r *http.Request) {
	var req ItemResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, ItemDetail{})
}

func main() {
	p := &proxy{upstream: "https://upstream.example"}

	mux := http.NewServeMux()
	mux.Handle("GET /v1/items", http.HandlerFunc(p.list))
	mux.Handle("GET /v1/items/{id}", http.HandlerFunc(p.get))
	mux.Handle("POST /v1/items", http.HandlerFunc(p.create))

	http.ListenAndServe(":8080", mux)
}
