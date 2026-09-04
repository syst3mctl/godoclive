// Package main is a fixture for a house router wrapper over gin.
package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// API is the house router.
type API struct {
	r *gin.Engine
}

// GET registers a handler for a GET request.
func (a *API) GET(path string, h gin.HandlerFunc) {
	a.r.GET(path, h)
}

// POST registers a handler for a POST request.
func (a *API) POST(path string, h gin.HandlerFunc) {
	a.r.POST(path, h)
}

// Widget is a catalog item.
type Widget struct {
	ID   string `json:"id"`
	Name string `json:"name" binding:"required,min=2"`
}

// ListWidgets returns the catalog.
func ListWidgets(c *gin.Context) {
	c.JSON(http.StatusOK, []Widget{})
}

// CreateWidget adds a widget to the catalog.
func CreateWidget(c *gin.Context) {
	var in Widget
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, in)
}

func main() {
	a := &API{r: gin.New()}
	a.GET("/widgets", ListWidgets)
	a.POST("/widgets", CreateWidget)
	a.r.Run(":8080")
}
