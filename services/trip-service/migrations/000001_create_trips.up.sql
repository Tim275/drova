CREATE TABLE trips (
    id            TEXT        PRIMARY KEY,
    user_id       TEXT        NOT NULL,
    status        TEXT        NOT NULL DEFAULT 'searching',
    driver_id     TEXT,
    driver_name   TEXT,
    driver_plate  TEXT,
    driver_avatar TEXT,
    rating        INT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ride_fares (
    id                TEXT        PRIMARY KEY,
    user_id           TEXT        NOT NULL,
    package_slug      TEXT        NOT NULL,
    total_price_cents FLOAT8      NOT NULL,
    route             JSONB,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_trips_user_id      ON trips (user_id);
CREATE INDEX idx_ride_fares_user_id ON ride_fares (user_id);
