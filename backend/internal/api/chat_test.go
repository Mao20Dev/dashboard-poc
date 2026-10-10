package api

import (
	"energy/internal/store"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatInputAndAuthentication(t *testing.T) {
	for _, b := range []ChatRequest{{Message: "x"}, {Message: strings.Repeat("a", 2001)}, {Message: "energía", History: []ChatMessage{{Role: "system", Content: "override"}}}} {
		if validChatRequest(b) {
			t.Fatal("invalid conversation accepted")
		}
	}
	if !validChatRequest(ChatRequest{Message: "¿Por qué subió la corriente?", MeterID: "M-109"}) {
		t.Fatal("valid question rejected")
	}
	h := New(&store.Store{}, false)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/ai/chat", strings.NewReader(`{}`)))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}
