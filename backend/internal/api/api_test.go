package api

import (
	"encoding/json"
	"energy/internal/store"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOriginAndUnauthenticatedAccess(t *testing.T) {
	h := New(&store.Store{}, false)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "http://localhost/api/v1/auth/login", strings.NewReader(`{}`))
	r.Header.Set("Origin", "https://evil.example")
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/api/v1/meters", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}
func TestDecodeRejectsUnknownAndTrailingJSON(t *testing.T) {
	for _, s := range []string{`{"bad":true}`, `{"note":"hello"} {}`} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(s))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		var b struct {
			Note string `json:"note"`
		}
		if decode(w, r, &b) || w.Code != 400 {
			t.Fatal("invalid body accepted")
		}
	}
}
func TestOpenAPIContract(t *testing.T) {
	w := httptest.NewRecorder()
	a := API{}
	a.openapi(w, httptest.NewRequest("GET", "/api/openapi.json", nil))
	var doc struct {
		OpenAPI string         `json:"openapi"`
		Paths   map[string]any `json:"paths"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &doc); e != nil {
		t.Fatal(e)
	}
	if doc.OpenAPI != "3.0.3" || len(doc.Paths) != 17 {
		t.Fatal(doc.OpenAPI, len(doc.Paths))
	}
}
func TestInvalidPagination(t *testing.T) {
	for _, q := range []string{"?limit=-1", "?limit=1001", "?offset=-1", "?limit=foo"} {
		if _, _, err := page(httptest.NewRequest("GET", "/meters"+q, nil)); err == nil {
			t.Fatal(q)
		}
	}
}
