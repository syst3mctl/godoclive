package server

import (
	"github.com/go-chi/chi/v5"
	"github.com/syst3mctl/godoclive/testdata/chi-multipkg/handlers"
)

// cache has a Get method that is not a route registration.
type cache struct{}

func (c *cache) Get(key, def string) {}

// Server holds its router in a struct field.
type Server struct {
	Router chi.Router
	cache  *cache
}

func (s *Server) routes() {
	s.Router.Get("/status", handlers.Status)

	// Same method name, unrelated receiver: must not be read as a route.
	s.cache.Get("warm", "no")
}

// API embeds chi.Router, so route methods are promoted onto *API.
type API struct {
	chi.Router
}

func NewAPI() *API {
	a := &API{Router: chi.NewRouter()}
	a.Get("/embedded", handlers.Status)
	return a
}
