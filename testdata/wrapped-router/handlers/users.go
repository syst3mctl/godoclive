// Package handlers holds the service's HTTP handlers. It knows nothing about
// how they are registered.
package handlers

import (
	"encoding/json"
	"net/http"
)

// User is an account.
type User struct {
	ID    string `json:"id"`
	Email string `json:"email" validate:"required,email"`
}

// ListUsers returns every user.
func ListUsers(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode([]User{})
}

// GetUser returns one user by id.
func GetUser(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("id") == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(User{})
}

// CreateUser registers an account.
func CreateUser(w http.ResponseWriter, r *http.Request) {
	var in User
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(in)
}
