// Package main owns the App but registers nothing itself: every route lives in
// another package.
package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/syst3mctl/godoclive/testdata/fiber-multipkg/routes"
)

func main() {
	app := fiber.New()
	routes.Register(app)

	// A group handed to a registrar in another package: its prefix has to flow
	// into the routes that registrar adds.
	v1 := app.Group("/api/v1")
	routes.RegisterItems(v1)

	app.Listen(":8080")
}
