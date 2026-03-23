package model_test

import (
	"testing"

	"github.com/dicki/godoclive/internal/model"
)

func TestInferSummary(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"GetUserByID", "Get User By ID"},
		{"CreateUser", "Create User"},
		{"ListProducts", "List Products"},
		{"DeleteOrderItem", "Delete Order Item"},
		{"UploadAvatar", "Upload Avatar"},
		{"RefreshToken", "Refresh Token"},
		{"HealthCheck", "Health Check"},
		{"HandleGetUser", "Get User"},
		{"HTTPGetUser", "Get User"},
		{"APICreateOrder", "Create Order"},
		{"DoSomething", "Something"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := model.InferSummary(tt.input)
			if got != tt.expected {
				t.Errorf("InferSummary(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestInferTag(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"CreateUser", "users"},
		{"GetUserByID", "users"},
		{"ListProducts", "products"},
		{"DeleteOrderItem", "orders"},
		{"UploadAvatar", "avatars"},
		{"RefreshToken", "tokens"},
		{"HealthCheck", "health"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := model.InferTag(tt.input)
			if got != tt.expected {
				t.Errorf("InferTag(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
