// Package main is a service that registers routes on three frameworks at once:
// a chi tree for the v1 API, a gin engine for the v2 API, and a plain stdlib
// ServeMux for the operational endpoints. Splitting a service this way — a
// legacy router kept alive beside a new one, with health and metrics on the
// standard library — is ordinary, and every route below belongs in the docs.
package main

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/chi/v5"
)

// User is a registered account.
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// Product is an item in the catalog.
type Product struct {
	SKU   string  `json:"sku"`
	Price float64 `json:"price"`
}

// ListUsers returns every user in the account.
func ListUsers(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode([]User{})
}

// GetUser returns a single user by id.
func GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(User{})
}

// ListProducts returns the catalog.
func ListProducts(c *gin.Context) {
	c.JSON(http.StatusOK, []Product{})
}

// CreateProduct adds an item to the catalog.
func CreateProduct(c *gin.Context) {
	var p Product
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, p)
}

// Healthz reports process liveness.
func Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func main() {
	r := chi.NewRouter()
	r.Get("/api/v1/users", ListUsers)
	r.Get("/api/v1/users/{id}", GetUser)

	g := gin.New()
	g.GET("/api/v2/products", ListProducts)
	g.POST("/api/v2/products", CreateProduct)

	ops := http.NewServeMux()
	ops.HandleFunc("GET /healthz", Healthz)

	go http.ListenAndServe(":8081", g)
	go http.ListenAndServe(":8082", ops)
	http.ListenAndServe(":8080", r)
}
