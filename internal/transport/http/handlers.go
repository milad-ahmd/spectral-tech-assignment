package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	meterusagev1 "github.com/milad/spectral/gen/go/proto/meterusage/v1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	client MeterUsageClient
	mux    *http.ServeMux
}

func New(client MeterUsageClient) *Server {
	s := &Server{
		client: client,
		mux:    http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	reqID := newRequestID()

	w.Header().Set("X-Request-Id", reqID)
	rr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	defer func() {
		if rec := recover(); rec != nil {
			rr.status = http.StatusInternalServerError

			// Best-effort response. If headers/body were already written, we can
			// only log.
			if !rr.wroteHeader {
				if strings.HasPrefix(r.URL.Path, "/api") {
					writeAPIError(rr, http.StatusInternalServerError, "internal_error", "internal error")
				} else {
					http.Error(rr, "internal error", http.StatusInternalServerError)
				}
			}

			log.Printf("panic handling %s %s req_id=%s: %v\n%s",
				r.Method, r.URL.Path, reqID, rec, debug.Stack(),
			)
		}

		dur := time.Since(start)
		observeHTTPRequest(r, rr.status, dur)

		// Keep health checks + metrics endpoint quiet.
		if r.URL.Path != "/healthz" && r.URL.Path != "/metrics" {
			log.Printf("%s %s -> %d (%s) req_id=%s",
				r.Method, r.URL.Path, rr.status, dur.Truncate(time.Millisecond), reqID,
			)
		}
	}()

	s.mux.ServeHTTP(rr, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/readings", s.handleListReadings)
	s.mux.HandleFunc("/healthz", s.handleHealthz)
	s.mux.Handle("/metrics", promhttp.Handler())
	s.mux.HandleFunc("/", s.handleIndex)
}

// handleListReadings returns JSON readings filtered by [start, end) if provided.
// Query params `start` and `end` must be RFC3339 (UTC recommended).
func (s *Server) handleListReadings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	start, err := parseOptionalRFC3339(r.URL.Query().Get("start"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", "invalid start")
		return
	}
	end, err := parseOptionalRFC3339(r.URL.Query().Get("end"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", "invalid end")
		return
	}
	if start != nil && end != nil && !start.Before(*end) {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", "invalid range: start must be before end")
		return
	}

	pageSize, err := parseOptionalInt(r.URL.Query().Get("page_size"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", "invalid page_size")
		return
	}
	if pageSize < 0 {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", "page_size must be >= 0")
		return
	}
	pageToken := r.URL.Query().Get("page_token")
	if pageToken != "" && pageSize == 0 {
		writeAPIError(w, http.StatusBadRequest, "invalid_argument", "page_token requires page_size")
		return
	}

	req := &meterusagev1.ListReadingsRequest{}
	if start != nil {
		req.Start = timestamppb.New(*start)
	}
	if end != nil {
		req.End = timestamppb.New(*end)
	}
	if pageSize != 0 {
		req.PageSize = int32(pageSize)
	}
	if pageToken != "" {
		req.PageToken = pageToken
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	grpcStart := time.Now()
	resp, err := s.client.ListReadings(ctx, req)
	grpcDur := time.Since(grpcStart)
	if err != nil {
		code := codes.Unknown.String()
		if st, ok := status.FromError(err); ok {
			code = st.Code().String()
			if st.Code() == codes.InvalidArgument {
				observeUpstreamGRPC("ListReadings", code, grpcDur)
				writeAPIError(w, http.StatusBadRequest, "invalid_argument", st.Message())
				return
			}
			if st.Code() == codes.DeadlineExceeded {
				observeUpstreamGRPC("ListReadings", code, grpcDur)
				writeAPIError(w, http.StatusGatewayTimeout, "upstream_timeout", "upstream timeout")
				return
			}
		}
		observeUpstreamGRPC("ListReadings", code, grpcDur)
		writeAPIError(w, http.StatusBadGateway, "upstream_error", "upstream error")
		return
	}
	observeUpstreamGRPC("ListReadings", codes.OK.String(), grpcDur)

	out := make([]readingJSON, 0, len(resp.GetReadings()))
	for _, rr := range resp.GetReadings() {
		ts := rr.GetTime()
		if ts == nil {
			writeAPIError(w, http.StatusBadGateway, "upstream_error", "upstream returned invalid reading")
			return
		}
		if err := ts.CheckValid(); err != nil {
			writeAPIError(w, http.StatusBadGateway, "upstream_error", "upstream returned invalid timestamp")
			return
		}
		t := ts.AsTime()
		out = append(out, readingJSON{
			Time:       formatTime(t),
			MeterUsage: rr.GetMeterUsage(),
		})
	}

	_ = writeJSON(w, http.StatusOK, listReadingsResponseJSON{
		Readings:      out,
		NextPageToken: resp.GetNextPageToken(),
	})
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		// Keep API errors JSON.
		if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
			writeAPIError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		http.NotFound(w, r) // HTML/plain-text is fine for non-API paths.
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(p)
}

func newRequestID() string {
	var b [6]byte // 12 hex chars
	if _, err := rand.Read(b[:]); err != nil {
		return "000000000000"
	}
	return hex.EncodeToString(b[:])
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	reqID := w.Header().Get("X-Request-Id")
	_ = writeJSON(w, status, apiErrorJSON{
		Code:      code,
		Message:   message,
		RequestID: reqID,
	})
}

func parseOptionalInt(v string) (int, error) {
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, err
	}
	return n, nil
}
