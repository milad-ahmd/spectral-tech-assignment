package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	meterusagev1 "github.com/milad/spectral/gen/go/proto/meterusage/v1"
	httpserver "github.com/milad/spectral/internal/transport/http"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	var (
		addr     = flag.String("addr", envOr("HTTP_ADDR", ":8080"), "listen address")
		grpcAddr = flag.String("grpc", envOr("GRPC_TARGET", "127.0.0.1:9090"), "gRPC target host:port")
	)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conn, err := grpc.DialContext(ctx, *grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial gRPC %q: %v", *grpcAddr, err)
	}
	defer conn.Close()

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

