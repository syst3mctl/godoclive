// Package main keeps no routes of its own: the mux is built here and handed to
// another package, the layout of a real service.
package main

import (
	"net/http"

	"github.com/syst3mctl/godoclive/testdata/stdlib-multipkg/routes"
)

func main() {
	mux := http.NewServeMux()
	routes.Register(mux)
	http.ListenAndServe(":8080", mux)
}
