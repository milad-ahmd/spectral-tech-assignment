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

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
