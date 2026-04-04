package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func registerIgnoredRoutes(r chi.Router) {
	//godoclive:ignore
	r.Get("/debug/pprof", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	r.Post("/internal/webhook", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})) //godoclive:ignore

	// godoclive:skip
	r.Delete("/internal/nuke", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))
}
