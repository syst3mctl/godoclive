// Package main owns the Echo instance but registers nothing itself: every route
// lives in another package.
package main

import (
	"github.com/labstack/echo/v4"
	"github.com/syst3mctl/godoclive/testdata/echo-multipkg/routes"
)

func main() {
	e := echo.New()
	routes.Register(e)

	// A group handed to a registrar in another package: its prefix has to flow
	// into the routes that registrar adds.
	v1 := e.Group("/api/v1")
	routes.RegisterItems(v1)

	e.Start(":8080")
}
