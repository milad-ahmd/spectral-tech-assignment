## Spectral Tech Assignment (Go)

This repo implements the assignment requirements:

- **gRPC server** serves time-based electricity consumption data from `meterusage.csv`
- **HTTP server** calls the gRPC server and exposes the data as **JSON**
- **Single-page HTML** fetches the JSON and renders it in a table

### Quick start (local)

Run gRPC (default `:9090`):

```bash
go run ./cmd/grpcserver -csv ./meterusage.csv -addr :9090
```

Run HTTP (default `:8080`, pointing at gRPC):

```bash
go run ./cmd/httpserver -addr :8080 -grpc 127.0.0.1:9090
```

Open:
- `http://localhost:8080/`

### API

- **HTTP JSON**: `GET /api/readings?start=<RFC3339>&end=<RFC3339>`
  - `start` is inclusive, `end` is exclusive: \([start, end)\)
  - Example:

```bash
curl "http://localhost:8080/api/readings?start=2019-01-01T00:00:00Z&end=2019-01-02T00:00:00Z"
```

- **HTTP health**: `GET /healthz`

### Tests

```bash
go test ./...
```

### Docker

```bash
docker compose up --build
```

Then open `http://localhost:8080/`.

### Notes (kept intentionally simple)

- The CSV is **loaded once at startup** into memory and sorted by time, so responses stay **in-order**.
- One row in the provided CSV contains `NaN`; parsing **skips invalid rows** and continues (the gRPC server logs a warning at startup).

### Trade-offs

- **No TSDB**: the assignment asks to serve the CSV; adding a time-series database would be unnecessary complexity here.
- **In-memory load**: simplest way to keep reads fast and ordered; acceptable for the provided dataset size.
- **Range semantics**: \([start,end)\) avoids double-counting boundary points and matches common time-series APIs.
- **Bad rows**: invalid rows are skipped so the service stays available, but a warning is logged at startup.

