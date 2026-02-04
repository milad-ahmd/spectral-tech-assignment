package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/milad/spectral/internal/domain"
	"github.com/milad/spectral/internal/repo"
)

var ErrInvalidTimeRange = errors.New("invalid time range")

type MeterUsageService struct {
	repo repo.ReadingRepository
}

func NewMeterUsageService(r repo.ReadingRepository) *MeterUsageService {
	return &MeterUsageService{repo: r}
}

func (s *MeterUsageService) ListReadings(ctx context.Context, startInclusive *time.Time, endExclusive *time.Time) ([]domain.Reading, error) {
	if startInclusive != nil && endExclusive != nil && !startInclusive.Before(*endExclusive) {
		// Keep it strict and predictable: [start, end) where start must be < end.
		return nil, fmt.Errorf("%w: start must be before end", ErrInvalidTimeRange)
	}
	return s.repo.List(ctx, startInclusive, endExclusive)
}
