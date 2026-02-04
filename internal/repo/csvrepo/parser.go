package csvrepo

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/milad/spectral/internal/domain"
)

const (
	timeLayout = "2006-01-02 15:04:05"
)

// ParseReadingsCSV parses readings from the provided CSV reader.
//
// Expected header: time,meterusage
//
// Times are parsed using layout "2006-01-02 15:04:05" and interpreted as UTC.
// Invalid rows are skipped and returned as a joined error (errors.Join).
func ParseReadingsCSV(r io.Reader) ([]domain.Reading, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // be permissive; validate ourselves
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	if len(header) < 2 || strings.ToLower(strings.TrimSpace(header[0])) != "time" || strings.ToLower(strings.TrimSpace(header[1])) != "meterusage" {
		return nil, fmt.Errorf("unexpected header %q (want %q)", strings.Join(header, ","), "time,meterusage")
	}

	var (
		readings []domain.Reading
		rowErrs  []error
		rowNum   = 1 // header
	)

	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		if err != nil {
			rowErrs = append(rowErrs, fmt.Errorf("row %d: read: %w", rowNum, err))
			continue
		}
		if len(row) < 2 {
			rowErrs = append(rowErrs, fmt.Errorf("row %d: expected 2 columns, got %d", rowNum, len(row)))
			continue
		}

		t, err := time.ParseInLocation(timeLayout, strings.TrimSpace(row[0]), time.UTC)
		if err != nil {
			rowErrs = append(rowErrs, fmt.Errorf("row %d: parse time %q: %w", rowNum, row[0], err))
			continue
		}

		f, err := strconv.ParseFloat(strings.TrimSpace(row[1]), 64)
		if err != nil {
			rowErrs = append(rowErrs, fmt.Errorf("row %d: parse meterusage %q: %w", rowNum, row[1], err))
			continue
		}
		if math.IsNaN(f) || math.IsInf(f, 0) {
			rowErrs = append(rowErrs, fmt.Errorf("row %d: invalid meterusage %v", rowNum, f))
			continue
		}

		readings = append(readings, domain.Reading{
			Time:       t,
			MeterUsage: f,
		})
	}

	// Ensure we return stable, non-nil slice.
	if readings == nil {
		readings = []domain.Reading{}
	}
	return readings, errors.Join(rowErrs...)
}
