package main

import (
	"encoding/json"
	"net/http"
)

type Item struct {
	ID string `json:"id"`
}

type proxy struct{}

// relay mimics a verbatim upstream relay: status is a runtime value, not a constant,
// so WriteHeader(status) is unresolvable.
func (p *proxy) relay(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
	_, _ = w.Write([]byte("{}"))
}

// GET /v1/items/{id} — typed doc-only anchor + relay with unresolved status.
// EXPECT: exactly one response — 200 -> Item. NO status -1.
func (p *proxy) get(w http.ResponseWriter, r *http.Request) {
	p.relay(w, upstreamStatus())
	if false {
		_ = json.NewEncoder(w).Encode(Item{})
	}
}

// DELETE /v1/items/{id} — relay only. EXPECT: NO bogus -1 response.
func (p *proxy) remove(w http.ResponseWriter, r *http.Request) {
	p.relay(w, upstreamStatus())
}

func upstreamStatus() int { return http.StatusOK }

func main() {
	p := &proxy{}
	mux := http.NewServeMux()
	mux.Handle("GET /v1/items/{id}", http.HandlerFunc(p.get))
	mux.Handle("DELETE /v1/items/{id}", http.HandlerFunc(p.remove))
	_ = http.ListenAndServe(":8080", mux)
}
