// Package main is a fixture for a house router wrapper over echo.
package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// API is the house router.
type API struct {
	e *echo.Echo
}

// GET registers a handler for a GET request.
func (a *API) GET(path string, h echo.HandlerFunc) {
	a.e.GET(path, h)
}

// POST registers a handler for a POST request.
func (a *API) POST(path string, h echo.HandlerFunc) {
	a.e.POST(path, h)
}

// Widget is a catalog item.
type Widget struct {
	ID   string `json:"id"`
	Name string `json:"name" validate:"required,min=2"`
}

// ListWidgets returns the catalog.
func ListWidgets(c echo.Context) error {
	return c.JSON(http.StatusOK, []Widget{})
}

// CreateWidget adds a widget to the catalog.
func CreateWidget(c echo.Context) error {
	var in Widget
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, in)
}

func main() {
	a := &API{e: echo.New()}
	a.GET("/widgets", ListWidgets)
	a.POST("/widgets", CreateWidget)
	a.e.Start(":8080")
}
