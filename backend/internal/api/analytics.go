package api

import (
	"energy/internal/domain"
	"energy/internal/engine"
	"sort"
	"time"
)

func sortMeters(ms []Meter, field, direction string) {
	idSort := field == "" || field == "meter_id"
	ascending := direction == "asc" || (direction == "" && idSort)
	sort.Slice(ms, func(i, j int) bool {
		if idSort {
			if ascending {
				return ms[i].ID < ms[j].ID
			}
			return ms[i].ID > ms[j].ID
		}
		value := func(m Meter) float64 {
			switch field {
			case "consumption":
				return m.Consumption
			case "variation":
				return m.Variation
			default:
				return float64(m.Priority)
			}
		}
		a, b := value(ms[i]), value(ms[j])
		if a == b {
			return ms[i].ID < ms[j].ID
		}
		if ascending {
			return a < b
		}
		return a > b
	})
}

func allowed(value string, values ...string) bool {
	if value == "" {
		return true
	}
	for _, v := range values {
		if value == v {
			return true
		}
	}
	return false
}

type Day struct {
	Date        string  `json:"date"`
	Consumption float64 `json:"consumption_kwh"`
	Baseline    float64 `json:"baseline_kwh"`
	Samples     int     `json:"samples"`
	Complete    bool    `json:"complete"`
}

func dailyHistory(rs []domain.Reading, p engine.Profile, loc *time.Location) []Day {
	days := []Day{}
	for _, r := range rs {
		key := r.Timestamp.In(loc).Format("2006-01-02")
		if len(days) == 0 || days[len(days)-1].Date != key {
			days = append(days, Day{Date: key})
		}
		d := &days[len(days)-1]
		d.Consumption += r.Consumption
		d.Baseline += p.Slots[r.Timestamp.In(loc).Hour()].Consumption
		d.Samples++
		d.Complete = d.Samples == 24
	}
	return days
}

type Electrical struct {
	Metric   string   `json:"metric"`
	Unit     string   `json:"unit"`
	Observed float64  `json:"observed"`
	Baseline float64  `json:"baseline"`
	Change   *float64 `json:"change_percent,omitempty"`
}

func electrical(rs []domain.Reading, p engine.Profile, loc *time.Location) []Electrical {
	if len(rs) == 0 {
		return []Electrical{}
	}
	r := rs[len(rs)-1]
	s := p.Slots[r.Timestamp.In(loc).Hour()]
	out := []Electrical{{Metric: "voltage_v", Unit: "V", Observed: r.Voltage, Baseline: s.Voltage}, {Metric: "current_a", Unit: "A", Observed: r.Current, Baseline: s.Current}, {Metric: "power_factor", Unit: "", Observed: r.PowerFactor, Baseline: s.PF}}
	for i := range out {
		if out[i].Baseline > 0 {
			v := (out[i].Observed/out[i].Baseline - 1) * 100
			out[i].Change = &v
		}
	}
	return out
}
