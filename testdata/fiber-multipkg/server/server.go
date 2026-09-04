package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/syst3mctl/godoclive/testdata/fiber-multipkg/handlers"
)

// bus has an All method that registers nothing HTTP-related.
type bus struct{}

func (b *bus) All(topic string, fn func()) {}

// Build is a factory whose signature names no fiber type.
func Build() any {
	app := fiber.New()
	app.Get("/factory", handlers.Status)
	return app
}

// Server holds its App in a struct field.
type Server struct {
	Router *fiber.App
	events *bus
}

func (s *Server) routes() {
	s.Router.Get("/status", handlers.Status)

	// Same method name, unrelated receiver: must not be read as a route.
	s.events.All("warm", nil)
}
