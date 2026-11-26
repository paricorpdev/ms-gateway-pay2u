ALTER TABLE merchants
    ADD COLUMN IF NOT EXISTS admin_id UUID;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_merchants_admin_id'
          AND conrelid = 'merchants'::regclass
    ) THEN
        ALTER TABLE merchants
            ADD CONSTRAINT fk_merchants_admin_id
            FOREIGN KEY (admin_id)
            REFERENCES users(id)
            ON DELETE RESTRICT;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_merchants_admin_id ON merchants(admin_id);
