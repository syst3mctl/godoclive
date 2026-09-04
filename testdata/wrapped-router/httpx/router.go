// Package httpx is the house HTTP layer every service in this fictional
// company builds on. Nothing outside it touches net/http's ServeMux directly.
package httpx

import "net/http"

// Router wraps a ServeMux behind the company's own registration API.
type Router struct {
	mux *http.ServeMux
}

// New builds a house router.
func New() *Router {
	return &Router{mux: http.NewServeMux()}
}

// Handle registers a handler for a method-and-path pattern.
func (r *Router) Handle(pattern string, h http.HandlerFunc) {
	r.mux.HandleFunc(pattern, h)
}

// Mux exposes the underlying mux for serving.
func (r *Router) Mux() *http.ServeMux {
	return r.mux
}
