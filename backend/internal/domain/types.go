package domain

import "time"

type Reading struct {
	MeterID     string    `json:"meter_id"`
	Timestamp   time.Time `json:"timestamp"`
	Consumption float64   `json:"consumption_kwh"`
	Voltage     float64   `json:"voltage_v"`
	Current     float64   `json:"current_a"`
	PowerFactor float64   `json:"power_factor"`
	Status      string    `json:"status"`
}
type Event struct {
	ID            string    `json:"id"`
	MeterID       string    `json:"meter_id"`
	Timestamp     time.Time `json:"timestamp"`
	Type          string    `json:"type"`
	Description   string    `json:"description"`
	DurationHours float64   `json:"duration_hours,omitempty"`
}
type Input struct {
	Readings []Reading `json:"readings"`
	Events   []Event   `json:"events"`
	Timezone string    `json:"timezone"`
}

// Each fact binds a value to its meaning, unit and exact interval. The LLM never supplies numbers.
type Evidence struct {
	Minimum          float64     `json:"minimum"`
	Maximum          float64     `json:"maximum"`
	Changed          bool        `json:"changed"`
	ID               string      `json:"id"`
	Metric           string      `json:"metric"`
	Unit             string      `json:"unit"`
	Baseline         float64     `json:"baseline"`
	Observed         float64     `json:"observed"`
	Delta            float64     `json:"delta"`
	ChangePercent    *float64    `json:"change_percent,omitempty"`
	Start            time.Time   `json:"start"`
	End              time.Time   `json:"end"`
	ReferenceStart   time.Time   `json:"reference_start"`
	ReferenceEnd     time.Time   `json:"reference_end"`
	Samples          int         `json:"samples"`
	SampleTimestamps []time.Time `json:"sample_timestamps"`
	Aggregation      string      `json:"aggregation"`
	Text             string      `json:"text"`
}
type ScoreFactor struct {
	Name   string  `json:"name"`
	Value  float64 `json:"value"`
	Weight float64 `json:"weight"`
}
type Guidance struct {
	Code        string   `json:"code"`
	Text        string   `json:"text"`
	EvidenceIDs []string `json:"evidence_ids"`
}
type Explanation struct {
	Headline           string     `json:"headline,omitempty"`
	Interpretation     []Guidance `json:"interpretation,omitempty"`
	Uncertainty        string     `json:"uncertainty,omitempty"`
	NarrativeVersion   string     `json:"narrative_version,omitempty"`
	Source             string     `json:"source"`
	Model              string     `json:"model,omitempty"`
	Summary            string     `json:"summary"`
	SummaryEvidenceIDs []string   `json:"summary_evidence_ids,omitempty"`
	Facts              []Evidence `json:"facts"`
	Hypotheses         []Guidance `json:"hypotheses"`
	Actions            []Guidance `json:"actions"`
	Questions          []Guidance `json:"questions"`
	Limitations        []string   `json:"limitations"`
	FallbackReason     string     `json:"fallback_reason,omitempty"`
	LatencyMS          int64      `json:"latency_ms"`
}
type Finding struct {
	Recovery          string        `json:"recovery"`
	ID                string        `json:"id"`
	MeterID           string        `json:"meter_id"`
	Type              string        `json:"type"`
	Severity          string        `json:"severity"`
	Priority          int           `json:"priority"`
	Confidence        float64       `json:"confidence"`
	ConfidenceMeaning string        `json:"confidence_meaning"`
	ConfidenceFactors []ScoreFactor `json:"confidence_factors"`
	Start             time.Time     `json:"start"`
	End               time.Time     `json:"end"`
	Evidence          []Evidence    `json:"evidence"`
	RelatedEvents     []Event       `json:"related_events"`
	Explanation       Explanation   `json:"explanation"`
	Status            string        `json:"status"`
	Version           int           `json:"version"`
	RunID             string        `json:"run_id,omitempty"`
}
type Issue struct {
	MeterID string `json:"meter_id"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Report struct {
	Findings []Finding `json:"findings"`
	Issues   []Issue   `json:"issues"`
}
type Run struct {
	SelectedAt    time.Time      `json:"selected_at"`
	CacheHits     int            `json:"cache_hits"`
	Provider      string         `json:"provider"`
	Model         string         `json:"model"`
	ID            string         `json:"id"`
	Status        string         `json:"status"`
	Stage         string         `json:"stage"`
	InputHash     string         `json:"input_hash"`
	EngineVersion string         `json:"engine_version"`
	CreatedAt     time.Time      `json:"created_at"`
	StartedAt     *time.Time     `json:"started_at"`
	FinishedAt    *time.Time     `json:"finished_at"`
	Attempts      int            `json:"attempts"`
	Error         string         `json:"error,omitempty"`
	Summary       map[string]any `json:"summary"`
	Input         Input          `json:"-"`
	Lease         string         `json:"-"`
}
type Action struct {
	Verification *Verification `json:"verification,omitempty"`
	ID           string        `json:"id"`
	Action       string        `json:"action"`
	Note         string        `json:"note"`
	Actor        string        `json:"actor"`
	From         string        `json:"from_status"`
	To           string        `json:"to_status"`
	CreatedAt    time.Time     `json:"created_at"`
}

// Verification refers to a recommendation in an immutable analysis snapshot.
type Verification struct {
	RunID    string `json:"run_id"`
	StepCode string `json:"step_code"`
	Status   string `json:"status"`
}
