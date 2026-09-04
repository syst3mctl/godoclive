// Package main is a fixture for a house router wrapper over gorilla/mux.
package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// API is the house router.
type API struct {
	r *mux.Router
}

// GET registers a handler for a GET request.
func (a *API) GET(path string, h http.HandlerFunc) {
	a.r.HandleFunc(path, h).Methods("GET")
}

// POST registers a handler for a POST request.
func (a *API) POST(path string, h http.HandlerFunc) {
	a.r.HandleFunc(path, h).Methods("POST")
}

// Widget is a catalog item.
type Widget struct {
	ID   string `json:"id"`
	Name string `json:"name" validate:"required,min=2"`
}

// ListWidgets returns the catalog.
func ListWidgets(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode([]Widget{})
}

// CreateWidget adds a widget to the catalog.
func CreateWidget(w http.ResponseWriter, r *http.Request) {
	var in Widget
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(in)
}

func main() {
	a := &API{r: mux.NewRouter()}
	a.GET("/widgets", ListWidgets)
	a.POST("/widgets", CreateWidget)
	http.ListenAndServe(":8080", a.r)
}
