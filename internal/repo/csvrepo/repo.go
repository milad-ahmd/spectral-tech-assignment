package csvrepo

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/milad/spectral/internal/domain"
	"github.com/milad/spectral/internal/repo"
)

var _ repo.ReadingRepository = (*Repo)(nil)

// Repo is an in-memory repository backed by a CSV file loaded at startup.
type Repo struct {
	readings []domain.Reading // sorted ascending by Time
}

func NewFromFile(path string) (*Repo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv %q: %w", path, err)
	}
	defer f.Close()

	readings, parseErr := ParseReadingsCSV(f)
	if len(readings) == 0 && parseErr != nil {
		return nil, fmt.Errorf("parse csv %q: %w", path, parseErr)
	}
	sort.Slice(readings, func(i, j int) bool { return readings[i].Time.Before(readings[j].Time) })

	// Parsing can be partially successful; surface warnings to the caller.
	if parseErr != nil {
		return &Repo{readings: readings}, fmt.Errorf("parse csv %q: %w", path, parseErr)
	}
	return &Repo{readings: readings}, nil
}

func New(readings []domain.Reading) *Repo {
	cp := append([]domain.Reading(nil), readings...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Time.Before(cp[j].Time) })
	return &Repo{readings: cp}
}

func (r *Repo) List(ctx context.Context, startInclusive *time.Time, endExclusive *time.Time) ([]domain.Reading, error) {
	_ = ctx // reserved for future cancellation-aware backends

	readings := r.readings
	if startInclusive != nil {
		start := *startInclusive
		i := sort.Search(len(readings), func(i int) bool { return !readings[i].Time.Before(start) })
		readings = readings[i:]
	}
	if endExclusive != nil {
		end := *endExclusive
		j := sort.Search(len(readings), func(i int) bool { return !readings[i].Time.Before(end) })
		readings = readings[:j]
	}

	out := append([]domain.Reading(nil), readings...)
	return out, nil
}
