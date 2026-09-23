DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'transactions' AND column_name = 'idempotency_key'
    ) THEN
        ALTER TABLE transactions RENAME COLUMN idempotency_key TO request_id;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_transactions_request_id ON transactions(request_id);
