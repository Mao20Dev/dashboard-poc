// Package engine contains pure, chronological analysis. It has no provider, SQL or HTTP dependencies.
package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"energy/internal/domain"
)

const Version = "1.1.0"

type Config struct {
	ReferenceDays     int     `json:"reference_days"`
	MinimumSamples    int     `json:"minimum_samples_per_hour"`
	ShiftThreshold    float64 `json:"shift_threshold"`
	MinimumShiftHours int     `json:"minimum_shift_hours"`
}

func DefaultConfig() Config { return Config{7, 3, .25, 3} }

type Slot struct {
	Consumption float64 `json:"consumption_kwh"`
	MAD         float64 `json:"mad"`
	Voltage     float64 `json:"voltage_v"`
	Current     float64 `json:"current_a"`
	PF          float64 `json:"power_factor"`
	Ratio       float64 `json:"electrical_ratio"`
	Samples     int     `json:"samples"`
}
type Profile struct {
	Slots [24]Slot  `json:"hourly"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Valid bool      `json:"valid"`
}

func Median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	ys := append([]float64(nil), xs...)
	sort.Float64s(ys)
	n := len(ys)
	if n%2 == 0 {
		return (ys[n/2-1] + ys[n/2]) / 2
	}
	return ys[n/2]
}
func values(rs []domain.Reading, f func(domain.Reading) float64) []float64 {
	out := make([]float64, len(rs))
	for i, r := range rs {
		out[i] = f(r)
	}
	return out
}
func ratio(r domain.Reading) float64 {
	p := r.Voltage * r.Current * r.PowerFactor / 1000
	if p <= 0 {
		return 0
	}
	return r.Consumption / p
}
func round(x float64) float64 { return math.Round(x*10000) / 10000 }
func change(actual, expected float64) float64 {
	if expected <= 0 {
		if actual > 0 {
			return 10
		}
		return 0
	}
	return (actual - expected) / expected
}
func Sorted(rs []domain.Reading) []domain.Reading {
	out := append([]domain.Reading(nil), rs...)
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.Before(out[j].Timestamp) })
	return out
}
func Groups(in domain.Input) map[string][]domain.Reading {
	g := map[string][]domain.Reading{}
	for _, r := range in.Readings {
		g[r.MeterID] = append(g[r.MeterID], r)
	}
	for k, v := range g {
		g[k] = Sorted(v)
	}
	return g
}
func BuildProfile(rs []domain.Reading, cfg Config, loc *time.Location) Profile {
	p := Profile{}
	if len(rs) == 0 {
		return p
	}
	rs = Sorted(rs)
	p.Start = rs[0].Timestamp
	p.End = p.Start.AddDate(0, 0, cfg.ReferenceDays)
	bins := [24][]domain.Reading{}
	for _, r := range rs {
		if r.Timestamp.Before(p.End) {
			h := r.Timestamp.In(loc).Hour()
			bins[h] = append(bins[h], r)
		}
	}
	p.Valid = true
	for h, bin := range bins {
		c := values(bin, func(r domain.Reading) float64 { return r.Consumption })
		m := Median(c)
		d := make([]float64, len(c))
		for i, v := range c {
			d[i] = math.Abs(v - m)
		}
		p.Slots[h] = Slot{m, Median(d), Median(values(bin, func(r domain.Reading) float64 { return r.Voltage })), Median(values(bin, func(r domain.Reading) float64 { return r.Current })), Median(values(bin, func(r domain.Reading) float64 { return r.PowerFactor })), Median(values(bin, ratio)), len(bin)}
		if len(bin) < cfg.MinimumSamples {
			p.Valid = false
		}
	}
	return p
}
func ValidateConfig(c Config) error {
	if c.ReferenceDays < 3 || c.ReferenceDays > 30 || c.MinimumSamples < 3 || c.MinimumSamples > c.ReferenceDays || c.ShiftThreshold < .1 || c.ShiftThreshold > 2 || c.MinimumShiftHours < 2 {
		return fmt.Errorf("invalid analysis configuration")
	}
	return nil
}
func Hash(in domain.Input, cfg Config) string {
	rs := append([]domain.Reading(nil), in.Readings...)
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].MeterID != rs[j].MeterID {
			return rs[i].MeterID < rs[j].MeterID
		}
		return rs[i].Timestamp.Before(rs[j].Timestamp)
	})
	es := append([]domain.Event(nil), in.Events...)
	sort.Slice(es, func(i, j int) bool { return es[i].ID < es[j].ID })
	b, _ := json.Marshal(struct {
		Version string
		Config  Config
		Input   domain.Input
	}{Version, cfg, domain.Input{Readings: rs, Events: es, Timezone: in.Timezone}})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func Analyze(in domain.Input, cfg Config) (domain.Report, error) {
	out := domain.Report{Findings: []domain.Finding{}, Issues: []domain.Issue{}}
	if err := ValidateConfig(cfg); err != nil {
		return out, err
	}
	loc, err := time.LoadLocation(in.Timezone)
	if err != nil {
		return out, err
	}
	for id, rs := range Groups(in) {
		p := BuildProfile(rs, cfg, loc)
		if !p.Valid {
			out.Issues = append(out.Issues, domain.Issue{MeterID: id, Code: "INSUFFICIENT_HISTORY", Message: "Se requieren al menos tres muestras históricas por hora; no se emitió un diagnóstico."})
			continue
		}
		quality := []domain.Reading{}
		consumption := []domain.Reading{}
		for i, r := range rs {
			if i > 0 && r.Timestamp.Sub(rs[i-1].Timestamp) != time.Hour {
				out.Issues = append(out.Issues, domain.Issue{MeterID: id, Code: "IRREGULAR_INTERVAL", Message: "La serie contiene intervalos irregulares; los huecos no se imputan como consumo cero."})
			}
			if r.Timestamp.Before(p.End) {
				continue
			}
			s := p.Slots[r.Timestamp.In(loc).Hour()]
			delta := change(r.Consumption, s.Consumption)
			voltageJump := math.Abs(r.Voltage-s.Voltage) > 12
			pfJump := math.Abs(r.PowerFactor-s.PF) > .15
			ratioJump := s.Ratio > 0 && math.Abs(change(ratio(r), s.Ratio)) > .45
			invalidElectrical := r.Voltage <= 0 || r.Current <= 0 || r.PowerFactor <= 0
			isQuality := (math.Abs(delta) < .2 && (voltageJump || pfJump || ratioJump)) || (r.Consumption > 1 && invalidElectrical)
			if isQuality {
				quality = append(quality, r)
				continue
			}
			scale := math.Max(1.4826*s.MAD, math.Max(.5, s.Consumption*.05))
			if math.Abs(delta) >= cfg.ShiftThreshold && math.Abs(r.Consumption-s.Consumption)/scale >= 4 {
				consumption = append(consumption, r)
			}
		}
		for _, segment := range segments(consumption, time.Hour, true, p, loc) {
			if len(segment) < cfg.MinimumShiftHours {
				peak := 0.0
				for _, r := range segment {
					peak = math.Max(peak, math.Abs(change(r.Consumption, p.Slots[r.Timestamp.In(loc).Hour()].Consumption)))
				}
				if peak < .8 {
					continue
				}
			}
			f := makeFinding(id, segment, p, in.Events, false, loc)
			f.Recovery = recovery(rs, f.End, p, loc)
			if f.Type == "FALSE_POSITIVE" && f.Recovery != "CONFIRMED" {
				f.Type = "EXPLAINABLE_ANOMALY"
				f.Severity = "MEDIUM"
				f.Priority = 45
			}
			if f.Type == "REAL_ANOMALY" && f.Recovery == "CONFIRMED" {
				f.Severity = "MEDIUM"
				f.Priority = 55
			}
			out.Findings = append(out.Findings, f)
		}
		for _, segment := range segments(quality, 6*time.Hour, false, p, loc) {
			if len(segment) < 2 {
				continue
			}
			f := makeFinding(id, segment, p, in.Events, true, loc)
			f.Recovery = "NOT_ASSESSED"
			out.Findings = append(out.Findings, f)
		}
	}
	sort.Slice(out.Findings, func(i, j int) bool {
		if out.Findings[i].Priority != out.Findings[j].Priority {
			return out.Findings[i].Priority > out.Findings[j].Priority
		}
		return out.Findings[i].ID < out.Findings[j].ID
	})
	return out, nil
}
func segments(rs []domain.Reading, gap time.Duration, directional bool, p Profile, loc *time.Location) [][]domain.Reading {
	out := [][]domain.Reading{}
	for _, r := range rs {
		split := len(out) == 0
		if !split {
			prev := out[len(out)-1][len(out[len(out)-1])-1]
			split = r.Timestamp.Sub(prev.Timestamp) > gap
			if directional {
				split = split || (r.Consumption-p.Slots[r.Timestamp.In(loc).Hour()].Consumption)*(prev.Consumption-p.Slots[prev.Timestamp.In(loc).Hour()].Consumption) < 0
			}
		}
		if split {
			out = append(out, []domain.Reading{})
		}
		out[len(out)-1] = append(out[len(out)-1], r)
	}
	return out
}
func makeFinding(id string, rs []domain.Reading, p Profile, events []domain.Event, quality bool, loc *time.Location) domain.Finding {
	start, end := rs[0].Timestamp, rs[len(rs)-1].Timestamp.Add(time.Hour)
	actual, expected := 0.0, 0.0
	for _, r := range rs {
		actual += r.Consumption
		expected += p.Slots[r.Timestamp.In(loc).Hour()].Consumption
	}
	delta := change(actual, expected)
	f := domain.Finding{MeterID: id, Type: "REAL_ANOMALY", Severity: "HIGH", Priority: 80 + int(math.Min(15, math.Abs(delta)*10)), Start: start, End: end, Status: "OPEN", Version: 1, RelatedEvents: []domain.Event{}, Evidence: []domain.Evidence{}, ConfidenceMeaning: "Índice de solidez de evidencia; no es una probabilidad calibrada."}
	if len(rs) < 3 || math.Abs(delta) < .5 {
		f.Severity = "MEDIUM"
		f.Priority = 55
	}
	if quality {
		f.Type = "DATA_QUALITY"
		f.Priority = 70
		f.Severity = "MEDIUM"
		if len(rs) >= 12 {
			f.Severity = "HIGH"
		}
	}
	for _, e := range events {
		if e.MeterID != id || e.Timestamp.After(end) || e.Timestamp.Before(start.Add(-24*time.Hour)) {
			continue
		}
		f.RelatedEvents = append(f.RelatedEvents, e)
		if quality {
			continue
		}
		if e.Type == "SCHEDULED_OUTAGE" && delta < 0 && e.DurationHours > 0 && !start.Before(e.Timestamp) && !end.After(e.Timestamp.Add(time.Duration(e.DurationHours*float64(time.Hour)))) {
			f.Type = "FALSE_POSITIVE"
			f.Severity = "LOW"
			f.Priority = 5
		}
		if e.Type == "OPERATIONAL_CHANGE" && !e.Timestamp.After(start) && start.Sub(e.Timestamp) <= 24*time.Hour && f.Type != "FALSE_POSITIVE" {
			f.Type = "EXPLAINABLE_ANOMALY"
			f.Severity = "MEDIUM"
			f.Priority = 45
		}
	}
	kind := "consumption"
	if quality {
		kind = "quality"
	}
	sum := sha256.Sum256([]byte(id + "|" + kind + "|" + start.UTC().Format(time.RFC3339)))
	f.ID = hex.EncodeToString(sum[:16])
	fact := func(metric, unit, aggregation string, base, obs float64) {
		e := domain.Evidence{ID: fmt.Sprintf("e%d", len(f.Evidence)+1), Metric: metric, Unit: unit, Baseline: round(base), Observed: round(obs), Delta: round(obs - base), Start: start, End: end, ReferenceStart: p.Start, ReferenceEnd: p.End, Samples: len(rs), Aggregation: aggregation}
		if base != 0 {
			v := round((obs - base) / base * 100)
			e.ChangePercent = &v
		}
		e.Text = fmt.Sprintf("%s: %.2f %s frente a %.2f %s de referencia (%s, %d muestras).", metric, obs, unit, base, unit, aggregation, len(rs))
		f.Evidence = append(f.Evidence, e)
		for _, r := range rs {
			f.Evidence[len(f.Evidence)-1].SampleTimestamps = append(f.Evidence[len(f.Evidence)-1].SampleTimestamps, r.Timestamp)
		}
	}
	fact("consumption_kwh", "kWh", "sum_selected_intervals", expected, actual)
	f.Evidence[0].Minimum = rs[0].Consumption
	f.Evidence[0].Maximum = rs[0].Consumption
	for _, r := range rs {
		f.Evidence[0].Minimum = math.Min(f.Evidence[0].Minimum, r.Consumption)
		f.Evidence[0].Maximum = math.Max(f.Evidence[0].Maximum, r.Consumption)
	}
	f.Evidence[0].Changed = math.Abs(delta) >= .25
	for _, m := range []struct {
		name, unit string
		reading    func(domain.Reading) float64
		slot       func(Slot) float64
	}{
		{"voltage_v", "V", func(r domain.Reading) float64 { return r.Voltage }, func(s Slot) float64 { return s.Voltage }},
		{"current_a", "A", func(r domain.Reading) float64 { return r.Current }, func(s Slot) float64 { return s.Current }},
		{"power_factor", "ratio", func(r domain.Reading) float64 { return r.PowerFactor }, func(s Slot) float64 { return s.PF }},
	} {
		baseline := []float64{}
		for _, r := range rs {
			baseline = append(baseline, m.slot(p.Slots[r.Timestamp.In(loc).Hour()]))
		}
		fact(m.name, m.unit, "median_selected_intervals", Median(baseline), Median(values(rs, m.reading)))
		e := &f.Evidence[len(f.Evidence)-1]
		e.Minimum = m.reading(rs[0])
		e.Maximum = e.Minimum
		for _, r := range rs {
			e.Minimum = math.Min(e.Minimum, m.reading(r))
			e.Maximum = math.Max(e.Maximum, m.reading(r))
		}
		e.Changed = math.Abs(change(e.Observed, e.Baseline)) >= .25
		if m.name == "voltage_v" {
			e.Changed = math.Abs(e.Delta) > 12
		}
		if m.name == "power_factor" {
			e.Changed = math.Abs(e.Delta) > .15
		}
	}
	coverage := math.Min(1, float64(len(rs))/12)
	magnitude := math.Min(1, math.Abs(delta))
	if quality {
		magnitude = 1
	}
	currentChange := change(f.Evidence[2].Observed, f.Evidence[2].Baseline)
	corroboration := 0.0
	if delta*currentChange > 0 {
		corroboration = math.Min(1, math.Abs(currentChange/delta))
	}
	if quality {
		corroboration = .5
		if f.Evidence[1].Changed && f.Evidence[3].Changed {
			corroboration = 1
		}
	}
	history := 0.0
	for _, slot := range p.Slots {
		history += math.Min(1, float64(slot.Samples)/7) / 24
	}
	f.ConfidenceFactors = []domain.ScoreFactor{{Name: "history_samples", Value: history, Weight: .3}, {Name: "affected_samples", Value: coverage, Weight: .25}, {Name: "signal_magnitude", Value: magnitude, Weight: .25}, {Name: "electrical_corroboration", Value: corroboration, Weight: .2}}
	for _, v := range f.ConfidenceFactors {
		f.Confidence += v.Value * v.Weight
	}
	f.Confidence = round(math.Min(.95, f.Confidence))
	return f
}

// A gap or the end of the dataset never proves recovery. Require three consecutive
// hourly readings immediately after the episode, within 20% of their baseline.
func recovery(rs []domain.Reading, end time.Time, p Profile, loc *time.Location) string {
	next := end
	count := 0
	for _, r := range rs {
		if r.Timestamp.Before(end) {
			continue
		}
		if !r.Timestamp.Equal(next) {
			return "UNKNOWN"
		}
		if math.Abs(change(r.Consumption, p.Slots[r.Timestamp.In(loc).Hour()].Consumption)) > .2 {
			return "NOT_RECOVERED"
		}
		count++
		if count == 3 {
			return "CONFIRMED"
		}
		next = next.Add(time.Hour)
	}
	return "UNKNOWN"
}
