CREATE TABLE IF NOT EXISTS inbound_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id      VARCHAR(64) NOT NULL,
    transaction_id  UUID,
    source_service  VARCHAR(100),
    endpoint        TEXT NOT NULL,
    method          VARCHAR(10) NOT NULL,
    headers         JSONB,
    request_payload JSONB,
    response_status INT NOT NULL,
    response_payload JSONB,
    latency_ms      BIGINT NOT NULL DEFAULT 0,
    ip_address      VARCHAR(50),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_inbound_requests_request_id ON inbound_requests(request_id);
CREATE INDEX IF NOT EXISTS idx_inbound_requests_transaction_id ON inbound_requests(transaction_id);
CREATE INDEX IF NOT EXISTS idx_inbound_requests_created_at ON inbound_requests(created_at);

CREATE TABLE IF NOT EXISTS outbound_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id      VARCHAR(64) NOT NULL,
    transaction_id  UUID,
    provider        VARCHAR(50) NOT NULL,
    request_type    VARCHAR(50) NOT NULL,
    attempt         INT NOT NULL DEFAULT 1,
    endpoint        TEXT NOT NULL,
    method          VARCHAR(10) NOT NULL,
    headers         JSONB,
    request_payload JSONB,
    response_status INT NOT NULL,
    response_payload JSONB,
    latency_ms      BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_outbound_requests_request_id ON outbound_requests(request_id);
CREATE INDEX IF NOT EXISTS idx_outbound_requests_transaction_id ON outbound_requests(transaction_id);
CREATE INDEX IF NOT EXISTS idx_outbound_requests_created_at ON outbound_requests(created_at);
