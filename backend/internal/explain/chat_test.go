package explain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatSourcesAndTokenCap(t *testing.T) {
	for _, source := range []string{"meter:M-109", "invented"} {
		t.Run(source, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if json.NewDecoder(r.Body).Decode(&body) != nil {
					t.Fatal("invalid payload")
				}
				if body["max_tokens"] != float64(1000) {
					t.Error("unbounded response")
				}
				raw, _ := json.Marshal(ChatAnswer{Answer: "La corriente aumentó respecto a su referencia.", Sources: []string{source}})
				json.NewEncoder(w).Encode(map[string]any{"type": "message", "role": "assistant", "stop_reason": "end_turn", "content": []any{map[string]string{"type": "text", "text": string(raw)}}})
			}))
			defer server.Close()
			c := &Client{Provider: "claude", Key: "test", BaseURL: server.URL}
			answer, err := c.Chat(context.Background(), []byte(`{"question":"corriente"}`), map[string]bool{"meter:M-109": true})
			if source == "invented" {
				if err == nil {
					t.Fatal("invented source accepted")
				}
			} else if err != nil || answer.Answer == "" {
				t.Fatal(err)
			}
		})
	}
}
