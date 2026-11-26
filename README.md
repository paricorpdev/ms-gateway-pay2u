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
| HTTP Server | `web.port` | `8081` | Bind port number |
| HTTP Server | `web.request_timeout` | `30s` | Request deadline enforced by timeout middleware |
| HTTP Server | `web.rate_limit.enabled` | `true` | Rate limiting toggle |
| HTTP Server | `web.rate_limit.max` | `120` | Max requests allowed per expiration window |
| HTTP Server | `web.rate_limit.expiration` | `1m` | Sliding window duration for rate limits |
| Logging | `log.level` | `info` | Minimum log level (`debug`, `info`, `warn`, `error`) |
| Logging | `log.format` | `json` | Log format (`json` or `text`) |
| PostgreSQL | `database.postgresql.host` | `localhost` | Database host |
| PostgreSQL | `database.postgresql.port` | `5432` | Database port |
| PostgreSQL | `database.postgresql.name` | `pay2u` | Database name; must match the Dashboard backend database so per-user dashboard keys can be validated |
| PostgreSQL | `database.postgresql.pool.max` | `100` | Maximum open database connections |
| Redis | `database.redis.host` | `localhost` | Redis host |
| Redis | `database.redis.port` | `6379` | Redis port |
| Provider (Pay2U) | `pay2u.sandbox` | `true` | Sandbox mode switch |
| Provider (Pay2U) | `pay2u.base_url_sandbox` | `https://api-dev.pay2u.co.id` | Pay2U sandbox API root |
| Provider (Pay2U) | `pay2u.base_url_production` | `https://api.pay2u.co.id` | Pay2U production API root |
| Provider (Pay2U) | `pay2u.client_id` | - | OAuth client ID provided by Pay2U |
| Provider (Pay2U) | `pay2u.client_secret` | - | OAuth client secret provided by Pay2U |
| Provider (Pay2U) | `pay2u.merchant_code` | - | Merchant code assigned by Pay2U |
| Provider (Pay2U) | `pay2u.callback_base_url` | `http://localhost:8081` | Public base URL where Pay2U sends payment notifications |

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

Apply the database schema to the PostgreSQL instance configured in `config.yml`:

```bash
go run ./cmd/migrate up
```

To roll back only the latest migration:

```bash
go run ./cmd/migrate down 1
```

The migration runner invokes the `migrate` CLI, which must be installed and available on `PATH`. Install it with `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`, or use the existing `make check-migrate` target.

### Step 3: Seed a Demo Merchant

Set the merchant fields under `seed.merchant` in `config.yml` (defaults are in `config.yml.example`), then run:

```bash
go run ./cmd/seed
```

The seeder creates the merchant only if its code does not already exist. It generates an API key and webhook secret on first creation and prints them once; re-running it leaves existing credentials unchanged.

### Step 4: Start the Service

Run the API service:

```bash
make run-api
```

The server starts on `http://0.0.0.0:8080`.

### Step 5: Verify Service Health

Check service liveness:

```bash
curl -i http://localhost:8081/health/live
```

Check database and Redis readiness:

```bash
curl -i http://localhost:8081/health/ready
```

Access OpenAPI documentation in your browser:

```text
http://localhost:8081/api/v1/docs/index.html
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

## Merchant Administration API

Dashboard merchant reads and writes use these MS endpoints. Requests require the signed-in dashboard user's `X-API-Key`, stored in `users.ms_master_key` in the shared PostgreSQL database. Configure `database.postgresql` in both services to point to that same database; there is no global master key in MS configuration. When an Admin creates a merchant, MS derives `admin_id` from the authenticated key and stores the foreign key to `users.id`. Admin keys can list and change only their own merchants; Super Admin keys are read-only and can view all merchants. Existing merchants remain unassigned (`admin_id` is null) because their original owner is unknown.

| Method | Endpoint | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/admin/merchants/` | Create a merchant and issue its API key and webhook secret |
| `GET` | `/api/v1/admin/merchants/?page=1&per_page=10` | List merchants; supports `search`, `is_active`, `sort_by`, and `sort_dir` |
| `GET` | `/api/v1/admin/merchants/:uuid` | Read merchant details |
| `PUT` | `/api/v1/admin/merchants/:uuid` | Update merchant name, webhook URL, or active state |
| `POST` | `/api/v1/admin/merchants/:uuid/rotate-key` | Rotate the merchant API key |
| `POST` | `/api/v1/admin/merchants/:uuid/rotate-webhook-secret` | Rotate the webhook secret |

