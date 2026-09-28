-- Partial index to speed up scanning for pending transactions that have passed expired_at
CREATE INDEX IF NOT EXISTS idx_transactions_pending_expired 
ON transactions (expired_at) 
WHERE status = 'PENDING';

-- Index on webhook_dispatch_logs timestamp for retention cleanup efficiency
CREATE INDEX IF NOT EXISTS idx_webhook_dispatch_logs_dispatched_at 
ON webhook_dispatch_logs (dispatched_at);
