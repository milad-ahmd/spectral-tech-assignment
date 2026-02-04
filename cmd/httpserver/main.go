package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	meterusagev1 "github.com/milad/spectral/gen/go/proto/meterusage/v1"
	httpserver "github.com/milad/spectral/internal/transport/http"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	var (
		addr     = flag.String("addr", envOr("HTTP_ADDR", ":8080"), "listen address")
		grpcAddr = flag.String("grpc", envOr("GRPC_TARGET", "127.0.0.1:9090"), "gRPC target host:port")
	)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conn, err := grpc.NewClient(*grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial gRPC %q: %v", *grpcAddr, err)
	}
	defer conn.Close()

	// Reduce docker-compose race: wait a bit for gRPC to be ready.
	wait := envDurationMs("GRPC_WAIT_TIMEOUT_MS", 20_000) // 20s default
	waitForGRPC(ctx, conn, wait)

	client := meterusagev1.NewMeterUsageServiceClient(conn)
	srv := httpserver.New(client)

	h := &http.Server{
		Addr:              *addr,
		Handler:           srv,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen %q: %v", *addr, err)
	}
	log.Printf("HTTP listening on %s (gRPC target %s)", *addr, *grpcAddr)

	go func() {
		<-ctx.Done()
		log.Printf("shutting down HTTP")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.Shutdown(shutdownCtx)
	}()

	if err := h.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func envDurationMs(k string, fallbackMs int) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return time.Duration(fallbackMs) * time.Millisecond
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return time.Duration(fallbackMs) * time.Millisecond
	}
	return time.Duration(n) * time.Millisecond
}

func waitForGRPC(ctx context.Context, conn *grpc.ClientConn, maxWait time.Duration) {
	if maxWait <= 0 {
		return
	}

	hc := healthpb.NewHealthClient(conn)
	deadline := time.Now().Add(maxWait)

	backoff := 100 * time.Millisecond
	for {
		if ctx.Err() != nil {
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
		_, err := hc.Check(reqCtx, &healthpb.HealthCheckRequest{})
		cancel()
		if err == nil {
			log.Printf("gRPC is ready")
			return
		}

		if time.Now().After(deadline) {
			log.Printf("warning: gRPC not ready after %s; continuing anyway (%v)", maxWait, err)
			return
		}

		time.Sleep(backoff)
		if backoff < 1*time.Second {
			backoff *= 2
			if backoff > 1*time.Second {
				backoff = 1 * time.Second
			}
		}
	}
}
