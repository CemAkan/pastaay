# Stage 1: Builder
FROM golang:1.21-alpine AS builder
WORKDIR /app
# Copy all project files
COPY . .
# Download dependencies and build the application
RUN go mod download
RUN go build -o pastaay-demo examples/demo/main.go

# Stage 2: Runner - Keep it minimal
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/pastaay-demo .
# Copy the configuration file
COPY --from=builder /app/pastaay.yaml .

# Expose application and metrics ports
EXPOSE 8080 2112

# Start the application
CMD ["./pastaay-demo"]