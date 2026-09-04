// Package main wires the service together. The wrapper, the handlers and the
// wiring each live in their own package, which is the arrangement that makes a
// house router hard to analyze: the registration call names neither a router
// type nor a literal path, and the handler it is handed type-checks only here.
package main

import (
	"net/http"

	"github.com/syst3mctl/godoclive/testdata/wrapped-router/handlers"
	"github.com/syst3mctl/godoclive/testdata/wrapped-router/httpx"
)

func main() {
	r := httpx.New()
	r.Handle("GET /users", handlers.ListUsers)
	r.Handle("GET /users/{id}", handlers.GetUser)
	r.Handle("POST /users", handlers.CreateUser)
	http.ListenAndServe(":8080", r.Mux())
}
