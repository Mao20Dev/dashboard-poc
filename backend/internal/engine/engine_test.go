package engine

import (
	"energy/internal/domain"
	"energy/internal/ingest"
	"math"
	"testing"
	"time"
)

func dataset(t *testing.T) domain.Input {
	t.Helper()
	in, err := ingest.Load("../../../data", "America/Bogota")
	if err != nil {
		t.Fatal(err)
	}
	return in
}
func run(t *testing.T, in domain.Input) domain.Report {
	t.Helper()
	out, err := Analyze(in, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestDatasetFindingsAndArithmetic(t *testing.T) {
	out := run(t, dataset(t))
	if len(out.Findings) != 4 {
		for _, f := range out.Findings {
			t.Log(f.MeterID, f.Type, f.Start, f.End)
		}
		t.Fatalf("got %d findings, want 4", len(out.Findings))
	}
	want := map[string]string{"M-109": "REAL_ANOMALY", "M-112": "DATA_QUALITY", "M-104": "EXPLAINABLE_ANOMALY", "M-106": "FALSE_POSITIVE"}
	for _, f := range out.Findings {
		if want[f.MeterID] != f.Type {
			t.Errorf("%s: %s", f.MeterID, f.Type)
		}
		if f.Start.Before(f.Evidence[0].ReferenceEnd) {
			t.Error("future information in baseline")
		}
		if f.Confidence < 0 || f.Confidence > 1 {
			t.Error("confidence out of range")
		}
	}
	f := out.Findings[0]
	if f.MeterID != "M-109" {
		t.Fatal("M-109 must rank first")
	}
	e := f.Evidence[0]
	if math.Abs(e.Observed-5380.8) > .01 || math.Abs(e.Baseline-2555.85) > .01 {
		t.Fatalf("independent totals mismatch: %+v", e)
	}
	if e.Samples != 58 || e.Unit != "kWh" {
		t.Fatal("wrong scope")
	}
}
func TestRenamingAndOrderDoNotAffectDecisions(t *testing.T) {
	in := dataset(t)
	before := run(t, in)
	for i := range in.Readings {
		in.Readings[i].MeterID = "new_" + in.Readings[i].MeterID
	}
	for i := range in.Events {
		in.Events[i].MeterID = "new_" + in.Events[i].MeterID
	}
	for i, j := 0, len(in.Readings)-1; i < j; i, j = i+1, j-1 {
		in.Readings[i], in.Readings[j] = in.Readings[j], in.Readings[i]
	}
	after := run(t, in)
	if len(before.Findings) != len(after.Findings) {
		t.Fatal("changed count")
	}
	for i, f := range before.Findings {
		g := after.Findings[i]
		if g.MeterID != "new_"+f.MeterID || g.Type != f.Type || g.Priority != f.Priority || g.Confidence != f.Confidence {
			t.Fatal("ID/order-dependent decision")
		}
	}
}
func TestEventsMustMatchWindow(t *testing.T) {
	in := dataset(t)
	for i := range in.Events {
		in.Events[i].Timestamp = in.Events[i].Timestamp.Add(10 * 24 * time.Hour)
	}
	for _, f := range run(t, in).Findings {
		if f.Type == "FALSE_POSITIVE" || f.Type == "EXPLAINABLE_ANOMALY" {
			t.Fatal("future event explained a current signal")
		}
	}
}
func TestWithoutEventsKeepsSignals(t *testing.T) {
	in := dataset(t)
	in.Events = nil
	out := run(t, in)
	if len(out.Findings) != 4 {
		t.Fatal(len(out.Findings))
	}
	for _, f := range out.Findings {
		if f.MeterID != "M-112" && f.Type != "REAL_ANOMALY" {
			t.Fatal("event-free misclassification")
		}
	}
}
func TestFingerprintIncludesEventContext(t *testing.T) {
	in := dataset(t)
	before := Hash(in, DefaultConfig())
	in.Events[0].Description += " corrected"
	if Hash(in, DefaultConfig()) == before {
		t.Fatal("event omitted from input hash")
	}
	before = Hash(in, DefaultConfig())
	cfg := DefaultConfig()
	cfg.ShiftThreshold = .3
	if Hash(in, cfg) == before {
		t.Fatal("config omitted from input hash")
	}
}
func TestInsufficientHistoryIsNotHealthy(t *testing.T) {
	in := dataset(t)
	in.Readings = in.Readings[:24]
	in.Events = nil
	out := run(t, in)
	if len(out.Findings) != 0 || len(out.Issues) != 1 || out.Issues[0].Code != "INSUFFICIENT_HISTORY" {
		t.Fatalf("%+v", out)
	}
}
func TestNormalSyntheticMeterStaysQuiet(t *testing.T) {
	in := domain.Input{Timezone: "America/Bogota"}
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 14*24; i++ {
		v := 20 + float64(i%24)
		in.Readings = append(in.Readings, domain.Reading{MeterID: "X", Timestamp: start.Add(time.Duration(i) * time.Hour), Consumption: v, Voltage: 220, Current: v * 1000 / (220 * .95), PowerFactor: .95, Status: "OK"})
	}
	if len(run(t, in).Findings) != 0 {
		t.Fatal("normal periodic signal flagged")
	}
}
func TestBaselineUnaffectedByFuture(t *testing.T) {
	in := dataset(t)
	rs := Groups(in)["M-109"]
	loc, _ := time.LoadLocation(in.Timezone)
	before := BuildProfile(rs, DefaultConfig(), loc)
	for i := 168; i < len(rs); i++ {
		rs[i].Consumption *= 100
	}
	after := BuildProfile(rs, DefaultConfig(), loc)
	if before != after {
		t.Fatal("baseline uses future data")
	}
}

func TestIsolatedSpikeOnNewMeter(t *testing.T) {
	in := domain.Input{Timezone: "America/Bogota"}
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 14*24; i++ {
		consumption := 30.0
		if i == 250 {
			consumption = 90
		}
		in.Readings = append(in.Readings, domain.Reading{MeterID: "new-meter", Timestamp: start.Add(time.Duration(i) * time.Hour), Consumption: consumption, Voltage: 220, Current: consumption * 1000 / (220 * .95), PowerFactor: .95})
	}
	out := run(t, in)
	if len(out.Findings) != 1 || out.Findings[0].Evidence[0].Samples != 1 || out.Findings[0].Severity != "MEDIUM" {
		t.Fatalf("spike not handled: %+v", out)
	}
}
