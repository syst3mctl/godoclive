package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte("super-secret-key")

// --- Responses ---

type StatusResponse struct {
	Status string `json:"status"`
}

type UserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// --- Middleware ---

// JWTAuth validates a JWT bearer token from the Authorization header.
func JWTAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"missing authorization"}`, http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"error":"invalid auth scheme"}`, http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// APIKeyAuth validates an API key from the X-API-Key header.
func APIKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if key == "" {
			http.Error(w, `{"error":"missing api key"}`, http.StatusUnauthorized)
			return
		}
		if key != "valid-api-key-123" {
			http.Error(w, `{"error":"invalid api key"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// BasicAuth validates HTTP Basic authentication credentials.
func BasicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		validUser := subtle.ConstantTimeCompare([]byte(user), []byte("admin")) == 1
		validPass := subtle.ConstantTimeCompare([]byte(pass), []byte("secret")) == 1
		if !validUser || !validPass {
			http.Error(w, `{"error":"invalid credentials"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Handlers ---

// HealthCheck returns the service health status.
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(StatusResponse{Status: "ok"})
}

// ListUsers returns all users.
func ListUsers(w http.ResponseWriter, r *http.Request) {
	users := []UserResponse{
		{ID: "1", Name: "Alice", Email: "alice@example.com"},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(users)
}

// GetUser returns a single user by ID.
func GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(UserResponse{ID: id, Name: "Alice", Email: "alice@example.com"})
}

// GetWebhooks returns the list of configured webhooks.
func GetWebhooks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode([]string{"https://example.com/hook1"})
}

// CreateWebhook creates a new webhook.
func CreateWebhook(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": "webhook-1"})
}

// AdminStats returns admin statistics.
func AdminStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]int{"total_users": 42})
}

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	// Public route — no auth.
	r.Get("/health", HealthCheck)

	// JWT bearer auth on most routes.
	r.Group(func(r chi.Router) {
		r.Use(JWTAuth)
		r.Get("/users", ListUsers)
		r.Get("/users/{id}", GetUser)
	})

	// API key auth on webhooks group.
	r.Route("/webhooks", func(r chi.Router) {
		r.Use(APIKeyAuth)
		r.Get("/", GetWebhooks)
		r.Post("/", CreateWebhook)
	})

	// Basic auth on admin routes.
	r.Group(func(r chi.Router) {
		r.Use(BasicAuth)
		r.Get("/admin/stats", AdminStats)
	})

	http.ListenAndServe(":3000", r)
}
