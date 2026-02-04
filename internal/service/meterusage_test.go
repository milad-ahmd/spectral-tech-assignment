package service

import (
	"context"
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
}

