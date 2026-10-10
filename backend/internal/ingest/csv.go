package ingest

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"energy/internal/domain"
)

var durationPattern = regexp.MustCompile(`(?i)\b(\d+(?:\.\d+)?)\s*(?:hours?|horas?)\b`)
var meterPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func Load(dir, tz string) (domain.Input, error) {
	in := domain.Input{Timezone: tz, Readings: []domain.Reading{}, Events: []domain.Event{}}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return in, err
	}
	f, err := os.Open(filepath.Join(dir, "readings.csv"))
	if err != nil {
		return in, err
	}
	defer f.Close()
	in.Readings, err = Readings(f, loc)
	if err != nil {
		return in, err
	}
	f2, err := os.Open(filepath.Join(dir, "events.csv"))
	if err != nil {
		return in, err
	}
	defer f2.Close()
	rows, err := csv.NewReader(f2).ReadAll()
	if err != nil {
		return in, err
	}
	if len(rows) == 0 || strings.Join(rows[0], ",") != "meter_id,event_timestamp,event_type,description" {
		return in, fmt.Errorf("invalid event header")
	}
	meters := map[string]bool{}
	for _, r := range in.Readings {
		meters[r.MeterID] = true
	}
	for i, row := range rows[1:] {
		if len(row) != 4 || !meters[row[0]] {
			return in, fmt.Errorf("invalid event row %d", i+2)
		}
		ts, err := time.ParseInLocation("2006-01-02 15:04", row[1], loc)
		if err != nil {
			return in, fmt.Errorf("event row %d: invalid timestamp", i+2)
		}
		switch row[2] {
		case "OPERATIONAL_CHANGE", "SCHEDULED_OUTAGE", "UNKNOWN", "DATA_QUALITY":
		default:
			return in, fmt.Errorf("event row %d: unsupported type", i+2)
		}
		e := domain.Event{MeterID: row[0], Timestamp: ts, Type: row[2], Description: row[3]}
		if e.Type == "SCHEDULED_OUTAGE" {
			if m := durationPattern.FindStringSubmatch(e.Description); len(m) > 1 {
				e.DurationHours, _ = strconv.ParseFloat(m[1], 64)
			}
		}
		h := sha256.Sum256([]byte(strings.Join(row, "|")))
		e.ID = fmt.Sprintf("%x", h[:16])
		in.Events = append(in.Events, e)
	}
	return in, nil
}
func Readings(reader io.Reader, loc *time.Location) ([]domain.Reading, error) {
	cr := csv.NewReader(reader)
	header, err := cr.Read()
	if err != nil {
		return nil, err
	}
	header[0] = strings.TrimPrefix(header[0], "\ufeff")
	if strings.Join(header, ",") != "meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status" {
		return nil, fmt.Errorf("invalid readings header")
	}
	out := []domain.Reading{}
	seen := map[string]domain.Reading{}
	for n := 2; ; n++ {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", n, err)
		}
		if len(row) != 7 || !meterPattern.MatchString(row[0]) {
			return nil, fmt.Errorf("row %d: invalid meter or columns", n)
		}
		ts, err := time.ParseInLocation("2006-01-02 15:04:05", row[1], loc)
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid timestamp", n)
		}
		nums := [4]float64{}
		for i := range nums {
			v, e := strconv.ParseFloat(row[i+2], 64)
			if e != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
				return nil, fmt.Errorf("row %d: invalid numeric field %s", n, header[i+2])
			}
			nums[i] = v
		}
		if nums[3] > 1 {
			return nil, fmt.Errorf("row %d: power_factor must be in [0,1]", n)
		}
		r := domain.Reading{MeterID: row[0], Timestamp: ts, Consumption: nums[0], Voltage: nums[1], Current: nums[2], PowerFactor: nums[3], Status: row[6]}
		key := r.MeterID + ts.Format(time.RFC3339)
		if old, ok := seen[key]; ok {
			if old != r {
				return nil, fmt.Errorf("row %d: conflicting duplicate", n)
			}
			continue
		}
		seen[key] = r
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty readings dataset")
	}
	return out, nil
}
