package domain

import "time"

// Reading represents a single meter usage reading at a point in time.
type Reading struct {
	Time       time.Time
	MeterUsage float64
}
