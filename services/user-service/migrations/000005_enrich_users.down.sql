DROP TABLE IF EXISTS saved_places;

ALTER TABLE users
    DROP COLUMN IF EXISTS stripe_customer_id,
    DROP COLUMN IF EXISTS total_trips,
    DROP COLUMN IF EXISTS average_rating,
    DROP COLUMN IF EXISTS last_login_at,
    DROP COLUMN IF EXISTS updated_at;
