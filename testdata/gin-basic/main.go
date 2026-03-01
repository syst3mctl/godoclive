package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CreateItemRequest is the request body for creating items.
type CreateItemRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	Price       float64 `json:"price" binding:"required"`
}

// ItemResponse is the response body for item endpoints.
type ItemResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
}

func main() {
	r := gin.Default()
	r.Use(RequestID())

	v1 := r.Group("/api/v1")
	v1.GET("/items", ListItems)
	v1.GET("/items/:id", GetItem)
	v1.POST("/items", CreateItem)
	v1.DELETE("/items/:id", DeleteItem)

	admin := v1.Group("/admin")
	admin.Use(AuthRequired())
	admin.GET("/users", ListUsers)

	r.Run(":8080")
}

// RequestID is middleware that adds a request ID header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Request-ID", "req-123")
		c.Next()
	}
}

// AuthRequired is middleware that checks for an API key.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing api key"})
			return
		}
		c.Next()
	}
}

// ListItems returns a list of items with optional search.
func ListItems(c *gin.Context) {
	search := c.Query("search")
	limit := c.DefaultQuery("limit", "20")
	_ = search
	_ = limit
	c.JSON(http.StatusOK, []ItemResponse{
		{ID: "1", Name: "Widget", Price: 9.99},
	})
}

// GetItem returns a single item by ID.
func GetItem(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, ItemResponse{ID: id, Name: "Widget", Price: 9.99})
}

// CreateItem creates a new item from JSON body.
func CreateItem(c *gin.Context) {
	var req CreateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ItemResponse{
		ID:          "new-id",
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
	})
}

// DeleteItem removes an item by ID.
func DeleteItem(c *gin.Context) {
	_ = c.Param("id")
	c.Status(http.StatusNoContent)
}

// ListUsers returns all users (admin only).
func ListUsers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"users": []string{"alice", "bob"}})
}
