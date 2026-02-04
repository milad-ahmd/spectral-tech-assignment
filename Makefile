.PHONY: test fmt vet check run-grpc run-http docker-up docker-down

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

