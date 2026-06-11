// Package main mirrors a real-world net/http gateway where most routes are
// mounted conditionally — inside if/else/switch blocks gated on which backing
// dependency is wired. The extractor must descend into those blocks rather than
// only reading the top-level statements of NewRouter.
package main

import (
	"encoding/json"
	"net/http"
)

// Config gates which route groups are mounted, the way a production server only
// wires a route when its dependency is present.
type Config struct {
	Sessions bool
	Users    bool
	Mode     string
}

// NewRouter builds the mux. Only /v1/auth/login and the health probes are
// registered as top-level statements; everything else lives inside a branch.
func NewRouter(cfg Config) http.Handler {
	mux := http.NewServeMux()

	// Always mounted (top-level statement).
	mux.HandleFunc("GET /v1/auth/login", login)

	if cfg.Sessions {
		mux.HandleFunc("POST /v1/sessions", startSession)
		mux.HandleFunc("GET /v1/sessions/{id}", getSession)

		// Nested condition — must still be discovered.
		if cfg.Users {
			mux.HandleFunc("GET /v1/sessions/{id}/receipt", getReceipt)
		}
	}

	if cfg.Users {
		mux.HandleFunc("GET /v1/users/me", me)
	} else {
		mux.HandleFunc("GET /v1/users/{id}", getUser)
	}

	switch cfg.Mode {
	case "admin":
		mux.HandleFunc("DELETE /v1/users/{id}", deleteUser)
	default:
		mux.HandleFunc("GET /v1/status", status)
	}

	//godoclive:ignore
	mux.HandleFunc("GET /metrics", metrics)
	// /livez sits directly below the ignored /metrics: it must NOT inherit the
	// directive (regression guard for the single-statement ignore window).
	mux.HandleFunc("GET /livez", health)

	return mux
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func login(w http.ResponseWriter, r *http.Request)        { writeJSON(w, http.StatusOK, "ok") }
func startSession(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusAccepted, "ok") }
func getSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, r.PathValue("id"))
}
func getReceipt(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, r.PathValue("id"))
}
func me(w http.ResponseWriter, r *http.Request)      { writeJSON(w, http.StatusOK, "me") }
func getUser(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, r.PathValue("id")) }
func deleteUser(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
func status(w http.ResponseWriter, r *http.Request)  { writeJSON(w, http.StatusOK, "ready") }
func metrics(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, "metrics") }
func health(w http.ResponseWriter, r *http.Request)  { writeJSON(w, http.StatusOK, "live") }

func main() {
	_ = NewRouter(Config{Sessions: true, Users: true, Mode: "admin"})
}
