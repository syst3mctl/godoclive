// Package main mirrors the layout of a real service: main() owns the router
// but registers nothing itself — every route lives in another package.
package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/syst3mctl/godoclive/testdata/chi-multipkg/admin"
	"github.com/syst3mctl/godoclive/testdata/chi-multipkg/routes"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	// Registration handed off to another package.
	routes.Register(r)

	// Sub-router built by a factory in another package, mounted under a prefix.
	r.Mount("/admin", admin.Router())

	http.ListenAndServe(":8080", r)
}
