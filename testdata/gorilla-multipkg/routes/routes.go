package routes

import (
	"github.com/gorilla/mux"
	"github.com/syst3mctl/godoclive/testdata/gorilla-multipkg/handlers"
)

// Register registers on the router it is handed.
func Register(r *mux.Router) {
	r.HandleFunc("/health", handlers.Health).Methods("GET")
}

// RegisterItems registers on whichever router it is handed — the root or a
// subrouter carrying a prefix.
func RegisterItems(r *mux.Router) {
	r.HandleFunc("/items", handlers.ListItems).Methods("GET")
	r.HandleFunc("/items", handlers.CreateItem).Methods("POST")
	r.HandleFunc("/items/{itemID}", handlers.GetItem).Methods("GET")
}
