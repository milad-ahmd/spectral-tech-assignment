package httpserver

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
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

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "passthrough:///bufnet",
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

	var got []readingJSON
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	// start is inclusive: includes 00:30 and 00:45, but excludes 01:00.
	if got[0].Time != "2019-01-01T00:30:00Z" || got[1].Time != "2019-01-01T00:45:00Z" {
		t.Fatalf("unexpected times: %#v", got)
	}
	if got[0].MeterUsage != 2.2 || got[1].MeterUsage != 3.3 {
		t.Fatalf("unexpected usages: %#v", got)
	}
}

