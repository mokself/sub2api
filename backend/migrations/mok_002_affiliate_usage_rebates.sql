-- [mok] Fork-only tables (see FORK.md): daily affiliate rebates on invitees' balance usage.
-- No foreign keys, so upstream code and tests that delete users are unaffected.

-- One row per invitee and day. The primary key makes every settlement happen exactly once;
-- the row is written in the same transaction as the rebate it books.
CREATE TABLE IF NOT EXISTS mok_affiliate_usage_rebates (
    day           DATE NOT NULL,
    invitee_id    BIGINT NOT NULL,
    inviter_id    BIGINT NOT NULL,
    usage_amount  DECIMAL(20,10) NOT NULL,
    rate_percent  DECIMAL(10,4) NOT NULL,
    rebate_amount DECIMAL(20,8) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (day, invitee_id)
);

CREATE INDEX IF NOT EXISTS mok_idx_affiliate_usage_rebates_inviter
    ON mok_affiliate_usage_rebates (inviter_id, day);

-- One row per settled day, written after all of its invitees are settled.
-- skipped_reason is non-empty when the day was closed without rebates.
CREATE TABLE IF NOT EXISTS mok_affiliate_usage_rebate_days (
    day            DATE PRIMARY KEY,
    window_start   TIMESTAMPTZ NOT NULL,
    window_end     TIMESTAMPTZ NOT NULL,
    invitee_count  INT NOT NULL DEFAULT 0,
    usage_total    DECIMAL(20,10) NOT NULL DEFAULT 0,
    rebate_total   DECIMAL(20,8) NOT NULL DEFAULT 0,
    skipped_reason TEXT NOT NULL DEFAULT '',
    settled_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
