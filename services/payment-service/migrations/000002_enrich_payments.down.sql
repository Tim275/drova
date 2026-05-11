DROP INDEX IF EXISTS idx_payments_stripe_session_unique;

ALTER TABLE payments
    DROP COLUMN IF EXISTS stripe_payment_intent_id,
    DROP COLUMN IF EXISTS driver_payout_cents,
    DROP COLUMN IF EXISTS platform_fee_cents,
    DROP COLUMN IF EXISTS refund_id,
    DROP COLUMN IF EXISTS refunded_at;
