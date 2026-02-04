package repo

import (
	"context"
	"time"

	"github.com/milad/spectral/internal/domain"
)

// ReadingRepository provides access to meter usage readings.
type ReadingRepository interface {
	// List returns readings in ascending time order, optionally filtered by [start, end).
	// The returned slice must be treated as read-only by callers.
	List(ctx context.Context, startInclusive *time.Time, endExclusive *time.Time) ([]domain.Reading, error)
}
