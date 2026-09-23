ALTER TABLE transactions ADD COLUMN IF NOT EXISTS callback_url TEXT;
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS redirect_url TEXT;
