package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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

	offset, err := parseOffsetToken(pageSize, pageToken)
	if err != nil {
		return ListReadingsPageResult{}, err
	}
	if pageSize < 0 {
		return ListReadingsPageResult{}, fmt.Errorf("%w: page_size must be >= 0", ErrInvalidPagination)
	}
	if pageSize > MaxPageSize {
		return ListReadingsPageResult{}, fmt.Errorf("%w: page_size too large (max %d)", ErrInvalidPagination, MaxPageSize)
	}

	readings, err := s.repo.List(ctx, startInclusive, endExclusive)
	if err != nil {
		return ListReadingsPageResult{}, err
	}
	if offset > len(readings) {
		return ListReadingsPageResult{}, fmt.Errorf("%w: page_token out of range", ErrInvalidPagination)
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

	if offset == len(readings) {
		return ListReadingsPageResult{
			Readings:      nil,
			NextPageToken: "",
		}, nil
	}

	end := offset + pageSize
	if end > len(readings) {
		end = len(readings)
	}
	next := ""
	if end < len(readings) {
		next = strconv.Itoa(end)
	}
	return ListReadingsPageResult{
		Readings:      readings[offset:end],
		NextPageToken: next,
	}, nil
}

func parseOffsetToken(pageSize int, pageToken string) (int, error) {
	if pageToken == "" {
		return 0, nil
	}
	if pageSize <= 0 {
		return 0, fmt.Errorf("%w: page_token requires page_size", ErrInvalidPagination)
	}
	n, err := strconv.Atoi(pageToken)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%w: invalid page_token", ErrInvalidPagination)
	}
	return n, nil
}
