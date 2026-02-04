package httpserver

import (
	"encoding/json"
	"net/http"
	"time"
)

type readingJSON struct {
	Time       string  `json:"time"`
	MeterUsage float64 `json:"meterUsage"`
}

type listReadingsResponseJSON struct {
	Readings      []readingJSON `json:"readings"`
	NextPageToken string        `json:"nextPageToken,omitempty"`
}

type apiErrorJSON struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
