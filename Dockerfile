FROM golang:1.25 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/grpcserver ./cmd/grpcserver
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/httpserver ./cmd/httpserver


FROM alpine:3.21 AS grpc
WORKDIR /app
COPY --from=build /out/grpcserver /app/grpcserver
COPY meterusage.csv /app/meterusage.csv
ENV GRPC_ADDR=:9090
ENV CSV_PATH=/app/meterusage.csv
EXPOSE 9090
ENTRYPOINT ["/app/grpcserver"]


FROM alpine:3.21 AS http
WORKDIR /app
COPY --from=build /out/httpserver /app/httpserver
ENV HTTP_ADDR=:8080
ENV GRPC_TARGET=grpc:9090
EXPOSE 8080
ENTRYPOINT ["/app/httpserver"]

