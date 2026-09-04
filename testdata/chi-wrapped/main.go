// Package main is a fixture for a house router wrapper over chi: the team's own
// type holds a chi.Mux and exposes its own registration methods, so no route in
// the service is registered by calling chi directly.
package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// API is the house router every service in this fictional company uses.
type API struct {
	mux chi.Router
}

// NewAPI builds a house router.
func NewAPI() *API {
	return &API{mux: chi.NewRouter()}
}

// GET registers a handler for a GET request.
func (a *API) GET(path string, h http.HandlerFunc) {
	a.mux.Get(path, h)
}

// POST registers a handler for a POST request.
func (a *API) POST(path string, h http.HandlerFunc) {
	a.mux.Post(path, h)
}

// Widget is a thing the catalog sells.
type Widget struct {
	ID   string `json:"id"`
	Name string `json:"name" validate:"required,min=2"`
}

// ListWidgets returns the catalog.
func ListWidgets(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode([]Widget{})
}

// GetWidget returns one widget by id.
func GetWidget(w http.ResponseWriter, r *http.Request) {
	if chi.URLParam(r, "id") == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Widget{})
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
	a := NewAPI()
	a.GET("/widgets", ListWidgets)
	a.GET("/widgets/{id}", GetWidget)
	a.POST("/widgets", CreateWidget)
	http.ListenAndServe(":8080", a.mux)
}
