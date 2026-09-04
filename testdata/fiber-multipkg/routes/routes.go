package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/syst3mctl/godoclive/testdata/fiber-multipkg/handlers"
)

// Register registers on the App it is handed.
func Register(app *fiber.App) {
	app.Get("/health", handlers.Health)
}

// RegisterItems registers on whichever router it is handed. App.Group returns
// the fiber.Router interface, so that is the parameter type.
func RegisterItems(r fiber.Router) {
	r.Get("/items", handlers.ListItems)
	r.Post("/items", handlers.CreateItem)
	r.Get("/items/:itemID", handlers.GetItem)
}
