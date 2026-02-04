package service

import (
	"context"
	"fmt"
	"time"

	"github.com/milad/spectral/internal/domain"
	"github.com/milad/spectral/internal/repo"
)

type MeterUsageService struct {
	repo repo.ReadingRepository
}

func NewMeterUsageService(r repo.ReadingRepository) *MeterUsageService {
	return &MeterUsageService{repo: r}
}

func (s *MeterUsageService) ListReadings(ctx context.Context, startInclusive *time.Time, endExclusive *time.Time) ([]domain.Reading, error) {
	if startInclusive != nil && endExclusive != nil && !startInclusive.Before(*endExclusive) {
		// Keep it strict and predictable: [start, end) where start must be < end.
		return nil, fmt.Errorf("invalid time range: start must be before end")
	}
	return s.repo.List(ctx, startInclusive, endExclusive)
}

