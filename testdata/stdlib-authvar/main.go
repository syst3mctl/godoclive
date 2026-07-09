// Package main exercises the "middleware factory held in a local variable"
// idiom common in production edges:
//
//	requireAuth := RequireAuth(logger)
//	mux.Handle("GET /items", requireAuth(http.HandlerFunc(handleItems)))
//
// The middleware expression on the route is the IDENT of a local variable, not
// a function declaration — auth detection must trace the variable to its
// initializer (the factory call) and scan the factory's body.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// ItemsResponse is the GET /items payload.
type ItemsResponse struct {
	Items []string `json:"items"`
}

// RequireAuth is a middleware FACTORY: it returns the actual middleware. The
// bearer-token check lives in the innermost handler func.
func RequireAuth(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := r.Header.Get("Authorization")
			if !strings.HasPrefix(raw, "Bearer ") {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			logger.Println("authorized")
			next.ServeHTTP(w, r)
		})
	}
}

func handleItems(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ItemsResponse{Items: []string{"a", "b"}})
}

func handlePublic(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
	logger := log.Default()
	mux := http.NewServeMux()

	// The factory result is held in a local var — the route's middleware expr
	// is the var ident, one hop away from the factory FuncDecl.
	requireAuth := RequireAuth(logger)

	mux.Handle("GET /items", requireAuth(http.HandlerFunc(handleItems)))
	mux.HandleFunc("GET /public", handlePublic)

	_ = http.ListenAndServe(":8080", mux)
}
