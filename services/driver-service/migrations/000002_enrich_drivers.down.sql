ALTER TABLE drivers
    DROP COLUMN IF EXISTS stripe_account_id,
    DROP COLUMN IF EXISTS stripe_onboarding_complete,
    DROP COLUMN IF EXISTS total_trips,
    DROP COLUMN IF EXISTS average_rating,
    DROP COLUMN IF EXISTS acceptance_rate,
    DROP COLUMN IF EXISTS vehicle_model,
    DROP COLUMN IF EXISTS vehicle_color,
    DROP COLUMN IF EXISTS is_active,
    DROP COLUMN IF EXISTS is_verified,
    DROP COLUMN IF EXISTS verified_at,
    DROP COLUMN IF EXISTS license_number,
    DROP COLUMN IF EXISTS license_expiry,
    DROP COLUMN IF EXISTS created_at;
