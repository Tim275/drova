-- Enrich ride_fares: store addresses + denormalized distance/duration
ALTER TABLE ride_fares
    ADD COLUMN IF NOT EXISTS pickup_address  TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS dropoff_address TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS pickup_lat      FLOAT8  NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS pickup_lng      FLOAT8  NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS dropoff_lat     FLOAT8  NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS dropoff_lng     FLOAT8  NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS distance_meters INT     NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS duration_seconds INT    NOT NULL DEFAULT 0;

-- Enrich trips: denormalize key fare fields + audit timestamps
ALTER TABLE trips
    ADD COLUMN IF NOT EXISTS pickup_address  TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS dropoff_address TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS distance_meters INT     NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS duration_seconds INT    NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS package_slug    TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS amount_cents    BIGINT  NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS completed_at   TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cancelled_at   TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cancelled_by   TEXT,
    ADD COLUMN IF NOT EXISTS rider_rating   INT;
