package csvrepo

import (
	"strings"
	"testing"
	"time"
)

func TestParseReadingsCSV_OK(t *testing.T) {
	t.Parallel()

	csv := strings.NewReader(strings.TrimSpace(`
time,meterusage
2019-01-01 00:15:00,55.09
2019-01-01 00:30:00,54.64
`))

	readings, err := ParseReadingsCSV(csv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := len(readings), 2; got != want {
		t.Fatalf("len(readings)=%d want %d", got, want)
	}
	if readings[0].Time.Location() != time.UTC {
		t.Fatalf("time location=%v want UTC", readings[0].Time.Location())
	}
	if got, want := readings[0].MeterUsage, 55.09; got != want {
		t.Fatalf("meter usage[0]=%v want %v", got, want)
	}
}

func TestParseReadingsCSV_SkipsInvalidRows(t *testing.T) {
	t.Parallel()

	csv := strings.NewReader(strings.TrimSpace(`
time,meterusage
2019-01-01 00:15:00,55.09
2019-01-01 00:30:00,NaN
not-a-time,12.0
2019-01-01 00:45:00,55.18
`))

	readings, err := ParseReadingsCSV(csv)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got, want := len(readings), 2; got != want {
		t.Fatalf("len(readings)=%d want %d", got, want)
	}
}
