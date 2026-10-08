-- Drop old global unique constraint and index on merchant_reff
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_merchant_reff_key;
DROP INDEX IF EXISTS idx_transactions_merchant_reff;

-- Add composite unique constraint and index for multi-tenant isolation
ALTER TABLE transactions ADD CONSTRAINT uq_transactions_merchant_id_reff UNIQUE (merchant_id, merchant_reff);
CREATE INDEX IF NOT EXISTS idx_transactions_merchant_id_reff ON transactions(merchant_id, merchant_reff);
