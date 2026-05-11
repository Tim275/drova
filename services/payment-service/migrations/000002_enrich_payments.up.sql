ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS stripe_payment_intent_id TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS driver_payout_cents       BIGINT  NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS platform_fee_cents        BIGINT  NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS refund_id                 TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS refunded_at               TIMESTAMPTZ;

-- Prevent double-charge on Kafka redelivery
CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_stripe_session_unique
    ON payments (stripe_session_id)
    WHERE stripe_session_id <> '';
