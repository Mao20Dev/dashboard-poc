package explain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultOpenAIModel = "gpt-6-luna"

func (c *Client) requestOpenAI(ctx context.Context, input []byte, schema map[string]any, prompt string, tokens int) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	body, err := json.Marshal(map[string]any{
		"model": c.model(), "store": false, "max_output_tokens": tokens,
		"instructions": prompt,
		"input":        string(input), "reasoning": map[string]string{"effort": "low"},
		"text": map[string]any{"format": map[string]any{"type": "json_schema", "name": "investigation_plan", "strict": true, "schema": schema}},
	})
	if err != nil {
		return "", providerError("invalid_request")
	}
	base := c.BaseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/responses", bytes.NewReader(body))
	if err != nil {
		return "", providerError("invalid_provider_url")
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Key))
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 61 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	res, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", providerError("provider_timeout")
		}
		return "", providerError("provider_transport_error")
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case 200:
	case 401, 403:
		return "", providerError("provider_authentication_failed")
	case 429:
		return "", providerError("provider_rate_limited")
	case 400, 404:
		return "", providerError("provider_configuration_error")
	default:
		return "", providerError("provider_unavailable")
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		return "", providerError("provider_invalid_response")
	}
	var result struct {
		Status string `json:"status"`
		Output []struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return "", providerError("provider_invalid_response")
	}
	if result.Status != "completed" {
		return "", providerError("provider_incomplete_response")
	}
	var output strings.Builder
	for _, item := range result.Output {
		if item.Type == "reasoning" {
			continue
		}
		if item.Type != "message" || item.Role != "assistant" {
			return "", providerError("provider_invalid_response")
		}
		for _, block := range item.Content {
			if block.Type == "refusal" {
				return "", providerError("provider_refused")
			}
			if block.Type != "output_text" {
				return "", providerError("provider_invalid_response")
			}
			output.WriteString(block.Text)
		}
	}
	return output.String(), nil
}
