package httpserver

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	meterusagev1 "github.com/milad/spectral/gen/go/proto/meterusage/v1"
	"github.com/milad/spectral/internal/domain"
	"github.com/milad/spectral/internal/repo/csvrepo"
	"github.com/milad/spectral/internal/service"
	grpcserver "github.com/milad/spectral/internal/transport/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// This is a light end-to-end test:
// HTTP handler -> gRPC client -> in-memory gRPC server -> service -> repo.
func TestHTTP_ToGRPC_EndToEnd(t *testing.T) {
	t.Parallel()

	base := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := csvrepo.New([]domain.Reading{
		{Time: base.Add(15 * time.Minute), MeterUsage: 1.1},
		{Time: base.Add(30 * time.Minute), MeterUsage: 2.2},
		{Time: base.Add(45 * time.Minute), MeterUsage: 3.3},
	})
	svc := service.NewMeterUsageService(repo)
	api := grpcserver.New(svc)

	lis := bufconn.Listen(1024 * 1024)
	g := grpc.NewServer()
	meterusagev1.RegisterMeterUsageServiceServer(g, api)
	go func() { _ = g.Serve(lis) }()
	t.Cleanup(g.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	httpSrv := New(meterusagev1.NewMeterUsageServiceClient(conn))

	req := httptest.NewRequest(http.MethodGet,
		"/api/readings?start=2019-01-01T00:30:00Z&end=2019-01-01T01:00:00Z",
		nil,
	)
	rr := httptest.NewRecorder()
	httpSrv.ServeHTTP(rr, req)

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
	// start is inclusive: includes 00:30 and 00:45, but excludes 01:00.
	if got.Readings[0].Time != "2019-01-01T00:30:00Z" || got.Readings[1].Time != "2019-01-01T00:45:00Z" {
		t.Fatalf("unexpected times: %#v", got.Readings)
	}
	if got.Readings[0].MeterUsage != 2.2 || got.Readings[1].MeterUsage != 3.3 {
		t.Fatalf("unexpected usages: %#v", got.Readings)
	}
}

func TestHTTP_ToGRPC_EndToEnd_CursorPagination(t *testing.T) {
	t.Parallel()

	base := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := csvrepo.New([]domain.Reading{
		{Time: base.Add(15 * time.Minute), MeterUsage: 1.1},
		{Time: base.Add(30 * time.Minute), MeterUsage: 2.2},
		{Time: base.Add(45 * time.Minute), MeterUsage: 3.3},
	})
	svc := service.NewMeterUsageService(repo)
	api := grpcserver.New(svc)

	lis := bufconn.Listen(1024 * 1024)
	g := grpc.NewServer()
	meterusagev1.RegisterMeterUsageServiceServer(g, api)
	go func() { _ = g.Serve(lis) }()
	t.Cleanup(g.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	httpSrv := New(meterusagev1.NewMeterUsageServiceClient(conn))

	rr1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet,
		"/api/readings?start=2019-01-01T00:00:00Z&end=2019-01-01T01:00:00Z&page_size=2",
		nil,
	)
	httpSrv.ServeHTTP(rr1, req1)

	if got, want := rr1.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d want %d, body=%s", got, want, rr1.Body.String())
	}

	var page1 listReadingsResponseJSON
	if err := json.Unmarshal(rr1.Body.Bytes(), &page1); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got, want := len(page1.Readings), 2; got != want {
		t.Fatalf("len=%d want %d", got, want)
	}
	if page1.NextPageToken != "2019-01-01T00:30:00Z" {
		t.Fatalf("nextPageToken=%q want %q", page1.NextPageToken, "2019-01-01T00:30:00Z")
	}

	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet,
		"/api/readings?start=2019-01-01T00:00:00Z&end=2019-01-01T01:00:00Z&page_size=2&page_token="+url.QueryEscape(page1.NextPageToken),
		nil,
	)
	httpSrv.ServeHTTP(rr2, req2)

	if got, want := rr2.Code, http.StatusOK; got != want {
		t.Fatalf("status=%d want %d, body=%s", got, want, rr2.Body.String())
	}

	var page2 listReadingsResponseJSON
	if err := json.Unmarshal(rr2.Body.Bytes(), &page2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got, want := len(page2.Readings), 1; got != want {
		t.Fatalf("len=%d want %d", got, want)
	}
	if page2.Readings[0].Time != "2019-01-01T00:45:00Z" {
		t.Fatalf("time=%q want %q", page2.Readings[0].Time, "2019-01-01T00:45:00Z")
	}
	if page2.NextPageToken != "" {
		t.Fatalf("expected empty nextPageToken, got %q", page2.NextPageToken)
	}
}
