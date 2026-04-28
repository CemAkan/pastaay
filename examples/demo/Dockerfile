FROM golang:1.23-alpine AS builder
WORKDIR /app

COPY . .
RUN go mod download
RUN go build -o pastaay-demo examples/demo/main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/pastaay-demo .
COPY --from=builder /app/examples/demo/pastaay.yaml .

EXPOSE 8080 2112
CMD ["./pastaay-demo"]