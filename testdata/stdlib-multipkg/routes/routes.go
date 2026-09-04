package routes

import (
	"net/http"

	"github.com/syst3mctl/godoclive/testdata/stdlib-multipkg/handlers"
)

// Register registers on the mux it is handed, and fans out to a per-resource
// registrar.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", handlers.Health)
	registerItems(mux)
}

func registerItems(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/items", handlers.ListItems)
	mux.HandleFunc("POST /api/v1/items", handlers.CreateItem)
	mux.HandleFunc("GET /api/v1/items/{itemID}", handlers.GetItem)
}
