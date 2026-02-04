package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"time"

	meterusagev1 "github.com/milad/spectral/gen/go/proto/meterusage/v1"
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
	// Keep health checks quiet.
	if r.URL.Path == "/healthz" {
		s.mux.ServeHTTP(w, r)
		return
	}

	start := time.Now()
	reqID := newRequestID()

	w.Header().Set("X-Request-Id", reqID)
	rr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	s.mux.ServeHTTP(rr, r)

	log.Printf("%s %s -> %d (%s) req_id=%s",
		r.Method, r.URL.Path, rr.status, time.Since(start).Truncate(time.Millisecond), reqID,
	)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/readings", s.handleListReadings)
	s.mux.HandleFunc("/healthz", s.handleHealthz)
	s.mux.HandleFunc("/", s.handleIndex)
}

// handleListReadings returns JSON readings filtered by [start, end) if provided.
// Query params `start` and `end` must be RFC3339 (UTC recommended).
func (s *Server) handleListReadings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		_ = writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	start, err := parseOptionalRFC3339(r.URL.Query().Get("start"))
	if err != nil {
		_ = writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start"})
		return
	}
	end, err := parseOptionalRFC3339(r.URL.Query().Get("end"))
	if err != nil {
		_ = writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end"})
		return
	}
	if start != nil && end != nil && !start.Before(*end) {
		_ = writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid range: start must be before end"})
		return
	}

	req := &meterusagev1.ListReadingsRequest{}
	if start != nil {
		req.Start = timestamppb.New(*start)
	}
	if end != nil {
		req.End = timestamppb.New(*end)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.client.ListReadings(ctx, req)
	if err != nil {
		if st, ok := status.FromError(err); ok {
			if st.Code() == codes.InvalidArgument {
				_ = writeJSON(w, http.StatusBadRequest, map[string]string{"error": st.Message()})
				return
			}
		}
		_ = writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream error"})
		return
	}

	out := make([]readingJSON, 0, len(resp.GetReadings()))
	for _, rr := range resp.GetReadings() {
		ts := rr.GetTime()
		if ts == nil {
			_ = writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream returned invalid reading"})
			return
		}
		if err := ts.CheckValid(); err != nil {
			_ = writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream returned invalid timestamp"})
			return
		}
		t := ts.AsTime()
		out = append(out, readingJSON{
			Time:       formatTime(t),
			MeterUsage: rr.GetMeterUsage(),
		})
	}

	// Keep shape simple: JSON array of readings.
	_ = writeJSON(w, http.StatusOK, out)
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
			_ = writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
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
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func newRequestID() string {
	var b [6]byte // 12 hex chars
	if _, err := rand.Read(b[:]); err != nil {
		return "000000000000"
	}
	return hex.EncodeToString(b[:])
}
