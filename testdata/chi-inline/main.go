package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()

	// Direct function reference with non-standard param names.
	r.Get("/health", HealthCheck)

	// Inline function literal.
	r.Get("/inline", func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(map[string]string{"status": "ok"})
	})

	http.ListenAndServe(":3000", r)
}

// HealthCheck uses non-standard parameter names: rw and req.
func HealthCheck(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(map[string]string{"status": "healthy"})
}
