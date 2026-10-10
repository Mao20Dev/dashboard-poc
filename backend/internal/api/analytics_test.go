package api

import (
	"energy/internal/domain"
	"energy/internal/engine"
	"testing"
	"time"
)

func TestDailyHistoryKeepsMissingHoursAndMatchedBaseline(t *testing.T) {
	p := engine.Profile{}
	for i := range p.Slots {
		p.Slots[i].Consumption = 10
	}
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	rs := []domain.Reading{{Timestamp: start, Consumption: 20}, {Timestamp: start.Add(2 * time.Hour), Consumption: 30}}
	d := dailyHistory(rs, p, time.UTC)
	if len(d) != 1 || d[0].Samples != 2 || d[0].Complete || d[0].Consumption != 50 || d[0].Baseline != 20 {
		t.Fatal("missing hour was counted", d)
	}
}
func TestMeterSortDirectionAndStableTies(t *testing.T) {
	for _, direction := range []string{"asc", "desc"} {
		ms := []Meter{{ID: "B", Consumption: 20}, {ID: "A", Consumption: 20}, {ID: "C", Consumption: 10}}
		sortMeters(ms, "consumption", direction)
		if direction == "asc" && (ms[0].ID != "C" || ms[1].ID != "A") {
			t.Fatal(ms)
		}
		if direction == "desc" && (ms[0].ID != "A" || ms[1].ID != "B") {
			t.Fatal(ms)
		}
	}
}
