## Spectral Tech Assignment (Go)

A small two-service implementation of the assignment:

- **gRPC** service serves time-series meter usage readings from `meterusage.csv`
- **HTTP** service calls gRPC and exposes the same data as **JSON**
- A tiny **single-page UI** fetches the JSON and renders a table

This is intentionally simple (no database), with tests focused on **ordering**, **range filtering**, and **gRPC ↔ HTTP mapping**.

### Run (local)

In terminal 1 (gRPC, default `:9090`):

```bash
make run-grpc
```

In terminal 2 (HTTP, default `:8080`):

```bash
make run-http
```

Open `http://localhost:8080/`.

### Run (Docker)

```bash
make docker-up
```

Open `http://localhost:8080/`.

### HTTP API

- **List readings**: `GET /api/readings?start=<RFC3339>&end=<RFC3339>`
  - `start` is inclusive, `end` is exclusive: \([start, end)\)
  - times should be RFC3339 (UTC recommended)

```bash
curl "http://localhost:8080/api/readings?start=2019-01-01T00:00:00Z&end=2019-01-01T01:00:00Z"
```

- **Health**: `GET /healthz`

### Tests

```bash
make test
```

### Design notes (why it looks like this)

- **In-order processing**: the CSV is loaded once at startup and sorted by timestamp; all responses preserve time order.
- **Boundaries stay boring**: service layer does validation, gRPC maps errors to codes, HTTP maps gRPC failures to HTTP statuses.
- **No TSDB**: the prompt explicitly says to serve the provided CSV; a time-series database would be unnecessary complexity here.

### Known quirk in the input data

The provided `meterusage.csv` contains at least one `NaN` value. Parsing **skips invalid rows** and continues; the gRPC server logs a warning at startup.

