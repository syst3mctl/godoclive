package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ProfileUpdateRequest is the JSON body for updating a user profile.
type ProfileUpdateRequest struct {
	DisplayName string `json:"display_name" validate:"required"`
	Bio         string `json:"bio"`
}

func main() {
	r := chi.NewRouter()

	// File upload endpoint.
	r.Post("/users/{id}/avatar", UploadAvatar)

	// JSON body endpoint (for contrast).
	r.Put("/users/{id}/profile", UpdateProfile)

	// Endpoint with both multipart form and file.
	r.Post("/documents", UploadDocument)

	http.ListenAndServe(":3000", r)
}

// UploadAvatar handles file upload via multipart form.
func UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")

	r.ParseMultipartForm(32 << 20)
	file, header, err := r.FormFile("avatar")
	if err != nil {
		http.Error(w, "missing avatar file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"user_id":  userID,
		"filename": header.Filename,
	})
}

// UpdateProfile updates a user's profile from JSON body.
func UpdateProfile(w http.ResponseWriter, r *http.Request) {
	_ = chi.URLParam(r, "id")

	var req ProfileUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(req)
}

// UploadDocument handles document upload with metadata.
func UploadDocument(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(64 << 20)

	file, header, err := r.FormFile("document")
	if err != nil {
		http.Error(w, "missing document file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Also read the title from the form.
	title := r.FormValue("title")

	data, _ := io.ReadAll(file)
	_ = data

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"filename": header.Filename,
		"title":    title,
		"size":     fmt.Sprintf("%d", len(data)),
	})
}
