DROP INDEX IF EXISTS idx_transactions_request_id;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'transactions' AND column_name = 'request_id'
    ) THEN
        ALTER TABLE transactions RENAME COLUMN request_id TO idempotency_key;
    END IF;
END $$;
