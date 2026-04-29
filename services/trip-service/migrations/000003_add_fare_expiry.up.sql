ALTER TABLE ride_fares ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
UPDATE ride_fares SET expires_at = created_at + interval '5 minutes' WHERE expires_at IS NULL;
ALTER TABLE ride_fares ALTER COLUMN expires_at SET NOT NULL;
