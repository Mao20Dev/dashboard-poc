// Package explain combines immutable measured facts with a model-written,
// evidence-linked interpretation. Structural validation is not a proof of causality.
package explain

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"energy/internal/domain"
)

const PromptVersion = "analyst-2"

func plantLocation() *time.Location {
	loc, err := time.LoadLocation("America/Bogota")
	if err != nil {
		return time.UTC
	}
	return loc
}

type Plan struct {
	Narrative       *Narrative `json:"narrative,omitempty"`
	Focus           string     `json:"focus"`
	EvidenceIDs     []string   `json:"evidence_ids"`
	HypothesisCodes []string   `json:"hypothesis_codes"`
	ActionCodes     []string   `json:"action_codes"`
	QuestionCodes   []string   `json:"question_codes"`
}
type Catalog struct {
	Focus      string            `json:"focus"`
	Hypotheses map[string]string `json:"hypotheses"`
	Actions    map[string]string `json:"actions"`
	Questions  map[string]string `json:"questions"`
}

func Options(f domain.Finding) Catalog {
	c := Catalog{Focus: f.Type, Hypotheses: map[string]string{}, Actions: map[string]string{}, Questions: map[string]string{}}
	switch f.Type {
	case "DATA_QUALITY":
		c.Hypotheses["measurement_chain"] = "Hipótesis: un problema en la cadena de medición podría explicar la incoherencia; las lecturas no identifican su causa."
		c.Actions["crosscheck_meter"] = "Contrastar las lecturas afectadas con un instrumento o registro independiente antes de usarlas para decisiones de consumo."
		c.Actions["review_acquisition"] = "Revisar configuración, sincronización y adquisición del medidor con el equipo responsable."
		c.Questions["independent_measurement"] = "¿Existe una medición independiente para comparar los intervalos afectados?"
	case "EXPLAINABLE_ANOMALY":
		c.Hypotheses["operational_change"] = "Hipótesis: el evento registrado es compatible con el cambio de consumo, pero la coincidencia temporal no demuestra causalidad."
		c.Actions["confirm_operation"] = "Confirmar con operaciones la hora de inicio y el alcance del cambio registrado."
		c.Actions["review_baseline"] = "Si se confirma un nuevo régimen estable, proponer una nueva referencia sin modificar el histórico de esta investigación."
		c.Questions["operation_scope"] = "¿El cambio operativo afecta todo el consumo observado o solo una parte?"
	case "FALSE_POSITIVE":
		c.Hypotheses["scheduled_outage"] = "La caída observada queda dentro de la ventana de parada programada; no requiere escalarse como una anomalía sin explicación."
		c.Actions["verify_recovery"] = "Confirmar que la operación recuperó su comportamiento esperado al terminar la parada."
		c.Questions["outage_record"] = "¿Operaciones confirma que se ejecutó la parada registrada?"
	default:
		c.Hypotheses["unreported_load"] = "Hipótesis: un cambio de carga no registrado podría explicar la desviación. Debe contrastarse con la operación."
		c.Hypotheses["measurement_error"] = "Hipótesis alternativa: una desviación de medición también podría contribuir. Se necesita una comprobación independiente."
		c.Actions["check_operations"] = "Consultar con operaciones los cambios de carga durante la ventana detectada y registrar el resultado."
		c.Actions["crosscheck_meter"] = "Contrastar consumo y variables eléctricas con un registro independiente y solicitar revisión del personal responsable."
		c.Questions["load_record"] = "¿Se conectó, retiró o modificó alguna carga durante el periodo investigado?"
		c.Questions["independent_measurement"] = "¿Existe una medición independiente para contrastar el cambio?"
	}
	return c
}
func defaultPlan(f domain.Finding) Plan {
	c := Options(f)
	p := Plan{Focus: f.Type, EvidenceIDs: []string{}, HypothesisCodes: []string{}, ActionCodes: []string{}, QuestionCodes: []string{}}
	for _, e := range f.Evidence {
		p.EvidenceIDs = append(p.EvidenceIDs, e.ID)
	}
	// Stable order is part of reproducibility.
	for _, key := range []string{"unreported_load", "measurement_error", "measurement_chain", "operational_change", "scheduled_outage"} {
		if _, ok := c.Hypotheses[key]; ok {
			p.HypothesisCodes = append(p.HypothesisCodes, key)
		}
	}
	for _, key := range []string{"check_operations", "crosscheck_meter", "review_acquisition", "confirm_operation", "review_baseline", "verify_recovery"} {
		if _, ok := c.Actions[key]; ok {
			p.ActionCodes = append(p.ActionCodes, key)
		}
	}
	for _, key := range []string{"load_record", "independent_measurement", "operation_scope", "outage_record"} {
		if _, ok := c.Questions[key]; ok {
			p.QuestionCodes = append(p.QuestionCodes, key)
		}
	}
	return p
}
func Validate(f domain.Finding, p Plan) error {
	c := Options(f)
	if p.Focus != f.Type {
		return fmt.Errorf("classification_mismatch")
	}
	ids := map[string]string{}
	for _, e := range f.Evidence {
		ids[e.ID] = e.Metric
	}
	check := func(keys []string, allowed map[string]string, min int) error {
		if len(keys) < min || len(keys) > len(allowed) {
			return fmt.Errorf("invalid_selection")
		}
		seen := map[string]bool{}
		for _, k := range keys {
			if _, ok := allowed[k]; !ok || seen[k] {
				return fmt.Errorf("unsupported_or_duplicate_reference")
			}
			seen[k] = true
		}
		return nil
	}
	// Every fact remains visible: the provider cannot hide contradictory evidence.
	if err := check(p.EvidenceIDs, ids, len(ids)); err != nil {
		return err
	}
	// Legacy code selection remains only for deterministic plans and old tests.
	// A written interpretation has its own evidence-linked contract.
	if p.Narrative != nil {
		return nil
	}
	if err := check(p.HypothesisCodes, c.Hypotheses, 1); err != nil {
		return err
	}
	if err := check(p.ActionCodes, c.Actions, 1); err != nil {
		return err
	}
	return check(p.QuestionCodes, c.Questions, 1)
}
func Render(f domain.Finding, p Plan, source, model string) domain.Explanation {
	c := Options(f)
	e := domain.Explanation{Source: source, Model: model, Facts: []domain.Evidence{}, Hypotheses: []domain.Guidance{}, Actions: []domain.Guidance{}, Questions: []domain.Guidance{}, Limitations: []string{"La confianza es un índice de evidencia, no una probabilidad de avería.", "La referencia inicial utiliza los primeros siete días y supone que representan la operación habitual.", "Sin información de circuitos y equipos no se determina una causa física ni se calculan sanciones o costos."}}
	summary := map[string]string{"REAL_ANOMALY": "Cambio sostenido sin explicación operativa suficiente. Requiere investigación.", "DATA_QUALITY": "Lecturas eléctricas incoherentes con el comportamiento de consumo. Validar la medición.", "EXPLAINABLE_ANOMALY": "Cambio compatible con un evento operativo registrado. Confirmar su alcance.", "FALSE_POSITIVE": "Desviación temporal compatible con una parada programada. Verificar recuperación."}
	e.Summary = summary[f.Type]
	if len(f.Evidence) > 0 {
		fact := f.Evidence[0]
		if f.Type == "DATA_QUALITY" {
			e.Summary = fmt.Sprintf("%s presenta %d lecturas con incoherencias eléctricas. El consumo de esos intervalos suma %.1f kWh frente a %.1f kWh esperados. Contrasta la medición antes de atribuirlo a una falla operativa.", f.MeterID, fact.Samples, fact.Observed, fact.Baseline)
		} else if fact.ChangePercent != nil {
			e.Summary = fmt.Sprintf("%s: variación de %+.1f%% en %d intervalos horarios desde %s; %.1f kWh medidos frente a %.1f kWh esperados. %s", f.MeterID, *fact.ChangePercent, fact.Samples, f.Start.In(plantLocation()).Format("02/01 15:04"), fact.Observed, fact.Baseline, summary[f.Type])
		}
		if f.Recovery == "CONFIRMED" {
			e.Summary += " Se observan al menos tres horas consecutivas de recuperación."
		}
	}
	if f.Type == "REAL_ANOMALY" && len(f.Evidence) > 0 && f.Evidence[0].Samples < 3 {
		e.Summary = "Desviación puntual de gran magnitud. Contrastar la lectura y el contexto operativo."
	}
	for _, id := range p.EvidenceIDs {
		for _, fact := range f.Evidence {
			if fact.ID == id {
				e.Facts = append(e.Facts, fact)
			}
		}
	}
	for _, code := range p.HypothesisCodes {
		e.Hypotheses = append(e.Hypotheses, domain.Guidance{Code: code, Text: c.Hypotheses[code], EvidenceIDs: p.EvidenceIDs})
	}
	for _, code := range p.ActionCodes {
		e.Actions = append(e.Actions, domain.Guidance{Code: code, Text: c.Actions[code], EvidenceIDs: p.EvidenceIDs})
	}
	for _, code := range p.QuestionCodes {
		e.Questions = append(e.Questions, domain.Guidance{Code: code, Text: c.Questions[code], EvidenceIDs: p.EvidenceIDs})
	}
	if f.Type == "DATA_QUALITY" {
		e.Limitations = append(e.Limitations, "Las métricas del incidente agregan solo intervalos señalados, no todas las horas comprendidas entre inicio y fin.")
	}
	return e
}

