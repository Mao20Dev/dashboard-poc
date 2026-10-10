package ingest

import (
	"strings"
	"testing"
	"time"
)

const header = "meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status\n"

func TestInvalidAndDuplicateReadings(t *testing.T) {
	row := "X,2026-09-01 00:00:00,10,220,50,0.95,OK\n"
	rs, err := Readings(strings.NewReader(header+row+row), time.UTC)
	if err != nil || len(rs) != 1 {
		t.Fatal("identical duplicate not idempotent", err)
	}
	for _, bad := range []string{strings.Replace(row, ",10,", ",NaN,", 1), strings.Replace(row, ",10,", ",-1,", 1), strings.Replace(row, "0.95", "1.1", 1), row + strings.Replace(row, ",10,", ",11,", 1)} {
		if _, err = Readings(strings.NewReader(header+bad), time.UTC); err == nil {
			t.Fatal("accepted invalid/conflicting data", bad)
		}
	}
}
