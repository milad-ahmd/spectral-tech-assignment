package csvrepo

import (
	"context"
	"testing"
	"time"

	"github.com/milad/spectral/internal/domain"
)

func mustUTC(t *testing.T, s string) time.Time {
	t.Helper()
	got, err := time.ParseInLocation(timeLayout, s, time.UTC)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return got
}

func TestRepo_ListFiltersByTimeRange(t *testing.T) {
	t.Parallel()

	r := New([]domain.Reading{
		{Time: mustUTC(t, "2019-01-01 00:15:00"), MeterUsage: 1},
		{Time: mustUTC(t, "2019-01-01 00:30:00"), MeterUsage: 2},
		{Time: mustUTC(t, "2019-01-01 00:45:00"), MeterUsage: 3},
	})

	start := mustUTC(t, "2019-01-01 00:30:00")
	end := mustUTC(t, "2019-01-01 00:45:00")

	out, err := r.List(context.Background(), &start, &end)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got, want := len(out), 1; got != want {
		t.Fatalf("len(out)=%d want %d", got, want)
	}
	if got, want := out[0].MeterUsage, 2.0; got != want {
		t.Fatalf("out[0].MeterUsage=%v want %v", got, want)
	}
}
