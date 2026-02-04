package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/milad/spectral/internal/domain"
	"github.com/milad/spectral/internal/repo/csvrepo"
)

func TestMeterUsageService_RejectsInvalidRange(t *testing.T) {
	t.Parallel()

	r := csvrepo.New([]domain.Reading{})
	svc := NewMeterUsageService(r)

	t0 := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0

	_, err := svc.ListReadings(context.Background(), &t0, &t1)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidTimeRange) {
		t.Fatalf("expected ErrInvalidTimeRange, got %v", err)
	}
}

func TestMeterUsageService_Pagination_CursorToken(t *testing.T) {
	t.Parallel()

	base := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	r := csvrepo.New([]domain.Reading{
		{Time: base.Add(15 * time.Minute), MeterUsage: 1.1},
		{Time: base.Add(30 * time.Minute), MeterUsage: 2.2},
		{Time: base.Add(45 * time.Minute), MeterUsage: 3.3},
	})
	svc := NewMeterUsageService(r)

	res1, err := svc.ListReadingsPage(context.Background(), nil, nil, 2, "")
	if err != nil {
		t.Fatalf("ListReadingsPage: %v", err)
	}
	if got, want := len(res1.Readings), 2; got != want {
		t.Fatalf("len=%d want %d", got, want)
	}
	if res1.NextPageToken == "" {
		t.Fatalf("expected next page token")
	}

	res2, err := svc.ListReadingsPage(context.Background(), nil, nil, 2, res1.NextPageToken)
	if err != nil {
		t.Fatalf("ListReadingsPage: %v", err)
	}
	if got, want := len(res2.Readings), 1; got != want {
		t.Fatalf("len=%d want %d", got, want)
	}
	if got, want := res2.Readings[0].Time, base.Add(45*time.Minute); !got.Equal(want) {
		t.Fatalf("time=%s want %s", got, want)
	}
	if res2.NextPageToken != "" {
		t.Fatalf("expected final page token to be empty, got %q", res2.NextPageToken)
	}
}

func TestMeterUsageService_RejectsTooLargeUnpagedRange(t *testing.T) {
	t.Parallel()

	r := csvrepo.New([]domain.Reading{})
	svc := NewMeterUsageService(r)

	start := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(MaxUnpagedRange + time.Hour)

	_, err := svc.ListReadings(context.Background(), &start, &end)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidTimeRange) {
		t.Fatalf("expected ErrInvalidTimeRange, got %v", err)
	}

	// With pagination, the same range is allowed.
	if _, err := svc.ListReadingsPage(context.Background(), &start, &end, 100, ""); err != nil {
		t.Fatalf("expected paged request to pass, got %v", err)
	}
}
