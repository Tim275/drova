CREATE TABLE drivers (
    id              TEXT        PRIMARY KEY,
    name            TEXT        NOT NULL,
    package_slug    TEXT        NOT NULL,
    car_plate       TEXT        NOT NULL,
    profile_picture TEXT        NOT NULL,
    last_seen       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
