package explain

import (
	"context"
	"encoding/json"
	"energy/internal/domain"
	"errors"
	"io"
	"sort"
	"strings"
	"time"
)

func (c *Client) request(ctx context.Context, input []byte, schema map[string]any, prompt string, tokens int) (string, error) {
	if c.Provider == "openai" {
		return c.requestOpenAI(ctx, input, schema, prompt, tokens)
	}
	return c.requestClaude(ctx, input, schema, prompt, tokens)
}

// Legacy single-finding entry point shares the transport and never retries.
func (c *Client) generate(ctx context.Context, f domain.Finding) (Plan, error) {
	input, err := json.Marshal(map[string]any{"finding": f, "prompt_version": PromptVersion})
	if err != nil {
		return Plan{}, err
	}
	raw, err := c.request(ctx, input, planSchema(), analystPrompt, 4096)
	var p Plan
	if err == nil {
		err = decodeStrict(raw, &p)
	}
	return p, err
}
func (c *Client) generateOpenAI(ctx context.Context, f domain.Finding) (Plan, error) {
	return c.generate(ctx, f)
}
func decodeStrict(raw string, target any) error {
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(target) != nil {
		return providerError("provider_invalid_plan")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return providerError("provider_invalid_plan")
	}
	return nil
}

type batchItem struct {
	FindingID string     `json:"finding_id"`
	Narrative *Narrative `json:"narrative"`
}
type batchResponse struct {
	Analyses []batchItem `json:"analyses"`
}

// ExplainBatch sends only computed evidence and compact plant context, never raw
// time series. A malformed or failed response falls back once; no paid repairs.
func (c *Client) ExplainBatch(ctx context.Context, findings []domain.Finding, input domain.Input) []domain.Explanation {
	out := make([]domain.Explanation, len(findings))
	for i, f := range findings {
		out[i] = Render(f, defaultPlan(f), "rules", "")
	}
	if len(findings) == 0 || (c != nil && c.Provider == "rules") {
		return out
	}
	fallback := func(err error) []domain.Explanation {
		code := "provider_unavailable_or_invalid_plan"
		var failure providerError
		if errors.As(err, &failure) {
			code = string(failure)
		}
		for i := range out {
			out[i].FallbackReason = code
		}
		return out
	}
	if c == nil || strings.TrimSpace(c.Key) == "" {
		return fallback(providerError("provider_not_configured"))
	}
	// Bound both input size and maximum generation cost before any HTTP request.
	if len(findings) > 12 {
		return fallback(providerError("provider_batch_too_large"))
	}
	compact := make([]domain.Finding, len(findings))
	for i, f := range findings {
		compact[i] = f
		compact[i].Explanation = domain.Explanation{}
		compact[i].Evidence = append([]domain.Evidence(nil), f.Evidence...)
		for j := range compact[i].Evidence {
			compact[i].Evidence[j].SampleTimestamps = nil
		}
	}
	type meterContext struct {
		ID          string  `json:"meter_id"`
		Readings    int     `json:"readings"`
		Consumption float64 `json:"consumption_kwh"`
	}
	meters := map[string]*meterContext{}
	for _, r := range input.Readings {
		if meters[r.MeterID] == nil {
			meters[r.MeterID] = &meterContext{ID: r.MeterID}
		}
		meters[r.MeterID].Readings++
		meters[r.MeterID].Consumption += r.Consumption
	}
	ids := make([]string, 0, len(meters))
	for id := range meters {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	contextMeters := make([]*meterContext, 0, len(ids))
	for _, id := range ids {
		contextMeters = append(contextMeters, meters[id])
	}
	payload, err := json.Marshal(map[string]any{"findings": compact, "meters": contextMeters, "timezone": input.Timezone, "prompt_version": PromptVersion})
	if err != nil || len(payload) > 128*1024 {
		return fallback(providerError("provider_batch_too_large"))
	}
	narrative := planSchema()["properties"].(map[string]any)["narrative"]
	item := map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"finding_id": map[string]any{"type": "string"}, "narrative": narrative}, "required": []string{"finding_id", "narrative"}}
	schema := map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"analyses": map[string]any{"type": "array", "items": item}}, "required": []string{"analyses"}}
	prompt := strings.ReplaceAll(analystPrompt, "Devuelve solo narrative", "Devuelve analyses") + `
 Analiza la planta completa en esta única solicitud. Devuelve un objeto analyses con exactamente una entrada por finding: finding_id idéntico y narrative. Las instrucciones de narrative se aplican a cada entrada. Usa exclusivamente las evidencias del hallazgo correspondiente; no mezcles medidores. meters aporta contexto de toda la planta, incluidos medidores sin hallazgos. No inventes anomalías para ellos.`
	started := time.Now()
	raw, err := c.request(ctx, payload, schema, prompt, min(12000, 3000*len(findings)))
	if err != nil {
		return fallback(err)
	}
	var response batchResponse
	if err = decodeStrict(raw, &response); err != nil {
		return fallback(err)
	}
	if len(response.Analyses) != len(findings) {
		return fallback(providerError("provider_invalid_batch"))
	}
	byID := map[string]*Narrative{}
	for _, a := range response.Analyses {
		if _, exists := byID[a.FindingID]; exists {
			return fallback(providerError("provider_invalid_batch"))
		}
		byID[a.FindingID] = a.Narrative
	}
	// Validate the complete batch before rendering any model attribution.
	for _, f := range findings {
		if err = validateNarrative(f, byID[f.ID]); err != nil {
			return fallback(err)
		}
	}
	source := c.Provider
	if source == "" {
		source = "claude"
	}
	for i, f := range findings {
		out[i] = Render(f, defaultPlan(f), source, c.model())
		applyNarrative(&out[i], byID[f.ID])
		out[i].LatencyMS = time.Since(started).Milliseconds()
	}
	return out
}
