// Package main is a fixture for a house router wrapper over fiber.
package main

import (
	"github.com/gofiber/fiber/v2"
)

// API is the house router.
type API struct {
	app *fiber.App
}

// GET registers a handler for a GET request.
func (a *API) GET(path string, h fiber.Handler) {
	a.app.Get(path, h)
}

// POST registers a handler for a POST request.
func (a *API) POST(path string, h fiber.Handler) {
	a.app.Post(path, h)
}

// Widget is a catalog item.
type Widget struct {
	ID   string `json:"id"`
	Name string `json:"name" validate:"required,min=2"`
}

// ListWidgets returns the catalog.
func ListWidgets(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON([]Widget{})
}

// CreateWidget adds a widget to the catalog.
func CreateWidget(c *fiber.Ctx) error {
	var in Widget
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(in)
}

func main() {
	a := &API{app: fiber.New()}
	a.GET("/widgets", ListWidgets)
	a.POST("/widgets", CreateWidget)
	a.app.Listen(":8080")
}
