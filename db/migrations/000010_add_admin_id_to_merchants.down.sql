DROP INDEX IF EXISTS idx_merchants_admin_id;

ALTER TABLE merchants
    DROP CONSTRAINT IF EXISTS fk_merchants_admin_id,
    DROP COLUMN IF EXISTS admin_id;
