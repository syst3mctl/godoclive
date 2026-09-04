package server

import (
	"net/http"

	"github.com/syst3mctl/godoclive/testdata/stdlib-multipkg/handlers"
)

// bus has a Handle method that registers nothing HTTP-related.
type bus struct{}

func (b *bus) Handle(topic string, fn func()) {}

// Build is a factory whose signature names no mux type.
func Build() any {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /factory", handlers.Status)
	return mux
}

// Server holds its mux in a struct field.
type Server struct {
	Router *http.ServeMux
	events *bus
}

func (s *Server) routes() {
	s.Router.HandleFunc("GET /status", handlers.Status)

	// Same method name, unrelated receiver: must not be read as a route.
	s.events.Handle("warm", nil)
}
