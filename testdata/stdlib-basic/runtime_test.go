package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"
)

func TestCaptureResponses(t *testing.T) {
	path := os.Getenv("GODOCLIVE_RESPONSE_CAPTURE")
	if path == "" {
		t.Skip("run by the OpenAPI integration test")
	}
	rr := httptest.NewRecorder()
	ListUsers(rr, httptest.NewRequest("GET", "/users?page=1", nil))
	if rr.Code != 200 {
		t.Fatalf("status = %d", rr.Code)
	}
	data, err := json.Marshal(map[string]json.RawMessage{"users": rr.Body.Bytes()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
