// Package main is a godoclive regression fixture for TYPED RESPONSE BODY extraction
// on net/http handlers. Handlers relay upstream bytes and document their response
// shape via a never-executed encode. Today godoclive emits request-body schemas for
// these but NOT response schemas. Every EXPECT below must hold after the fix.
package main

import (
	"encoding/json"
	"net/http"
)

type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type ItemList struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}
type ItemDetail struct {
	Item            // embedded: id + name must appear flattened on ItemDetail
	Tags []string `json:"tags"`
}
type CreateItemRequest struct {
	Name string `json:"name"`
}

type proxy struct{}

// GET /v1/items — EXPECT: 200 -> ItemList.
func (p *proxy) list(w http.ResponseWriter, r *http.Request) {
	if false {
		_ = json.NewEncoder(w).Encode(ItemList{})
	}
	relay(w, r)
}

// GET /v1/items/{id} — EXPECT: 200 -> ItemDetail with {id, name, tags} (Item inlined).
func (p *proxy) get(w http.ResponseWriter, r *http.Request) {
	if false {
		_ = json.NewEncoder(w).Encode(ItemDetail{})
	}
	relay(w, r)
}

// POST /v1/items — EXPECT: requestBody CreateItemRequest; 201 -> Item.
func (p *proxy) create(w http.ResponseWriter, r *http.Request) {
	var req CreateItemRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if false {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Item{})
	}
	relay(w, r)
}

// GET /v1/items/helper — EXPECT: 200 -> Item (helper tracing binds v's CONCRETE type, not `any`).
func (p *proxy) viaHelper(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, Item{})
}

// DELETE /v1/items/{id} — EXPECT: 204, no body.
func (p *proxy) remove(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func relay(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("{}")) }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	p := &proxy{}
	mux := http.NewServeMux()
	mux.Handle("GET /v1/items", http.HandlerFunc(p.list))
	mux.Handle("GET /v1/items/{id}", http.HandlerFunc(p.get))
	mux.Handle("POST /v1/items", http.HandlerFunc(p.create))
	mux.Handle("GET /v1/items/helper", http.HandlerFunc(p.viaHelper))
	mux.Handle("DELETE /v1/items/{id}", http.HandlerFunc(p.remove))
	_ = http.ListenAndServe(":8080", mux)
}
