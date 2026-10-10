package explain

import (
	"context"
	"encoding/json"
	"energy/internal/domain"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestBatchOneRequestForAllMeters(t *testing.T) {
	for _, provider := range []string{"claude", "openai"} {
		t.Run(provider, func(t *testing.T) {
			fs := []domain.Finding{}
			in := domain.Input{Timezone: "America/Bogota"}
			for i := 0; i < 12; i++ {
				in.Readings = append(in.Readings, domain.Reading{MeterID: fmt.Sprintf("M-%d", i), Consumption: 10})
			}
			for i := 0; i < 4; i++ {
				f := fixture()
				f.ID = fmt.Sprintf("f%d", i)
				f.MeterID = in.Readings[i].MeterID
				fs = append(fs, f)
			}
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				var body map[string]any
				if json.NewDecoder(r.Body).Decode(&body) != nil {
					t.Error("invalid request")
					return
				}
				var input string
				if provider == "openai" {
					input = body["input"].(string)
					if body["max_output_tokens"].(float64) > 12000 {
						t.Error("output not capped")
					}
				} else {
					input = body["messages"].([]any)[0].(map[string]any)["content"].(string)
				}
				var payload struct {
					Findings []domain.Finding `json:"findings"`
					Meters   []any            `json:"meters"`
				}
				if json.Unmarshal([]byte(input), &payload) != nil || len(payload.Findings) != 4 || len(payload.Meters) != 12 {
					t.Error("not whole plant")
				}
				result := batchResponse{}
				// Deliberately reverse responses: attach by ID, never array position.
				for i := len(fs) - 1; i >= 0; i-- {
					result.Analyses = append(result.Analyses, batchItem{FindingID: fs[i].ID, Narrative: narrativePlan(fs[i]).Narrative})
				}
				raw, _ := json.Marshal(result)
				if provider == "openai" {
					json.NewEncoder(w).Encode(map[string]any{"status": "completed", "output": []any{map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]string{"type": "output_text", "text": string(raw)}}}}})
				} else {
					json.NewEncoder(w).Encode(message("end_turn", []any{map[string]string{"type": "text", "text": string(raw)}}))
				}
			}))
			defer server.Close()
			c := Client{Provider: provider, Key: "test", BaseURL: server.URL}
			out := c.ExplainBatch(context.Background(), fs, in)
			if calls.Load() != 1 || len(out) != 4 {
				t.Fatal("request count", calls.Load())
			}
			for i, e := range out {
				if e.Source != provider || e.NarrativeVersion != PromptVersion || e.Facts[0].Observed != fs[i].Evidence[0].Observed {
					t.Fatal("lost evidence or narrative", e)
				}
			}
		})
	}
}

func TestInvalidBatchNeverRetries(t *testing.T) {
	for _, scenario := range []string{"duplicate", "missing", "unknown", "invalid_narrative", "rate_limit", "malformed"} {
		t.Run(scenario, func(t *testing.T) {
			fs := []domain.Finding{fixture(), fixture()}
			fs[0].ID = "a"
			fs[1].ID = "b"
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if scenario == "rate_limit" {
					w.WriteHeader(429)
					return
				}
				result := batchResponse{Analyses: []batchItem{{FindingID: "a", Narrative: narrativePlan(fs[0]).Narrative}, {FindingID: "b", Narrative: narrativePlan(fs[1]).Narrative}}}
				switch scenario {
				case "duplicate":
					result.Analyses[1].FindingID = "a"
				case "missing":
					result.Analyses = result.Analyses[:1]
				case "unknown":
					result.Analyses[1].FindingID = "other"
				case "invalid_narrative":
					result.Analyses[1].Narrative.Summary.EvidenceIDs = []string{"invented"}
				}
				raw, _ := json.Marshal(result)
				if scenario == "malformed" {
					raw = []byte("{bad")
				}
				json.NewEncoder(w).Encode(message("end_turn", []any{map[string]string{"type": "text", "text": string(raw)}}))
			}))
			defer server.Close()
			c := Client{Provider: "claude", Key: "test", BaseURL: server.URL}
			out := c.ExplainBatch(context.Background(), fs, domain.Input{})
			if calls.Load() != 1 {
				t.Fatal("paid retry", calls.Load())
			}
			for _, e := range out {
				if e.Source != "rules" || e.FallbackReason == "" {
					t.Fatal("invalid attribution")
				}
			}
		})
	}
}
