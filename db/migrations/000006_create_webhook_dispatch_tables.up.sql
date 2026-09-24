CREATE TABLE IF NOT EXISTS webhook_dispatches (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id  UUID NOT NULL REFERENCES transactions(id),
    merchant_id     UUID NOT NULL REFERENCES merchants(id),
    target_url      TEXT NOT NULL,
    event_type      VARCHAR(50) NOT NULL DEFAULT 'payment.success',
    payload         JSONB NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    attempts        INT NOT NULL DEFAULT 0,
    max_attempts    INT NOT NULL DEFAULT 3,
    next_retry_at   TIMESTAMPTZ,
    locked_until    TIMESTAMPTZ,
    locked_by       VARCHAR(100),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_dispatches_status_next_retry ON webhook_dispatches(status, next_retry_at);
CREATE INDEX IF NOT EXISTS idx_webhook_dispatches_locked_until ON webhook_dispatches(locked_until);
CREATE INDEX IF NOT EXISTS idx_webhook_dispatches_trx ON webhook_dispatches(transaction_id);
CREATE INDEX IF NOT EXISTS idx_webhook_dispatches_merchant ON webhook_dispatches(merchant_id);

CREATE TABLE IF NOT EXISTS webhook_dispatch_logs (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dispatch_id      UUID NOT NULL REFERENCES webhook_dispatches(id) ON DELETE CASCADE,
    attempt_number   INT NOT NULL,
    http_status      INT,
    response_payload TEXT,
    error_message    TEXT,
    latency_ms       BIGINT,
    dispatched_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_dispatch_logs_dispatch_id ON webhook_dispatch_logs(dispatch_id);
