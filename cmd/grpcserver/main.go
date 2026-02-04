package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcserver "github.com/milad/spectral/internal/transport/grpc"

	"github.com/milad/spectral/internal/repo/csvrepo"
	"github.com/milad/spectral/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	meterusagev1 "github.com/milad/spectral/gen/go/proto/meterusage/v1"
)

func main() {
	var (
		addr    = flag.String("addr", envOr("GRPC_ADDR", ":9090"), "listen address")
		csvPath = flag.String("csv", envOr("CSV_PATH", "meterusage.csv"), "path to meterusage.csv")
	)
	flag.Parse()

	repo, err := csvrepo.NewFromFile(*csvPath)
	if err != nil {
		// CSV may contain a few bad rows (e.g. NaN). We keep going if we have usable readings.
		log.Printf("warning: %v", err)
	}
	if repo == nil {
		log.Fatalf("failed to load csv from %q", *csvPath)
	}

	svc := service.NewMeterUsageService(repo)
	api := grpcserver.New(svc)

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen %q: %v", *addr, err)
	}
	log.Printf("gRPC listening on %s", *addr)

	g := grpc.NewServer()
	meterusagev1.RegisterMeterUsageServiceServer(g, api)

	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(g, hs)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Printf("shutting down gRPC")
		ch := make(chan struct{})
		go func() {
			g.GracefulStop()
			close(ch)
		}()
		select {
		case <-ch:
		case <-time.After(5 * time.Second):
			g.Stop()
		}
	}()

	if err := g.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
