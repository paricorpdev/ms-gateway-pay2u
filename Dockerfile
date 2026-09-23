# Builder Stage
FROM golang:1.25-alpine3.21 AS builder

ARG VERSION=dev

ENV CGO_ENABLED=0 \
    GO111MODULE=on

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build binary with -trimpath and injected version
RUN go build \
    -trimpath \
    -ldflags="-s -w -X 'paygate/internal/bootstrap.Version=${VERSION}'" \
    -o bin/paygate \
    ./cmd/api

# Runtime Stage
FROM alpine:3.21

ENV TZ=Asia/Jakarta

# Install ca-certificates & tzdata, and create non-root user
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && \
    adduser -S app -G app

WORKDIR /app

# Copy binary with non-root ownership & restricted execute permissions
COPY --from=builder --chown=app:app --chmod=500 /app/bin/paygate /app/paygate

# Copy default config template and migrations
COPY --from=builder --chown=app:app --chmod=644 /app/config.yml.example /app/config.yml
COPY --from=builder --chown=app:app /app/db/migrations /app/db/migrations

USER app

EXPOSE 8080

ENTRYPOINT ["/app/paygate"]
