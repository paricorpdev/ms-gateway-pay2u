# Build stage
FROM golang:1.24.0-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache make git tzdata

# Copy dependency files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN make build

# Runtime stage
FROM alpine:3.21

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata \
    && cp /usr/share/zoneinfo/Asia/Jakarta /etc/localtime \
    && echo "Asia/Jakarta" > /etc/timezone

# Copy binary from builder
COPY --from=builder /app/bin/paygate .
COPY --from=builder /app/config.yml.example ./config.yml

# Copy migrations
COPY --from=builder /app/db/migrations ./db/migrations

# Expose port
EXPOSE 8080

# Run the binary
CMD ["./paygate"]
