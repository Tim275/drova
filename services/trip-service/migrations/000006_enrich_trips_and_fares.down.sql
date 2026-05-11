ALTER TABLE ride_fares
    DROP COLUMN IF EXISTS pickup_address,
    DROP COLUMN IF EXISTS dropoff_address,
    DROP COLUMN IF EXISTS pickup_lat,
    DROP COLUMN IF EXISTS pickup_lng,
    DROP COLUMN IF EXISTS dropoff_lat,
    DROP COLUMN IF EXISTS dropoff_lng,
    DROP COLUMN IF EXISTS distance_meters,
    DROP COLUMN IF EXISTS duration_seconds;

ALTER TABLE trips
    DROP COLUMN IF EXISTS pickup_address,
    DROP COLUMN IF EXISTS dropoff_address,
    DROP COLUMN IF EXISTS distance_meters,
    DROP COLUMN IF EXISTS duration_seconds,
    DROP COLUMN IF EXISTS package_slug,
    DROP COLUMN IF EXISTS amount_cents,
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS cancelled_at,
    DROP COLUMN IF EXISTS cancelled_by,
    DROP COLUMN IF EXISTS rider_rating;
