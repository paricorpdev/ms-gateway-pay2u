# Paygate Service

Payment gateway microservice integrating internal applications with external payment providers. The service processes Virtual Account (VA), QRIS, and Credit Card (CC) transactions via Pay2U, tracks transaction states, ingests callbacks, and records asynchronous audit logs.

## Tech Stack

- Programming language: Go 1.25
- HTTP framework: Fiber v2 (`github.com/gofiber/fiber/v2`)
- Database ORM: GORM (`gorm.io/gorm`) with PostgreSQL driver (`gorm.io/driver/postgres`)
- Caching: Redis (`github.com/redis/go-redis/v9`) and Fiber Redis storage (`github.com/gofiber/storage/redis/v3`)
- Configuration management: Viper (`github.com/spf13/viper`)
- Request validation: Go Playground Validator v10 (`github.com/go-playground/validator/v10`)
- Structured logging: Logrus (`github.com/sirupsen/logrus`)
- API documentation: Swagger / OpenAPI via Swag (`github.com/swaggo/swag`, `github.com/gofiber/swagger`)
- Database migrations: golang-migrate (`github.com/golang-migrate/migrate/v4`)

## Layout

```text
ms-paygate/
├── cmd/
│   └── api/
│       └── main.go                # Service entrypoint and graceful shutdown setup
├── config.yml.example             # Configuration file template
├── Makefile                       # Build, run, test, and migration scripts
├── Dockerfile                     # Multi-stage Docker container build
├── db/
│   └── migrations/                # Schema migration SQL files
└── internal/
    ├── audit/                     # Non-blocking audit queue and worker
    ├── bootstrap/                 # Dependency injection and HTTP router setup
    ├── common/                    # Constants and key generators
    ├── config/                    # Config loaders (Viper, DB, Redis, Fiber, Logrus)
    ├── delivery/
    │   └── http/
    │       ├── controller/        # HTTP controllers (Payment, Merchant, Health)
    │       ├── middleware/        # Middlewares (API Key, Audit, Request ID, Timeout)
    │       ├── response/          # Response helper and error handler
    │       └── route/             # Route groups and route registration
    ├── entity/                    # GORM entity models and types
    ├── exception/                 # Application error types
    ├── model/
    │   ├── converter/             # Entity to response DTO mappers
    │   └── payload/               # Request, response, and pagination structs
    ├── provider/
    │   └── pay2u/                 # Pay2U client, token manager, and billing adapters
    ├── repository/                # Data access layer for PostgreSQL and Redis
    └── usecase/                   # Business logic layer
```

## Configuration

The service reads configuration values from `config.yml`. Create this file using `config.yml.example` as a baseline.

You can override any YAML setting using environment variables prefixed with `APP_`. Replace dot separators with underscores. For example, `database.postgresql.host` becomes `APP_DATABASE_POSTGRESQL_HOST`.

You can also specify an explicit configuration path via `CONFIG_FILE` (absolute path to a file) or `CONFIG_PATH` (directory containing `config.yml`).

### Core Settings Reference

| Category | Key | Default | Description |
| --- | --- | --- | --- |
| Application | `app.env` | `development` | Runtime environment (`development`, `staging`, `production`) |
| Application | `app.timezone` | `Asia/Jakarta` | Local server timezone |
| Application | `app.shutdown_timeout` | `15s` | Maximum time allowed for graceful shutdown |
| HTTP Server | `web.host` | `0.0.0.0` | Bind IP address |
| HTTP Server | `web.port` | `8080` | Bind port number |
| HTTP Server | `web.request_timeout` | `30s` | Request deadline enforced by timeout middleware |
| HTTP Server | `web.rate_limit.enabled` | `true` | Rate limiting toggle |
| HTTP Server | `web.rate_limit.max` | `120` | Max requests allowed per expiration window |
| HTTP Server | `web.rate_limit.expiration` | `1m` | Sliding window duration for rate limits |
| Logging | `log.level` | `info` | Minimum log level (`debug`, `info`, `warn`, `error`) |
| Logging | `log.format` | `json` | Log format (`json` or `text`) |
| PostgreSQL | `database.postgresql.host` | `localhost` | Database host |
| PostgreSQL | `database.postgresql.port` | `5432` | Database port |
| PostgreSQL | `database.postgresql.name` | `paygate` | Database name |
| PostgreSQL | `database.postgresql.pool.max` | `100` | Maximum open database connections |
| Redis | `database.redis.host` | `localhost` | Redis host |
| Redis | `database.redis.port` | `6379` | Redis port |
| Security | `api_key` | `dev-api-key-change-me` | Master API key used to access admin endpoints |
| Provider (Pay2U) | `pay2u.sandbox` | `true` | Sandbox mode switch |
| Provider (Pay2U) | `pay2u.base_url_sandbox` | `https://api-dev.pay2u.co.id` | Pay2U sandbox API root |
| Provider (Pay2U) | `pay2u.base_url_production` | `https://api.pay2u.co.id` | Pay2U production API root |
| Provider (Pay2U) | `pay2u.client_id` | - | OAuth client ID provided by Pay2U |
| Provider (Pay2U) | `pay2u.client_secret` | - | OAuth client secret provided by Pay2U |
| Provider (Pay2U) | `pay2u.merchant_code` | - | Merchant code assigned by Pay2U |
| Provider (Pay2U) | `pay2u.callback_base_url` | `http://localhost:8080` | Public base URL where Pay2U sends payment notifications |

## Quick Start

### Prerequisites

- Go 1.25 or newer
- PostgreSQL 14 or newer
- Redis 6 or newer
- `golang-migrate` CLI (installed automatically by the Makefile if missing)
- `swag` CLI (optional, needed only when updating OpenAPI docs)

