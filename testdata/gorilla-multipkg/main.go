// Package main owns the router but registers nothing itself: every route lives
// in another package.
package main

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/syst3mctl/godoclive/testdata/gorilla-multipkg/routes"
)

func main() {
	r := mux.NewRouter()
	routes.Register(r)

	// A subrouter handed to a registrar in another package: its prefix has to
	// flow into the routes that registrar adds.
	api := r.PathPrefix("/api/v1").Subrouter()
	routes.RegisterItems(api)

	http.ListenAndServe(":8080", r)
}
