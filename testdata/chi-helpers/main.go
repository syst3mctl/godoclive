package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// UserResponse is a user resource.
type UserResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ErrorResponse is an error response body.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

func main() {
	r := chi.NewRouter()
	r.Get("/users/{id}", GetUser)
	r.Get("/users", ListUsers)
	r.Delete("/users/{id}", DeleteUser)
	r.Get("/health", HealthCheck)
	http.ListenAndServe(":3000", r)
}

// GetUser uses the respond helper for the success case and sendError for errors.
// This tests mixed direct + helper responses.
func GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		sendError(w, "missing user id", http.StatusBadRequest)
		return
	}

	user := UserResponse{ID: id, Name: "Alice"}
	respond(w, user, http.StatusOK)
}

// ListUsers uses the writeJSON helper (always 200).
func ListUsers(w http.ResponseWriter, r *http.Request) {
	users := []UserResponse{{ID: "1", Name: "Alice"}, {ID: "2", Name: "Bob"}}
	writeJSON(w, users)
}

// DeleteUser uses direct w.WriteHeader (no body).
func DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		sendError(w, "missing user id", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HealthCheck uses json.Encode with no preceding WriteHeader → implicit 200.
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// respond writes a JSON response with the given status code.
// This is the common respond-wrapper pattern.
func respond(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeJSON writes a JSON response with status 200.
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

// sendError writes an error JSON response.
func sendError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Error: msg, Code: code})
}
