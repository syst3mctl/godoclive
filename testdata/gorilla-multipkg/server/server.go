package server

import (
	"github.com/gorilla/mux"
	"github.com/syst3mctl/godoclive/testdata/gorilla-multipkg/handlers"
)

// bus has a Handle method that registers nothing HTTP-related.
type bus struct{}

func (b *bus) Handle(topic string, fn func()) {}

// Build is a factory whose signature names no mux type.
func Build() any {
	r := mux.NewRouter()
	r.HandleFunc("/factory", handlers.Status).Methods("GET")
	return r
}

// Server holds its router in a struct field.
type Server struct {
	Router *mux.Router
	events *bus
}

func (s *Server) routes() {
	s.Router.HandleFunc("/status", handlers.Status).Methods("GET")

	// Same method name, unrelated receiver: must not be read as a route.
	s.events.Handle("warm", nil)
}
