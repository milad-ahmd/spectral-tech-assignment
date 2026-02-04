package grpcserver

import (
	"context"
	"net"
	"testing"
	"time"

	meterusagev1 "github.com/milad/spectral/gen/go/proto/meterusage/v1"
	"github.com/milad/spectral/internal/domain"
	"github.com/milad/spectral/internal/repo/csvrepo"
	"github.com/milad/spectral/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestServer_ListReadings_PreservesOrder(t *testing.T) {
	t.Parallel()

	base := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := csvrepo.New([]domain.Reading{
		{Time: base.Add(30 * time.Minute), MeterUsage: 2},
		{Time: base.Add(15 * time.Minute), MeterUsage: 1},
		{Time: base.Add(45 * time.Minute), MeterUsage: 3},
	})
	svc := service.NewMeterUsageService(repo)
	srv := New(svc)

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	g := grpc.NewServer()
	meterusagev1.RegisterMeterUsageServiceServer(g, srv)
	go func() { _ = g.Serve(lis) }()
	t.Cleanup(g.Stop)

	ctx := context.Background()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := meterusagev1.NewMeterUsageServiceClient(conn)

	resp, err := client.ListReadings(ctx, &meterusagev1.ListReadingsRequest{})
	if err != nil {
		t.Fatalf("ListReadings: %v", err)
	}
	if got, want := len(resp.Readings), 3; got != want {
		t.Fatalf("len(readings)=%d want %d", got, want)
	}
	if !resp.Readings[0].Time.AsTime().Before(resp.Readings[1].Time.AsTime()) ||
		!resp.Readings[1].Time.AsTime().Before(resp.Readings[2].Time.AsTime()) {
		t.Fatalf("expected strict ascending times, got %v, %v, %v",
			resp.Readings[0].Time.AsTime(), resp.Readings[1].Time.AsTime(), resp.Readings[2].Time.AsTime())
	}
}

func TestServer_ListReadings_FiltersRange(t *testing.T) {
	t.Parallel()

	base := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := csvrepo.New([]domain.Reading{
		{Time: base.Add(15 * time.Minute), MeterUsage: 1},
		{Time: base.Add(30 * time.Minute), MeterUsage: 2},
		{Time: base.Add(45 * time.Minute), MeterUsage: 3},
	})
	svc := service.NewMeterUsageService(repo)
	srv := New(svc)

	lis := bufconn.Listen(1024 * 1024)
	g := grpc.NewServer()
	meterusagev1.RegisterMeterUsageServiceServer(g, srv)
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

	client := meterusagev1.NewMeterUsageServiceClient(conn)

	start := timestamppb.New(base.Add(30 * time.Minute))
	end := timestamppb.New(base.Add(45 * time.Minute))
	resp, err := client.ListReadings(context.Background(), &meterusagev1.ListReadingsRequest{Start: start, End: end})
	if err != nil {
		t.Fatalf("ListReadings: %v", err)
	}
	if got, want := len(resp.Readings), 1; got != want {
		t.Fatalf("len(readings)=%d want %d", got, want)
	}
	if got, want := resp.Readings[0].MeterUsage, 2.0; got != want {
		t.Fatalf("meter usage=%v want %v", got, want)
	}
}
