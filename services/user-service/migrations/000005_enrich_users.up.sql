ALTER TABLE users
    ADD COLUMN IF NOT EXISTS stripe_customer_id TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS total_trips        INT     NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS average_rating     FLOAT4,
    ADD COLUMN IF NOT EXISTS last_login_at      TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS updated_at         TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS saved_places (
    id         BIGSERIAL    PRIMARY KEY,
    user_id    BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label      TEXT         NOT NULL,
    address    TEXT         NOT NULL,
    lat        FLOAT8       NOT NULL,
    lng        FLOAT8       NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_saved_places_user_id ON saved_places (user_id);
