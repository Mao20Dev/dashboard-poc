package explain

import (
	"context"
	"encoding/json"
	"energy/internal/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestInvalidNarrativeKeepsFactsWithoutPaidRepair(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := narrativePlan(fixture())
		if calls.Add(1) == 1 {
			p.Narrative.Headline = "Cambio registrado en 12 horas"
		}
		raw, _ := json.Marshal(p)
		json.NewEncoder(w).Encode(message("end_turn", []any{map[string]string{"type": "text", "text": string(raw)}}))
	}))
	defer server.Close()
	c := Client{Key: "test", BaseURL: server.URL}
	e := c.Explain(context.Background(), fixture())
	if calls.Load() != 1 || e.Source != "rules" || e.FallbackReason != "provider_invalid_narrative" || e.Facts[0].Observed != 110.5 {
		t.Fatal("repair failed", calls.Load(), e)
	}
}

func TestInvalidNarrativeNeverRetries(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		p := narrativePlan(fixture())
		p.Narrative.Headline = "Cambio registrado en 12 horas"
		raw, _ := json.Marshal(p)
		json.NewEncoder(w).Encode(message("end_turn", []any{map[string]string{"type": "text", "text": string(raw)}}))
	}))
	defer server.Close()
	c := Client{Key: "test", BaseURL: server.URL}
	e := c.Explain(context.Background(), fixture())
	if calls.Load() != 1 || e.Source != "rules" || e.FallbackReason != "provider_invalid_narrative" {
		t.Fatal("unbounded repair or mislabeled result", calls.Load(), e)
	}
}

func narrativePlan(f domain.Finding) Plan {
	p := defaultPlan(f)
	passage := func(text string) Passage { return Passage{Text: text, EvidenceIDs: []string{f.Evidence[0].ID}} }
	p.Narrative = &Narrative{
		Headline:       "El consumo cambió y falta explicar la causa",
		Summary:        passage("El consumo merece una revisión con operaciones. Las lecturas muestran un cambio, pero no identifican qué lo originó."),
		Interpretation: []Passage{passage("El cambio de consumo invita a contrastar los registros de operación antes de atribuir una falla."), passage("La lectura por sí sola no permite identificar el equipo involucrado; hace falta contexto operativo.")},
		Hypotheses:     []Passage{passage("Podría haber un cambio de carga no registrado. El registro de operación permitiría contrastarlo.")},
		Actions:        []Passage{passage("Consulta a operaciones por cambios de carga durante el periodo afectado para comprobar si explican la señal.")},
		Questions:      []Passage{passage("¿Hubo cambios de operación sin registrar durante el periodo afectado?")},
		Uncertainty:    "Estas lecturas no identifican el equipo responsable ni prueban una avería.",
	}
	return p
}

func TestNarrativeValidationRejectsUnsupportedReferencesAndNumbers(t *testing.T) {
	f := fixture()
	for name, mutate := range map[string]func(*Narrative){
		"numeric claim": func(n *Narrative) { n.Summary.Text = "El consumo aumentó 999 kWh sin que lo explique un evento." },
		"unicode digits": func(n *Narrative) {
			n.Summary.Text = "Se detectaron ９９９ fallas en el equipo durante la operación."
		},
		"invented citation": func(n *Narrative) { n.Actions[0].EvidenceIDs = []string{"invented"} },
		"missing citation":  func(n *Narrative) { n.Summary.EvidenceIDs = nil },
		"empty reasoning":   func(n *Narrative) { n.Interpretation = nil },
		"oversized prose":   func(n *Narrative) { n.Summary.Text = strings.Repeat("texto ", 300) },
		"markup": func(n *Narrative) {
			n.Summary.Text = "<script>La operación necesita una revisión cuidadosa.</script>"
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := narrativePlan(f)
			mutate(p.Narrative)
			if validateNarrative(f, p.Narrative) == nil {
				t.Fatal("invalid narrative accepted")
			}
		})
	}
	if validateNarrative(f, nil) == nil {
		t.Fatal("missing narrative accepted")
	}
}
func TestNarrativeRenderingKeepsFactsAndFreeInterpretation(t *testing.T) {
	f := fixture()
	p := narrativePlan(f)
	if err := validateNarrative(f, p.Narrative); err != nil {
		t.Fatal(err)
	}
	e := Render(f, p, "openai", DefaultOpenAIModel)
	before, _ := json.Marshal(e.Facts)
	applyNarrative(&e, p.Narrative)
	after, _ := json.Marshal(e.Facts)
	if string(before) != string(after) || e.Summary != p.Narrative.Summary.Text || e.Actions[0].Text != p.Narrative.Actions[0].Text || len(e.Interpretation) != 2 || e.NarrativeVersion != PromptVersion {
		t.Fatal("narrative not rendered or facts modified")
	}
}
