package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/syst3mctl/godoclive/testdata/chi-multipkg/handlers"
)

// Router builds a self-contained sub-router. Its signature names no chi type —
// the caller mounts it under a prefix.
func Router() http.Handler {
	r := chi.NewRouter()
	r.Use(RequireAdmin)
	r.Get("/dashboard", handlers.Dashboard)
	return r
}

// RequireAdmin rejects requests without an admin API key.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") == "" {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