### Step 1: Create Configuration

Generate your local configuration file:

```bash
make config
```

Open `config.yml` and enter your PostgreSQL credentials, Redis connection settings, and Pay2U merchant keys.

### Step 2: Run Database Migrations

Apply the database schema to your PostgreSQL instance:

```bash
make migrate-up DATABASE_URL="postgresql://postgres:password@localhost:5432/paygate?sslmode=disable"
```

To roll back migrations:

```bash
make migrate-down DATABASE_URL="postgresql://postgres:password@localhost:5432/paygate?sslmode=disable"
```

### Step 3: Start the Service

Run the API service:

```bash
make run-api
```

The server starts on `http://0.0.0.0:8080`.

### Step 4: Verify Service Health

Check service liveness:

```bash
curl -i http://localhost:8080/health/live
```

Check database and Redis readiness:

```bash
curl -i http://localhost:8080/health/ready
```

Access OpenAPI documentation in your browser:

```text
http://localhost:8080/api/v1/docs/index.html
```

### Additional Make Targets

- `make build`: Compiles the binary to `bin/paygate`.
- `make test`: Runs unit tests with race detection enabled.
- `make cover`: Runs test coverage and generates an HTML report.
- `make lint`: Runs `golangci-lint` to check code style and errors.
- `make swagger`: Recompiles Swagger documentation from Go comments.

## Request Flow

```text
Client Application / Webhook Caller
                │
                ▼
┌────────────────────────────────────────┐
│ Fiber Middleware Pipeline              │
│ 1. Recover                             │
│ 2. Request ID (X-Request-Id or UUID)   │
│ 3. CORS & Helmet Security Headers      │
│ 4. Compression                         │
│ 5. Audit Middleware (Inbound Capture)  │
│ 6. Access Logger                       │
│ 7. Rate Limiter (Skipped on /health)   │
│ 8. Request Timeout Context             │
└──────────────────┬─────────────────────┘
                   │
                   ▼
┌────────────────────────────────────────┐
│ Authentication Check                   │
│ • Public: /health, /docs, /callback    │
│ • Admin: Master key check              │
│ • Payment: Merchant key (Redis -> DB)  │
└──────────────────┬─────────────────────┘
                   │
                   ▼
┌────────────────────────────────────────┐
│ Controller Layer                       │
│ Parse JSON body and URL parameters     │
└──────────────────┬─────────────────────┘
                   │
                   ▼
┌────────────────────────────────────────┐
│ Usecase Layer                          │
│ 1. Struct validation (validator/v10)   │
│ 2. Request ID lookup (Postgres)        │
│    (Return stored record if replay)    │
└──────────────────┬─────────────────────┘
                   │
                   ▼
┌────────────────────────────────────────┐
│ Provider Integration (Pay2U)           │
│ 1. Fetch OAuth token (Redis or API)    │
│ 2. Request transaction token           │
│ 3. Submit billing (VA / QRIS / CC)     │
│ 4. Log outbound call via AuditWorker   │
└──────────────────┬─────────────────────┘
                   │
                   ▼
┌────────────────────────────────────────┐
│ Database Persistence                   │
│ Save transaction record (PENDING)      │
└──────────────────┬─────────────────────┘
                   │
                   ▼
┌────────────────────────────────────────┐
│ Response Writer                        │
│ Format standardized response envelope  │
└──────────────────┬─────────────────────┘
                   │
                   ▼ (Background)
┌────────────────────────────────────────┐
│ Audit Worker (Buffered Channel)        │
│ Asynchronously inserts into Postgres:  │
│ • inbound_requests                     │
│ • outbound_requests                    │
└────────────────────────────────────────┘
```

## Response Format

All API responses follow a uniform JSON structure.

### Success Response

```json
{
  "status": "success",
  "status_code": 200,
  "response_data": {},
  "request_id": "a90ef59a-5f33-4df4-8d48-3162788c93a0"
}
```

### Paginated Response

```json
{
  "status": "success",
  "status_code": 200,
  "response_data": {
    "data": [],
    "paging": {
      "page": 1,
      "per_page": 10,
      "total_data": 0,
      "total_page": 0
    }
  },
  "request_id": "0d6199fc-8b29-4d6d-88b9-e16e451b54bb"
}
```

### Error Response

```json
{
  "status": "error",
  "status_code": 400,
  "error": {
    "code": "BAD_REQUEST",
    "message": "validation error",
    "details": null,
    "path": "/api/v1/payments",
    "timestamp": "2026-09-23T02:46:20Z"
  },
  "request_id": "c7117180-b2be-4972-882d-450f383e20ec"
}
```

### Error Codes

| HTTP Status | Error Code | Description |
| --- | --- | --- |
| 400 | `BAD_REQUEST` | Validation error, malformed JSON body, or missing required parameter |
| 401 | `UNAUTHORIZED` | Missing or invalid `X-API-Key` |
| 403 | `FORBIDDEN` | The merchant account is deactivated or lacks permissions |
| 404 | `NOT_FOUND` | The requested route, merchant, or transaction ID does not exist |
| 409 | `CONFLICT` | Resource uniqueness collision |
| 413 | `TOO_LARGE` | Request payload exceeds `web.body_limit` |
| 429 | `RATE_LIMITED` | The client exceeded the allowed request quota |
| 499 | `BAD_REQUEST` | Client closed the connection before the server finished processing |
| 500 | `INTERNAL` | Unexpected server failure |
| 503 | `UNAVAILABLE` | Database or downstream dependency is unreachable |
| 504 | `TIMEOUT` | Request deadline expired before completing |
