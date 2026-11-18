-- [mok] Fork-only tables (see FORK.md): SMS notifications through Tencent Cloud.
-- The settings (credentials, templates, switches, admin alert phones) are stored in the
-- settings table under the key mok_sms_config. No foreign keys, so upstream code and tests
-- that delete users are unaffected.

-- The one phone number a user receives SMS reminders on. Only verified numbers are stored.
CREATE TABLE IF NOT EXISTS mok_sms_phones (
    user_id     BIGINT PRIMARY KEY,
    phone       VARCHAR(20) NOT NULL, -- mainland China mobile number, 11 digits without +86
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    verified_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The verification code a user is binding a phone number with; one pending code per user.
CREATE TABLE IF NOT EXISTS mok_sms_codes (
    user_id    BIGINT PRIMARY KEY,
    phone      VARCHAR(20) NOT NULL,
    code_hash  VARCHAR(64) NOT NULL,
    attempts   INTEGER NOT NULL DEFAULT 0,
    sent_at    TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

-- Every SMS sent or attempted. A row is written before the provider is called, so the
-- partial unique index stops a deduplicated notice from going out twice, also across
-- instances. status: pending, sent, failed, dry_run (logged, not sent) or limited (the
-- daily cap was reached). Verification codes are not stored, except in dry-run mode.
CREATE TABLE IF NOT EXISTS mok_sms_logs (
    id          BIGSERIAL PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    event       VARCHAR(32) NOT NULL,
    user_id     BIGINT, -- NULL for admin alerts and test messages
    phone       VARCHAR(20) NOT NULL,
    dedup_key   VARCHAR(128),
    template_id VARCHAR(32) NOT NULL DEFAULT '',
    params      JSONB NOT NULL DEFAULT '[]'::jsonb,
    status      VARCHAR(16) NOT NULL,
    error       TEXT NOT NULL DEFAULT '',
    request_id  VARCHAR(64) NOT NULL DEFAULT '',
    finished_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS mok_sms_logs_dedup_idx
    ON mok_sms_logs (event, dedup_key, phone) WHERE dedup_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS mok_sms_logs_user_created_idx
    ON mok_sms_logs (user_id, created_at) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS mok_sms_logs_phone_created_idx
    ON mok_sms_logs (phone, created_at);
CREATE INDEX IF NOT EXISTS mok_sms_logs_created_idx
    ON mok_sms_logs (created_at);
