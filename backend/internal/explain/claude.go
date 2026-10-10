package explain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultModel = "claude-sonnet-5-5"

// Only these application-owned error codes may be persisted or shown to users.
type providerError string

func (e providerError) Error() string { return string(e) }

func (c *Client) model() string {
	if model := strings.TrimSpace(c.Model); model != "" {
		return model
	}
	if c.Provider == "openai" {
		return DefaultOpenAIModel
	}
	return DefaultModel
}

func (c *Client) requestClaude(ctx context.Context, payload []byte, schema map[string]any, prompt string, tokens int) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	reqBody, err := json.Marshal(map[string]any{
		"model": c.model(), "max_tokens": tokens, "system": prompt,
		"messages": []any{map[string]string{"role": "user", "content": string(payload)}},
		"output_config": map[string]any{
			"effort": "low",
			"format": map[string]any{"type": "json_schema", "schema": schema},
		},
	})
	if err != nil {
		return "", fmt.Errorf("invalid_request")
	}
	base := c.BaseURL
	if base == "" {
		base = "https://api.anthropic.com/v1"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/messages", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("invalid_provider_url")
	}
	req.Header.Set("x-api-key", strings.TrimSpace(c.Key))
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	if c.WorkspaceID != "" {
		req.Header.Set("anthropic-workspace-id", c.WorkspaceID)
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 61 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}
	}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", providerError("provider_timeout")
		}
		return "", providerError("provider_transport_error")
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return "", providerError("provider_authentication_failed")
	case http.StatusTooManyRequests:
		return "", providerError("provider_rate_limited")
	case http.StatusBadRequest, http.StatusNotFound:
		return "", providerError("provider_configuration_error")
	default:
		return "", providerError("provider_unavailable")
	}
	const maxResponseBytes = 1 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return "", providerError("provider_invalid_response")
	}
	var result struct {
		Type       string `json:"type"`
		Role       string `json:"role"`
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", providerError("provider_invalid_response")
	}
	if result.StopReason == "refusal" {
		return "", providerError("provider_refused")
	}
	if result.StopReason != "end_turn" || result.Type != "message" || result.Role != "assistant" {
		return "", providerError("provider_incomplete_response")
	}
	var output strings.Builder
	for _, block := range result.Content {
		switch block.Type {
		case "text":
			output.WriteString(block.Text)
		case "thinking", "redacted_thinking":
			// Internal reasoning is neither used as evidence nor persisted.
		default:
			return "", providerError("provider_invalid_response")
		}
	}
	if output.Len() == 0 {
		return "", providerError("provider_invalid_response")
	}
	return output.String(), nil
}