type Client struct {
	Provider    string
	Key         string
	Model       string
	WorkspaceID string
	BaseURL     string
	HTTP        *http.Client
}

func (c *Client) Explain(ctx context.Context, f domain.Finding) domain.Explanation {
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	start := time.Now()
	fallback := Render(f, defaultPlan(f), "rules", "")
	if c != nil && c.Provider == "rules" {
		return fallback
	}
	if c == nil || strings.TrimSpace(c.Key) == "" {
		fallback.FallbackReason = "provider_not_configured"
		return fallback
	}
	var p Plan
	var err error
	if c.Provider == "openai" {
		p, err = c.generateOpenAI(ctx, f)
	} else {
		p, err = c.generate(ctx, f)
	}
	if err == nil {
		err = validateNarrative(f, p.Narrative)

	}
	if err != nil {
		fallback.FallbackReason = "provider_unavailable_or_invalid_plan"
		var failure providerError
		if errors.As(err, &failure) {
			fallback.FallbackReason = string(failure)
		}
		fallback.LatencyMS = time.Since(start).Milliseconds()
		return fallback
	}
	source := c.Provider
	if source == "" {
		source = "claude"
	}
	result := Render(f, defaultPlan(f), source, c.model())
	applyNarrative(&result, p.Narrative)
	result.LatencyMS = time.Since(start).Milliseconds()
	return result
}
