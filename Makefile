.PHONY: test fmt vet check run-grpc run-http docker-up docker-down docker-smoke

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go test ./...

run-grpc:
	go run ./cmd/grpcserver -csv ./meterusage.csv -addr :9090

run-http:
	go run ./cmd/httpserver -addr :8080 -grpc 127.0.0.1:9090

docker-up:
	docker compose up --build --pull=missing

docker-down:
	docker compose down --remove-orphans

docker-smoke:
	docker compose up -d --build --pull=missing
	@echo "Waiting for HTTP healthz..."
	@sh -c 'i=0; until curl -fsS http://localhost:8080/healthz >/dev/null 2>&1; do i=$$((i+1)); if [ $$i -gt 60 ]; then echo "timeout"; exit 1; fi; sleep 1; done'
	@echo "Fetching a small range..."
	@curl -fsS "http://localhost:8080/api/readings?start=2019-01-01T00:00:00Z&end=2019-01-01T01:00:00Z" >/dev/null
	@echo "OK"
	docker compose down --remove-orphans

