FROM golang:1.25-bookworm AS builder
WORKDIR /build

COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN go build -v -o /build/main ./examples/visualizer/main.go

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /build/main .

WORKDIR /root/

CMD ["/app/main"]