package explain

import "strings"

// Registry is shared by HTTP admission and the worker; it never exposes credentials.
type Registry map[string]*Client
type Provider struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Model      string `json:"model"`
	Configured bool   `json:"configured"`
}

func (r Registry) List() []Provider {
	out := []Provider{}
	for _, p := range []Provider{{ID: "claude", Name: "Claude Sonnet"}, {ID: "openai", Name: "GPT"}, {ID: "rules", Name: "Reglas verificables", Configured: true}} {
		if c := r[p.ID]; c != nil {
			p.Model = c.model()
			p.Configured = strings.TrimSpace(c.Key) != ""
		}
		if p.ID == "rules" {
			p.Configured = true
			p.Model = ""
		}
		out = append(out, p)
	}
	return out
}
func (r Registry) Default() string {
	for _, p := range r.List() {
		if p.Configured {
			return p.ID
		}
	}
	return "rules"
}
func (r Registry) Resolve(id string) (*Client, bool) {
	if id == "rules" {
		return &Client{Provider: "rules"}, true
	}
	c := r[id]
	return c, c != nil && strings.TrimSpace(c.Key) != ""
}
func (c *Client) ModelName() string {
	if c.Provider == "rules" {
		return ""
	}
	return c.model()
}
