// Package httpx provides small JSON response helpers used by the proxy handlers.
package httpx

import (
	"encoding/json"
	"net/http"
)

// WriteJSON writes v as a JSON response with the given status code. The
// concrete type of v is determined at the call site — this helper itself
// receives it as an interface.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
