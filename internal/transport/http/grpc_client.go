package httpserver

import (
	"context"
	"time"

	meterusagev1 "github.com/milad/spectral/gen/go/proto/meterusage/v1"
	"google.golang.org/grpc"
)

// MeterUsageClient is the small subset of the gRPC client we need, to keep tests simple.
type MeterUsageClient interface {
	ListReadings(ctx context.Context, in *meterusagev1.ListReadingsRequest, opts ...grpc.CallOption) (*meterusagev1.ListReadingsResponse, error)
}

func parseOptionalRFC3339(v string) (*time.Time, error) {
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		// allow nano timestamps too (RFC3339Nano is a superset)
		t2, err2 := time.Parse(time.RFC3339Nano, v)
		if err2 != nil {
			return nil, err
		}
		t = t2
	}
	tt := t.UTC()
	return &tt, nil
}

