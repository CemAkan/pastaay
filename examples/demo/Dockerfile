FROM golang:1.26-bookworm AS builder
WORKDIR /app

COPY go.mod go.sum ./
COPY vendor/ ./vendor/
COPY docs/ ./docs/
COPY pkg/ ./pkg/
COPY web/ ./web/
COPY examples/demo/ ./examples/demo/

RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -v -ldflags="-s -w" -o /app/main ./examples/demo/main.go

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /root/

COPY --from=builder /app/main .
COPY --from=builder /app/examples/demo/pastaay.yaml .

EXPOSE 8080 2112
CMD ["./main"]