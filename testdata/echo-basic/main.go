package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Item struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

func loggingMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		return next(c)
	}
}

func authMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		token := c.Request().Header.Get("Authorization")
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}
		return next(c)
	}
}

func main() {
	e := echo.New()
	e.Use(loggingMiddleware)

	// Public routes
	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	e.GET("/users", listUsers)
	e.POST("/users", createUser)
	e.GET("/users/:id", getUser)
	e.DELETE("/users/:id", deleteUser)

	// API group with auth middleware
	api := e.Group("/api/v1")
	api.Use(authMiddleware)

	api.GET("/items", listItems)
	api.GET("/items/:id", getItem)
	api.POST("/items", createItem)

	e.Logger.Fatal(e.Start(":8080"))
}

func listUsers(c echo.Context) error {
	users := []User{}
	return c.JSON(http.StatusOK, users)
}

func createUser(c echo.Context) error {
	var req CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	user := User{ID: 1, Name: req.Name, Email: req.Email}
	return c.JSON(http.StatusCreated, user)
}

func getUser(c echo.Context) error {
	id := c.Param("id")
	user := User{ID: 1, Name: "Alice", Email: "alice@example.com"}
	_ = id
	return c.JSON(http.StatusOK, user)
}

func deleteUser(c echo.Context) error {
	id := c.Param("id")
	_ = id
	return c.NoContent(http.StatusNoContent)
}

func listItems(c echo.Context) error {
	category := c.QueryParam("category")
	_ = category
	items := []Item{}
	return c.JSON(http.StatusOK, items)
}

func getItem(c echo.Context) error {
	id := c.Param("id")
	_ = id
	item := Item{ID: 1, Name: "Widget", Category: "tools"}
	return c.JSON(http.StatusOK, item)
}

func createItem(c echo.Context) error {
	var item Item
	if err := c.Bind(&item); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, item)
}
