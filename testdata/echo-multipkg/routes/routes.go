package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/syst3mctl/godoclive/testdata/echo-multipkg/handlers"
)

// Register registers on the Echo instance it is handed.
func Register(e *echo.Echo) {
	e.GET("/health", handlers.Health)
}

// RegisterItems registers on whichever group it is handed.
func RegisterItems(g *echo.Group) {
	g.GET("/items", handlers.ListItems)
	g.POST("/items", handlers.CreateItem)
	g.GET("/items/:itemID", handlers.GetItem)
}
