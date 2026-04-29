CREATE TABLE payments (
    id                BIGSERIAL    PRIMARY KEY,
    trip_id           TEXT         NOT NULL,
    user_id           BIGINT       NOT NULL,
    driver_id         TEXT         NOT NULL DEFAULT '',
    amount_cents      BIGINT       NOT NULL,
    currency          TEXT         NOT NULL DEFAULT 'eur',
    status            TEXT         NOT NULL DEFAULT 'pending',
    stripe_session_id TEXT         NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    paid_at           TIMESTAMPTZ
);

CREATE INDEX idx_payments_trip_id ON payments (trip_id);
CREATE INDEX idx_payments_user_id ON payments (user_id);
CREATE INDEX idx_payments_stripe_session_id ON payments (stripe_session_id);
