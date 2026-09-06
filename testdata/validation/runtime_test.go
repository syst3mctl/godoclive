package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"
)

// The parent OpenAPI integration test validates these actual handler responses.
func TestCaptureResponses(t *testing.T) {
	path := os.Getenv("GODOCLIVE_RESPONSE_CAPTURE")
	if path == "" {
		t.Skip("run by the OpenAPI integration test")
	}
	bodies := make(map[string]json.RawMessage)
	for _, tc := range []struct{ name, url string }{{"summary", "/articles/1?format=summary"}, {"full", "/articles/1"}} {
		rr := httptest.NewRecorder()
		GetArticle(rr, httptest.NewRequest("GET", tc.url, nil))
		if rr.Code != 200 {
			t.Fatalf("%s status = %d", tc.name, rr.Code)
		}
		bodies[tc.name] = json.RawMessage(rr.Body.Bytes())
	}
	data, err := json.Marshal(bodies)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
