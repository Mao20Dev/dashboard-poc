package explain

import (
	"context"
	"encoding/json"
	"energy/internal/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fixture() domain.Finding {
	return domain.Finding{Type: "REAL_ANOMALY", Evidence: []domain.Evidence{{ID: "e1", Metric: "consumption_kwh", Unit: "kWh", Observed: 110.5}, {ID: "e2", Metric: "current_a", Unit: "A", Observed: 220}}}
}
func TestRejectUnsupportedEvidenceAndClaims(t *testing.T) {
	f := fixture()
	for _, mutate := range []func(*Plan){func(p *Plan) { p.Focus = "FALSE_POSITIVE" }, func(p *Plan) { p.EvidenceIDs = []string{"e1"} }, func(p *Plan) { p.EvidenceIDs = []string{"e1", "e1"} }, func(p *Plan) { p.HypothesisCodes = []string{"confirmed_fire"} }, func(p *Plan) { p.ActionCodes = []string{"disconnect_plant"} }} {
		p := defaultPlan(f)
		mutate(&p)
		if Validate(f, p) == nil {
			t.Fatal("accepted unsupported plan")
		}
	}
}
func TestProviderCannotChangeNumbers(t *testing.T) {
	f := fixture()
	p := narrativePlan(f)
	raw, _ := json.Marshal(p)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" || r.URL.RawQuery != "" {
			t.Error("incorrect Anthropic endpoint")
		}
		if r.Header.Get("x-api-key") != "test" || r.Header.Get("anthropic-version") != "2023-06-01" || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("anthropic-workspace-id") != "workspace-test" {
			t.Error("incorrect Anthropic headers")
		}
		var request struct {
			Model        string                           `json:"model"`
			MaxTokens    int                              `json:"max_tokens"`
			System       string                           `json:"system"`
			Messages     []struct{ Role, Content string } `json:"messages"`
			OutputConfig struct {
				Effort string `json:"effort"`
				Format struct {
					Type   string `json:"type"`
					Schema struct {
						Type                 string   `json:"type"`
						AdditionalProperties *bool    `json:"additionalProperties"`
						Required             []string `json:"required"`
					} `json:"schema"`
				} `json:"format"`
			} `json:"output_config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Model != DefaultModel || request.MaxTokens != 4096 || request.System == "" || len(request.Messages) != 1 || request.Messages[0].Role != "user" || !strings.Contains(request.Messages[0].Content, "finding") {
			t.Error("missing Anthropic request contract or investigation context")
		}
		format := request.OutputConfig.Format
		if request.OutputConfig.Effort != "low" || format.Type != "json_schema" || format.Schema.Type != "object" || format.Schema.AdditionalProperties == nil || *format.Schema.AdditionalProperties || len(format.Schema.Required) != 1 {
			t.Error("structured output schema is not enforced")
		}
		_ = json.NewEncoder(w).Encode(message("end_turn", []any{
			map[string]string{"type": "thinking", "thinking": "Private reasoning is not evidence."},
			map[string]string{"type": "text", "text": string(raw)},
		}))
	}))
	defer server.Close()
	c := Client{Key: "test", WorkspaceID: "workspace-test", BaseURL: server.URL + "/v1/"}
	e := c.Explain(context.Background(), f)
	if e.Source != "claude" || e.Model != DefaultModel || e.FallbackReason != "" || len(e.Facts) != 2 || e.Facts[0].Observed != 110.5 || e.Facts[0].Unit != "kWh" {
		t.Fatal(e)
	}
}

func message(stop string, content []any) map[string]any {
	return map[string]any{"type": "message", "role": "assistant", "stop_reason": stop, "content": content}
}

func TestFallbackOnProviderFailure(t *testing.T) {
	for status, reason := range map[int]string{
		401: "provider_authentication_failed", 403: "provider_authentication_failed",
		429: "provider_rate_limited", 503: "provider_unavailable", 529: "provider_unavailable",
		400: "provider_configuration_error", 404: "provider_configuration_error",
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"error":{"message":"sensitive provider diagnostic"}}`))
			}))
			defer server.Close()
			c := Client{Key: "test", Model: "test", BaseURL: server.URL}
			e := c.Explain(context.Background(), fixture())
			if e.Source != "rules" || len(e.Actions) == 0 || e.FallbackReason != reason {
				t.Fatal(e)
			}
			encoded, _ := json.Marshal(e)
			if strings.Contains(string(encoded), "sensitive provider diagnostic") {
				t.Fatal("provider body leaked")
			}
		})
	}
}
func TestUnknownFreeTextRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(message("end_turn", []any{map[string]string{"type": "text", "text": `{"focus":"REAL_ANOMALY","reason":"consumption is 1105 kWh"}`}}))
	}))
	defer server.Close()
	c := Client{Key: "test", Model: "test", BaseURL: server.URL}
	if e := c.Explain(context.Background(), fixture()); e.Source != "rules" {
		t.Fatal("untrusted prose accepted")
	}
}

func TestUnusableClaudeResponsesFallBack(t *testing.T) {
	valid, _ := json.Marshal(narrativePlan(fixture()))
	textBlock := func(text string) []any { return []any{map[string]string{"type": "text", "text": text}} }
	cases := []struct {
		name string
		body any
	}{
		{"refusal", message("refusal", textBlock("I cannot provide this plan."))},
		{"truncated even if JSON parses", message("max_tokens", textBlock(string(valid)))},
		{"missing stop reason", message("", textBlock(string(valid)))},
		{"empty content", message("end_turn", nil)},
		{"only thinking", message("end_turn", []any{map[string]string{"type": "thinking", "thinking": string(valid)}})},
		{"unexpected tool", message("end_turn", []any{map[string]string{"type": "tool_use"}})},
		{"invalid JSON plan", message("end_turn", textBlock("{broken"))},
		{"extra JSON value", message("end_turn", textBlock(string(valid)+" {}"))},
		{"unsupported evidence", message("end_turn", textBlock(strings.Replace(string(valid), "e1", "fabricated", 1)))},
		{"empty plan", message("end_turn", textBlock("{}"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _ = json.NewEncoder(w).Encode(tc.body) }))
			defer server.Close()
			c := Client{Key: "test", BaseURL: server.URL}
			e := c.Explain(context.Background(), fixture())
			if e.Source != "rules" || e.FallbackReason == "" || len(e.Actions) == 0 || e.Facts[0].Observed != 110.5 {
				t.Fatal(e)
			}
		})
	}
}

func TestMissingKeyDoesNotCallProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("provider called without key") }))
	defer server.Close()
	c := Client{Key: "  ", BaseURL: server.URL}
	if e := c.Explain(context.Background(), fixture()); e.Source != "rules" || e.FallbackReason != "provider_not_configured" {
		t.Fatal(e)
	}
}

func TestClaudeTimeoutFallsBack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(40 * time.Millisecond) }))
	defer server.Close()
	c := Client{Key: "test", BaseURL: server.URL, HTTP: &http.Client{Timeout: 5 * time.Millisecond}}
	if e := c.Explain(context.Background(), fixture()); e.Source != "rules" || e.FallbackReason != "provider_timeout" {
		t.Fatal(e)
	}
}
