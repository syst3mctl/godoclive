package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ItemResponse is an item resource.
type ItemResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ErrorBody is an error response body.
type ErrorBody struct {
	Message string `json:"message"`
}

func main() {
	r := gin.Default()
	r.GET("/items/:id", GetItem)
	r.GET("/items", ListItems)
	r.DELETE("/items/:id", DeleteItem)
	r.Run(":8080")
}

// GetItem returns a single item or 404.
func GetItem(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		respondError(c, "missing id", http.StatusBadRequest)
		return
	}

	item := ItemResponse{ID: id, Name: "Widget"}
	respondOK(c, item)
}

// ListItems returns all items.
func ListItems(c *gin.Context) {
	items := []ItemResponse{{ID: "1", Name: "Widget"}, {ID: "2", Name: "Gadget"}}
	respondOK(c, items)
}

// DeleteItem removes an item.
func DeleteItem(c *gin.Context) {
	_ = c.Param("id")
	c.Status(http.StatusNoContent)
}

// respondOK writes a JSON response with status 200.
func respondOK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, data)
}

// respondError writes a JSON error response.
func respondError(c *gin.Context, msg string, code int) {
	c.JSON(code, ErrorBody{Message: msg})
}
