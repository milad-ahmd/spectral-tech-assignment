package httpserver

import (
	"net/http"

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
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/readings", s.handleListReadings)
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
}

func (s *Server) handleListReadings(w http.ResponseWriter, r *http.Request) {
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

	ctx := r.Context()
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
		t := rr.GetTime().AsTime()
		out = append(out, readingJSON{
			Time:       formatTime(t),
			MeterUsage: rr.GetMeterUsage(),
		})
	}

	// Keep shape simple: JSON array of readings.
	_ = writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTMLStatic))
}

var indexHTMLStatic = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Meter Usage</title>
    <style>
      body { font-family: system-ui, -apple-system, Segoe UI, Roboto, sans-serif; margin: 24px; color: #111; }
      .row { display: flex; gap: 12px; flex-wrap: wrap; align-items: end; margin-bottom: 16px; }
      label { display: flex; flex-direction: column; gap: 6px; font-size: 14px; }
      input { padding: 8px 10px; font-size: 14px; width: 240px; }
      button { padding: 9px 12px; font-size: 14px; cursor: pointer; }
      table { border-collapse: collapse; width: 100%; margin-top: 16px; }
      th, td { border-bottom: 1px solid #eee; padding: 10px 8px; text-align: left; font-size: 14px; }
      th { position: sticky; top: 0; background: #fff; }
      .muted { color: #666; font-size: 13px; }
      .error { color: #b00020; font-size: 13px; }
      code { background: #f6f6f6; padding: 2px 6px; border-radius: 6px; }
    </style>
  </head>
  <body>
    <h1>Meter usage readings</h1>
    <p class="muted">Data comes from <code>/api/readings</code> (HTTP) backed by gRPC.</p>

    <div class="row">
      <label>
        Start (RFC3339)
        <input id="start" placeholder="2019-01-01T00:00:00Z" />
      </label>
      <label>
        End (RFC3339)
        <input id="end" placeholder="2019-01-02T00:00:00Z" />
      </label>
      <button id="load">Load</button>
    </div>

    <div id="meta" class="muted"></div>
    <div id="err" class="error"></div>

    <table>
      <thead>
        <tr><th>Time (UTC)</th><th>Meter usage</th></tr>
      </thead>
      <tbody id="rows"></tbody>
    </table>

    <script>
      const elStart = document.getElementById('start');
      const elEnd = document.getElementById('end');
      const elRows = document.getElementById('rows');
      const elMeta = document.getElementById('meta');
      const elErr = document.getElementById('err');

      function setError(msg) { elErr.textContent = msg || ''; }
      function setMeta(msg) { elMeta.textContent = msg || ''; }

      function render(readings) {
        elRows.innerHTML = '';
        for (const r of readings) {
          const tr = document.createElement('tr');
          const tdT = document.createElement('td');
          const tdU = document.createElement('td');
          tdT.textContent = r.time;
          tdU.textContent = String(r.meterUsage);
          tr.appendChild(tdT);
          tr.appendChild(tdU);
          elRows.appendChild(tr);
        }
      }

      async function load() {
        setError('');
        setMeta('Loading...');
        const qs = new URLSearchParams();
        if (elStart.value.trim()) qs.set('start', elStart.value.trim());
        if (elEnd.value.trim()) qs.set('end', elEnd.value.trim());

        const res = await fetch('/api/readings?' + qs.toString(), { headers: { 'Accept': 'application/json' }});
        const body = await res.json().catch(() => null);
        if (!res.ok) {
          setMeta('');
          setError((body && body.error) ? body.error : ('Request failed: ' + res.status));
          return;
        }
        setMeta('Loaded ' + body.length + ' readings.');
        render(body);
      }

      document.getElementById('load').addEventListener('click', load);
      load();
    </script>
  </body>
</html>`;

