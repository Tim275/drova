CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    email         VARCHAR(255) NOT NULL UNIQUE,
    password_hash BYTEA        NOT NULL,
    role          VARCHAR(20)  NOT NULL DEFAULT 'rider',
    is_activated  BOOLEAN      NOT NULL DEFAULT false,
    created_at    TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_invitations (
    token   VARCHAR(64)  PRIMARY KEY,
    user_id BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expiry  TIMESTAMP(0) WITH TIME ZONE NOT NULL
);
