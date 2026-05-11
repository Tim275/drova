ALTER TABLE drivers
    ADD COLUMN IF NOT EXISTS stripe_account_id          TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS stripe_onboarding_complete BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS total_trips                INT     NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS average_rating             FLOAT4,
    ADD COLUMN IF NOT EXISTS acceptance_rate            FLOAT4,
    ADD COLUMN IF NOT EXISTS vehicle_model              TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS vehicle_color              TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS is_active                  BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS is_verified                BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS verified_at                TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS license_number             TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS license_expiry             DATE,
    ADD COLUMN IF NOT EXISTS created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW();
