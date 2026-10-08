-- Revert composite unique constraint and restore global unique on merchant_reff
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS uq_transactions_merchant_id_reff;
DROP INDEX IF EXISTS idx_transactions_merchant_id_reff;

ALTER TABLE transactions ADD CONSTRAINT transactions_merchant_reff_key UNIQUE (merchant_reff);
CREATE INDEX IF NOT EXISTS idx_transactions_merchant_reff ON transactions(merchant_reff);
