package httpserver

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests served.",
		},
		[]string{"route", "method", "status"},
	)
	httpRequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"route", "method"},
	)

	grpcUpstreamRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_upstream_requests_total",
			Help: "Total number of upstream gRPC calls made by the HTTP service.",
		},
		[]string{"method", "code"},
	)
	grpcUpstreamDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_upstream_duration_seconds",
			Help:    "Upstream gRPC call latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)
)

func observeHTTPRequest(r *http.Request, status int, dur time.Duration) {
	route := routeLabel(r.URL.Path)
	method := r.Method

	httpRequestsTotal.WithLabelValues(route, method, strconv.Itoa(status)).Inc()
	httpRequestDurationSeconds.WithLabelValues(route, method).Observe(dur.Seconds())
}

func observeUpstreamGRPC(method, code string, dur time.Duration) {
	grpcUpstreamRequestsTotal.WithLabelValues(method, code).Inc()
	grpcUpstreamDurationSeconds.WithLabelValues(method).Observe(dur.Seconds())
}

func routeLabel(path string) string {
	switch path {
	case "/":
		return "index"
	case "/api/readings":
		return "api_readings"
	case "/healthz":
		return "healthz"
	case "/metrics":
		return "metrics"
	default:
		return "other"
	}
}
