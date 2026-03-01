package main

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// UserResponse is the response body for user endpoints.
type UserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// CreateUserRequest is the request body for creating users.
type CreateUserRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required"`
}

// ErrorResponse is an error response body.
type ErrorResponse struct {
	Error string `json:"error"`
}

func main() {
	r := gin.Default()

	// Public endpoints — no auth.
	r.GET("/health", HealthCheck)

	// API v1 group.
	v1 := r.Group("/api/v1")

	// Users group — JWT auth.
	users := v1.Group("/users")
	users.Use(JWTAuth())
	users.GET("", ListUsers)
	users.GET("/:id", GetUser)
	users.POST("", CreateUser)

	// Admin group — API key auth, nested under v1.
	admin := v1.Group("/admin")
	admin.Use(APIKeyAuth())
	admin.GET("/stats", GetStats)
	admin.DELETE("/cache", ClearCache)

	r.Run(":8080")
}

// JWTAuth validates a JWT bearer token.
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: "missing bearer token"})
			return
		}
		c.Next()
	}
}

// APIKeyAuth validates an API key header.
func APIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: "missing api key"})
			return
		}
		c.Next()
	}
}

// HealthCheck returns service status.
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ListUsers returns all users with optional search.
func ListUsers(c *gin.Context) {
	search := c.Query("search")
	_ = search
	c.JSON(http.StatusOK, []UserResponse{
		{ID: "1", Name: "Alice", Email: "alice@example.com"},
	})
}

// GetUser returns a single user by ID.
func GetUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "missing id"})
		return
	}
	c.JSON(http.StatusOK, UserResponse{ID: id, Name: "Alice", Email: "alice@example.com"})
}

// CreateUser creates a new user.
func CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, UserResponse{
		ID:    "new-id",
		Name:  req.Name,
		Email: req.Email,
	})
}

// GetStats returns admin statistics.
func GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"total_users": 42})
}

// ClearCache clears the application cache.
func ClearCache(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
