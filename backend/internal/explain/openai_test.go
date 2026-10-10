package explain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIResponsesContractAndEvidence(t *testing.T) {
	f := fixture()
	plan, _ := json.Marshal(narrativePlan(f))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("wrong OpenAI endpoint or credentials")
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Fatal("bad JSON")
		}
		if body["model"] != DefaultOpenAIModel || body["store"] != false {
			t.Error("model or storage policy")
		}
		format := body["text"].(map[string]any)["format"].(map[string]any)
		if format["strict"] != true || format["type"] != "json_schema" {
			t.Error("missing strict structured output")
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "completed", "output": []any{map[string]any{"type": "reasoning"}, map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]string{"type": "output_text", "text": string(plan)}}}}})
	}))
	defer server.Close()
	c := Client{Provider: "openai", Key: "test-key", BaseURL: server.URL}
	e := c.Explain(context.Background(), f)
	if e.Source != "openai" || e.Model != DefaultOpenAIModel || e.FallbackReason != "" || e.Facts[0].Observed != 110.5 {
		t.Fatalf("unexpected explanation: %+v", e)
	}
}
func TestOpenAIFailuresNeverInventEvidence(t *testing.T) {
	valid, _ := json.Marshal(narrativePlan(fixture()))
	for _, tc := range []struct {
		name, status, kind, plan string
		httpStatus               int
	}{
		{"incomplete", "incomplete", "output_text", string(valid), 200},
		{"refusal", "completed", "refusal", "", 200},
		{"bad evidence", "completed", "output_text", strings.Replace(string(valid), "e1", "invented", 1), 200},
		{"extra prose", "completed", "output_text", string(valid) + " {}", 200},
		{"unauthorized", "", "", "private error", 401},
		{"rate limited", "", "", "private error", 429},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.httpStatus)
				json.NewEncoder(w).Encode(map[string]any{"status": tc.status, "output": []any{map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]string{"type": tc.kind, "text": tc.plan}}}}})
			}))
			defer server.Close()
			c := Client{Provider: "openai", Key: "test", BaseURL: server.URL}
			e := c.Explain(context.Background(), fixture())
			if e.Source != "rules" || e.FallbackReason == "" || e.Facts[0].Observed != 110.5 {
				t.Fatal("unsafe fallback", e)
			}
		})
	}
}
