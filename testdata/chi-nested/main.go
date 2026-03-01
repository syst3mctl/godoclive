package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Nested Route: /api/v1/users/...
	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/users", func(r chi.Router) {
			r.Get("/", ListUsers)
			r.Post("/", CreateUser)

			r.Route("/{userID}", func(r chi.Router) {
				r.Get("/", GetUser)
				r.Put("/", UpdateUser)
			})
		})

		// Group with middleware — no path prefix
		r.Group(func(r chi.Router) {
			r.Use(AdminOnly)
			r.Get("/stats", GetStats)
			r.Delete("/cache", ClearCache)
		})
	})

	// Mount a sub-router at /admin
	r.Mount("/admin", adminRouter())

	http.ListenAndServe(":3000", r)
}

func adminRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(AdminOnly)
	r.Get("/dashboard", AdminDashboard)
	r.Post("/settings", UpdateSettings)
	return r
}

// AdminOnly is middleware that checks for admin privileges.
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := r.Header.Get("X-Role")
		if role != "admin" {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func ListUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]string{"alice", "bob"})
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "created"})
}

func GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "userID")
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "userID")
	json.NewEncoder(w).Encode(map[string]string{"id": id, "status": "updated"})
}

func GetStats(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]int{"users": 42})
}

func ClearCache(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func AdminDashboard(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"page": "dashboard"})
}

func UpdateSettings(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}
