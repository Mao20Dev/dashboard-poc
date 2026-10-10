package engine

import (
	"energy/internal/domain"
	"testing"
	"time"
)

func synthetic(consumption, current float64, operational bool) domain.Input {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	in := domain.Input{Timezone: "UTC"}
	for i := 0; i < 192; i++ {
		c, a := 100.0, 100.0
		if i >= 168 {
			c, a = consumption, current
		}
		in.Readings = append(in.Readings, domain.Reading{MeterID: "test", Timestamp: start.Add(time.Duration(i) * time.Hour), Consumption: c, Current: a, Voltage: 220, PowerFactor: .9})
	}
	if operational {
		in.Events = append(in.Events, domain.Event{ID: "op", MeterID: "test", Timestamp: start.Add(168 * time.Hour), Type: "OPERATIONAL_CHANGE"})
	}
	return in
}
func TestDirectionalCorroborationAndModerateSeverity(t *testing.T) {
	opposite := run(t, synthetic(200, 50, false)).Findings[0]
	for _, factor := range opposite.ConfidenceFactors {
		if factor.Name == "electrical_corroboration" && factor.Value != 0 {
			t.Fatal("opposite current corroborates consumption")
		}
	}
	if opposite.Confidence >= .95 {
		t.Fatal("contradictory evidence has maximal score")
	}
	moderate := run(t, synthetic(130, 130, false)).Findings[0]
	if moderate.Severity != "MEDIUM" {
		t.Fatal("30 percent change must not default to high", moderate.Severity)
	}
	decrease := run(t, synthetic(60, 60, true)).Findings[0]
	if decrease.Type != "EXPLAINABLE_ANOMALY" {
		t.Fatal("operational decrease ignored")
	}
}
func TestOutageRequiresObservedRecovery(t *testing.T) {
	in := synthetic(20, 20, false)
	start := in.Readings[168].Timestamp
	in.Events = []domain.Event{{ID: "out", MeterID: "test", Timestamp: start, Type: "SCHEDULED_OUTAGE", DurationHours: 24}}
	f := run(t, in).Findings[0]
	if f.Type == "FALSE_POSITIVE" || f.Recovery != "UNKNOWN" {
		t.Fatal("dataset end mistaken for recovery")
	}
	for i := 0; i < 3; i++ {
		r := in.Readings[0]
		r.Timestamp = start.Add(time.Duration(24+i) * time.Hour)
		in.Readings = append(in.Readings, r)
	}
	f = run(t, in).Findings[0]
	if f.Type != "FALSE_POSITIVE" || f.Recovery != "CONFIRMED" {
		t.Fatal("recovery not recognized")
	}
	in.Readings = in.Readings[:len(in.Readings)-2]
	in.Readings[len(in.Readings)-1].Timestamp = in.Readings[len(in.Readings)-1].Timestamp.Add(time.Hour)
	if run(t, in).Findings[0].Type == "FALSE_POSITIVE" {
		t.Fatal("gap mistaken for recovery")
	}
}
