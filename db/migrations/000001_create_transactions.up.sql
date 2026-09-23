CREATE TABLE IF NOT EXISTS transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id      VARCHAR(255) UNIQUE NOT NULL,
    provider        VARCHAR(50)  NOT NULL DEFAULT 'pay2u',
    provider_token  VARCHAR(255),
    merchant_reff   VARCHAR(255) UNIQUE NOT NULL,
    payment_method  VARCHAR(50)  NOT NULL,
    payment_code    TEXT,
    status          VARCHAR(20)  NOT NULL DEFAULT 'PENDING',
    amount          BIGINT       NOT NULL,
    amount_admin    BIGINT       NOT NULL DEFAULT 0,
    amount_discount BIGINT       NOT NULL DEFAULT 0,
    amount_total    BIGINT       NOT NULL,
    currency        VARCHAR(10)  NOT NULL DEFAULT 'IDR',
    customer_name   VARCHAR(255),
    customer_phone  VARCHAR(50),
    customer_email  VARCHAR(255),
    bill_title      VARCHAR(255),
    bill_description TEXT,
    callback_url    TEXT,
    redirect_url    TEXT,
    provider_callback JSONB,
    payment_reff    VARCHAR(255),
    expired_at      TIMESTAMPTZ,
    paid_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_provider_token ON transactions(provider_token);
CREATE INDEX idx_transactions_merchant_reff ON transactions(merchant_reff);
CREATE INDEX idx_transactions_request_id ON transactions(request_id);

