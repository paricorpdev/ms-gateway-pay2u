CREATE TABLE IF NOT EXISTS disbursements (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id           UUID NOT NULL UNIQUE REFERENCES transactions(id),
    merchant_id              UUID NOT NULL REFERENCES merchants(id),
    request_id               VARCHAR(255) NOT NULL UNIQUE,
    merchant_ref             VARCHAR(255) NOT NULL,
    provider                 VARCHAR(50) NOT NULL DEFAULT 'pay2u',
    provider_token           VARCHAR(255),
    provider_disbursement_ref VARCHAR(255) UNIQUE,
    channel_code             VARCHAR(50) NOT NULL,
    url_listener_merchant    TEXT NOT NULL,
    url_redirect_merchant    TEXT,
    expired_minutes          INT NOT NULL DEFAULT 30 CHECK (expired_minutes BETWEEN 1 AND 1440),
    amount                   BIGINT NOT NULL CHECK (amount > 0),
    amount_admin             BIGINT NOT NULL DEFAULT 0 CHECK (amount_admin >= 0),
    amount_discount          BIGINT NOT NULL DEFAULT 0 CHECK (amount_discount >= 0),
    amount_total             BIGINT NOT NULL CHECK (amount_total > 0 AND amount_total = amount + amount_admin - amount_discount),
    currency                 VARCHAR(10) NOT NULL DEFAULT 'IDR',
    beneficiary_name         VARCHAR(255) NOT NULL,
    beneficiary_bank_code    VARCHAR(50) NOT NULL,
    beneficiary_bank_name    VARCHAR(150),
    beneficiary_account_ref  VARCHAR(255) NOT NULL,
    note                     TEXT,
    status                   VARCHAR(30) NOT NULL DEFAULT 'WAITING_SETTLEMENT',
    eligible_at              TIMESTAMPTZ,
    inquiry_at               TIMESTAMPTZ,
    transferred_at           TIMESTAMPTZ,
    attempts                 INT NOT NULL DEFAULT 0,
    max_attempts             INT NOT NULL DEFAULT 3,
    next_retry_at            TIMESTAMPTZ,
    locked_until             TIMESTAMPTZ,
    locked_by                VARCHAR(100),
    provider_response        JSONB,
    provider_callback        JSONB,
    paid_at                  TIMESTAMPTZ,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_disbursements_merchant_ref UNIQUE (merchant_id, merchant_ref)
);

CREATE INDEX IF NOT EXISTS idx_disbursements_status_eligible_at
    ON disbursements (status, eligible_at);
CREATE INDEX IF NOT EXISTS idx_disbursements_next_retry_at
    ON disbursements (next_retry_at);
CREATE INDEX IF NOT EXISTS idx_disbursements_locked_until
    ON disbursements (locked_until);