The Dashboard backend stores the MS base URL under `ms_gateway.base_url` in its own `config.yml`. Credential rotation remains available only through this MS administration API.

Apply Dashboard migrations first, then apply MS migrations so the shared `users` table exists before migration `000010_add_admin_id_to_merchants` creates its foreign key. The `admin_id` column is nullable for historical merchants whose owner cannot be determined; all merchants created through Dashboard receive an Admin owner.

## Disbursement API

Disbursements are linked to one successful payment transaction owned by the authenticated merchant. The `transaction_id` is required because the existing schema records the source payment on each disbursement. The gateway calculates `amount_total = amount + amount_admin - amount_discount`, and rejects a total greater than the source transaction total.

Payment and disbursement endpoints require the merchant's `X-API-Key`. Merchant administration endpoints require the per-user dashboard key stored on the dashboard user record. The Pay2U callback endpoint is public and is correlated using the provider token and `merchant_reff`.

### 1. Account inquiry

`POST /api/v1/disbursements/inquiry`

```json
{
  "transaction_id": "3f88f847-7237-42d7-9d52-4383a9558701",
  "merchant_reff": "payout-2026-0001",
  "channel_code": "TF-MITRA",
  "url_redirect_merchant": "https://merchant.example.com/payouts/return",
  "receiver_account_number": "1234567890",
  "receiver_name": "Recipient Name",
  "receiver_bank": "014",
  "amount": 100000,
  "amount_admin": 2500,
  "amount_discount": 0,
  "amount_total": 102500,
  "note": "Payout order 2026-0001",
  "expired_minutes": 30
}
```

`amount_total` must equal `amount + amount_admin - amount_discount`. `channel_code` defaults to `TF-MITRA`, and `expired_minutes` defaults to 30. The gateway always sends Pay2U the static callback URL `{pay2u.callback_base_url}/api/v1/disbursements/callback/pay2u`; callers do not choose this URL. The legacy `url_listener_merchant` input is ignored. After validating a Pay2U callback, the gateway sends the final notification to the merchant's configured `webhook_url`.

The inquiry response contains the disbursement record in `WAITING_SETTLEMENT`. The Pay2U provider token stays inside the gateway and is reused for transfer.

### 2. Submit bank transfer

`POST /api/v1/disbursements/{merchant_reff}/transfer`

No body is required. The gateway submits the saved inquiry details once and returns `PROCESSING` while it waits for Pay2U's transfer notification. Repeating this call will not submit a second transfer.

### 3. Check status

`GET /api/v1/disbursements/{merchant_reff}`

Returns the latest disbursement status and provider reference number when available.

### Pay2U transfer callback

Pay2U calls `POST /api/v1/disbursements/callback/pay2u`. The gateway validates the token/reference pair and checks callback details such as channel, beneficiary, and any supplied amount fields against the saved inquiry. For success notifications, it also checks `amount_paid` against the saved `amount_total`. It stores the callback payload and updates the status, transfer date, and provider reference, then sends the final notification to the merchant's configured `webhook_url`, with `X-Paygate-Signature` when the merchant has a webhook secret.

Configure the Pay2U paths in `config.yml` under `pay2u.transfer_bank_inquiry_url` and `pay2u.transfer_bank_url`, and set `pay2u.callback_base_url` to a public base URL for this service. Apply the new `000009_create_disbursements_table` migration before starting the new endpoints:

```sh
go run ./cmd/migrate up
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
