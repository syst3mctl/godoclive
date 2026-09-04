package handlers

import (
	"encoding/json"
	"net/http"
)

// Item is a single item record.
type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Health reports service health.
func Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Status reports service status.
func Status(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ListItems returns all items.
func ListItems(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode([]Item{})
}

// CreateItem creates an item.
func CreateItem(w http.ResponseWriter, r *http.Request) {
	var it Item
	if err := json.NewDecoder(r.Body).Decode(&it); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid body"})
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(it)
}

// GetItem returns one item.
func GetItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("itemID")
	if id == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Item{ID: id})
}
