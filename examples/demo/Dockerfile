FROM golang:1.25-bookworm AS builder
WORKDIR /app

# Dependency Cache Layer 
COPY go.mod go.sum* ./
RUN go mod download

# Build Layer
COPY . .
RUN go build -v -o /app/main ./examples/demo/main.go

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /root/

COPY --from=builder /app/main .
COPY --from=builder /app/examples/demo/pastaay.yaml .

# 4317 OTLP, 2112 Metrics, 8080 API
EXPOSE 8080 2112 50051 4317
CMD ["./main"]