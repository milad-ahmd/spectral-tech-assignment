package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	meterusagev1 "github.com/milad/spectral/gen/go/proto/meterusage/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeClient struct {
	resp *meterusagev1.ListReadingsResponse
	err  error
	req  *meterusagev1.ListReadingsRequest
}

func (f *fakeClient) ListReadings(ctx context.Context, in *meterusagev1.ListReadingsRequest, _ ...grpc.CallOption) (*meterusagev1.ListReadingsResponse, error) {
	f.req = in
	return f.resp, f.err
}

func TestHTTP_ListReadings_OK_PreservesOrder(t *testing.T) {
	t.Parallel()

	t0 := time.Date(2019, 1, 1, 0, 15, 0, 0, time.UTC)
	t1 := t0.Add(15 * time.Minute)

	fc := &fakeClient{
		resp: &meterusagev1.ListReadingsResponse{
			Readings: []*meterusagev1.Reading{
				{Time: timestamppb.New(t0), MeterUsage: 1.1},
				{Time: timestamppb.New(t1), MeterUsage: 2.2},
			},
		},
	}
	srv := New(fc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/readings", nil)
	srv.ServeHTTP(rr, req)

	if got, want := rr.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d want %d, body=%s", got, want, rr.Body.String())
	}

	var got listReadingsResponseJSON
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Readings) != 2 {
		t.Fatalf("len=%d want 2", len(got.Readings))
	}
	if got.Readings[0].Time != formatTime(t0) || got.Readings[1].Time != formatTime(t1) {
		t.Fatalf("unexpected order or time formatting: %#v", got.Readings)
	}
}

func TestHTTP_ListReadings_InvalidStart(t *testing.T) {
	t.Parallel()

	srv := New(&fakeClient{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/readings?start=not-a-time", nil)
	srv.ServeHTTP(rr, req)

	if got, want := rr.Code, http.StatusBadRequest; got != want {
		t.Fatalf("status=%d want %d", got, want)
	}
}

func TestHTTP_ListReadings_MapsUpstreamInvalidArgument(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{err: status.Error(codes.InvalidArgument, "bad range")}
	srv := New(fc)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/readings", nil)
	srv.ServeHTTP(rr, req)

	if got, want := rr.Code, http.StatusBadRequest; got != want {
		t.Fatalf("status=%d want %d", got, want)
	}
}

func TestHTTP_ListReadings_MapsUpstreamUnavailableToBadGateway(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{err: status.Error(codes.Unavailable, "nope")}
	srv := New(fc)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/readings", nil)
	srv.ServeHTTP(rr, req)

	if got, want := rr.Code, http.StatusBadGateway; got != want {
		t.Fatalf("status=%d want %d", got, want)
	}
}

func TestHTTP_ListReadings_PageTokenRequiresPageSize(t *testing.T) {
	t.Parallel()

	srv := New(&fakeClient{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/readings?page_token=10", nil)
	srv.ServeHTTP(rr, req)

	if got, want := rr.Code, http.StatusBadRequest; got != want {
		t.Fatalf("status=%d want %d", got, want)
	}
}

func TestHTTP_Index(t *testing.T) {
	t.Parallel()

	srv := New(&fakeClient{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	srv.ServeHTTP(rr, req)

	if got, want := rr.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d want %d", got, want)
	}
	if ct := rr.Header().Get("Content-Type"); ct == "" {
		t.Fatalf("expected content-type")
	}
	if body := rr.Body.String(); body == "" {
		t.Fatalf("expected body")
	}
}
