package server

import (
	"github.com/labstack/echo/v4"
	"github.com/syst3mctl/godoclive/testdata/echo-multipkg/handlers"
)

// bus has an Any method that registers nothing HTTP-related.
type bus struct{}

func (b *bus) Any(topic string, fn func()) {}

// Build is a factory whose signature names no echo type.
func Build() any {
	e := echo.New()
	e.GET("/factory", handlers.Status)
	return e
}

// Server holds its Echo instance in a struct field.
type Server struct {
	Router *echo.Echo
	events *bus
}

func (s *Server) routes() {
	s.Router.GET("/status", handlers.Status)

	// Same method name, unrelated receiver: must not be read as a route.
	s.events.Any("warm", nil)
}
