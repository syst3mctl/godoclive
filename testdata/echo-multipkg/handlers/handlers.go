package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Item is a single item record.
type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Health reports service health.
func Health(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// Status reports service status.
func Status(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ListItems returns all items.
func ListItems(c echo.Context) error {
	return c.JSON(http.StatusOK, []Item{})
}

// CreateItem creates an item.
func CreateItem(c echo.Context) error {
	var it Item
	if err := c.Bind(&it); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	return c.JSON(http.StatusCreated, it)
}

// GetItem returns one item.
func GetItem(c echo.Context) error {
	id := c.Param("itemID")
	if id == "" {
		return c.NoContent(http.StatusNotFound)
	}
	return c.JSON(http.StatusOK, Item{ID: id})
}
