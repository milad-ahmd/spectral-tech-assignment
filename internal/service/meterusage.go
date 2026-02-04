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
var ErrInvalidPagination = errors.New("invalid pagination")

const (
	// MaxUnpagedRange is a guardrail against accidentally returning huge responses
	// when pagination is not used.
	MaxUnpagedRange = 31 * 24 * time.Hour
	MaxPageSize     = 5_000
)

type ListReadingsPageResult struct {
	Readings      []domain.Reading
	NextPageToken string
}

type MeterUsageService struct {
	repo repo.ReadingRepository
}

func NewMeterUsageService(r repo.ReadingRepository) *MeterUsageService {
	return &MeterUsageService{repo: r}
}

func (s *MeterUsageService) ListReadings(ctx context.Context, startInclusive *time.Time, endExclusive *time.Time) ([]domain.Reading, error) {
	res, err := s.ListReadingsPage(ctx, startInclusive, endExclusive, 0, "")
	return res.Readings, err
}

func (s *MeterUsageService) ListReadingsPage(
	ctx context.Context,
	startInclusive *time.Time,
	endExclusive *time.Time,
	pageSize int,
	pageToken string,
) (ListReadingsPageResult, error) {
	if startInclusive != nil && endExclusive != nil {
		// Keep it strict and predictable: [start, end) where start must be < end.
		if !startInclusive.Before(*endExclusive) {
			return ListReadingsPageResult{}, fmt.Errorf("%w: start must be before end", ErrInvalidTimeRange)
		}
		if pageSize <= 0 && endExclusive.Sub(*startInclusive) > MaxUnpagedRange {
			return ListReadingsPageResult{}, fmt.Errorf("%w: range too large without pagination (max %s)", ErrInvalidTimeRange, MaxUnpagedRange)
		}
	}

	if pageSize < 0 {
		return ListReadingsPageResult{}, fmt.Errorf("%w: page_size must be >= 0", ErrInvalidPagination)
	}
	if pageSize > MaxPageSize {
		return ListReadingsPageResult{}, fmt.Errorf("%w: page_size too large (max %d)", ErrInvalidPagination, MaxPageSize)
	}
	cursor, err := parseCursorToken(pageSize, pageToken)
	if err != nil {
		return ListReadingsPageResult{}, err
	}
	effectiveStart := startInclusive
	if cursor != nil && (effectiveStart == nil || cursor.After(*effectiveStart)) {
		effectiveStart = cursor
	}

	readings, err := s.repo.List(ctx, effectiveStart, endExclusive)
	if err != nil {
		return ListReadingsPageResult{}, err
	}
	if cursor != nil {
		// Cursor is exclusive: skip any reading at or before the cursor.
		i := 0
		for i < len(readings) && !readings[i].Time.After(*cursor) {
			i++
		}
		readings = readings[i:]
	}

	// Unpaged behavior (backwards compatible): return everything.
	if pageSize == 0 {
		if pageToken != "" {
			return ListReadingsPageResult{}, fmt.Errorf("%w: page_token requires page_size", ErrInvalidPagination)
		}
		return ListReadingsPageResult{
			Readings:      readings,
			NextPageToken: "",
		}, nil
	}

	if len(readings) == 0 {
		return ListReadingsPageResult{
			Readings:      nil,
			NextPageToken: "",
		}, nil
	}

	end := pageSize
	if end > len(readings) {
		end = len(readings)
	}
	page := readings[:end]
	next := ""
	if end < len(readings) {
		next = page[len(page)-1].Time.UTC().Format(time.RFC3339Nano)
	}
	return ListReadingsPageResult{
		Readings:      page,
		NextPageToken: next,
	}, nil
}

func parseCursorToken(pageSize int, pageToken string) (*time.Time, error) {
	if pageToken == "" {
		return nil, nil
	}
	if pageSize <= 0 {
		return nil, fmt.Errorf("%w: page_token requires page_size", ErrInvalidPagination)
	}
	t, err := time.Parse(time.RFC3339, pageToken)
	if err != nil {
		// allow nano timestamps too (RFC3339Nano is a superset)
		t2, err2 := time.Parse(time.RFC3339Nano, pageToken)
		if err2 != nil {
			return nil, fmt.Errorf("%w: invalid page_token", ErrInvalidPagination)
		}
		t = t2
	}
	tt := t.UTC()
	return &tt, nil
}
