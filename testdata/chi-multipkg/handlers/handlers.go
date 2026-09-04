package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Payment is a single payment record.
type Payment struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
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

// Dashboard returns admin dashboard data.
func Dashboard(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]int{"users": 0})
}

// ListPayments returns all payments.
func ListPayments(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode([]Payment{})
}

// CreatePayment creates a payment.
func CreatePayment(w http.ResponseWriter, r *http.Request) {
	var p Payment
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid body"})
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

// GetPayment returns one payment.
func GetPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "paymentID")
	if id == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Payment{ID: id})
}
