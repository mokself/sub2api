-- [mok] Fork-only tables (see FORK.md): affiliate usage rebates are settled per period
-- (an hour) instead of per day. These tables replace the ones of mok_002.
-- No foreign keys, so upstream code and tests that delete users are unaffected.

-- One row per invitee and period. The unique key makes every settlement happen exactly once;
-- the row is written in the same transaction as the rebate it books. day is the date of
-- period_start in the server time zone.
CREATE TABLE IF NOT EXISTS mok_affiliate_usage_rebate_items (
    id            BIGSERIAL PRIMARY KEY,
    period_start  TIMESTAMPTZ NOT NULL,
    period_end    TIMESTAMPTZ NOT NULL,
    day           DATE NOT NULL,
    invitee_id    BIGINT NOT NULL,
    inviter_id    BIGINT NOT NULL,
    usage_amount  DECIMAL(20,10) NOT NULL,
    rate_percent  DECIMAL(10,4) NOT NULL,
    rebate_amount DECIMAL(20,8) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT mok_affiliate_usage_rebate_items_period_invitee UNIQUE (period_start, invitee_id)
);

CREATE INDEX IF NOT EXISTS mok_idx_affiliate_usage_rebate_items_inviter
    ON mok_affiliate_usage_rebate_items (inviter_id, period_start DESC);

CREATE INDEX IF NOT EXISTS mok_idx_affiliate_usage_rebate_items_day
    ON mok_affiliate_usage_rebate_items (day);

-- One row per settled period, written after all of its invitees are settled. Periods are
-- closed in order, so the latest period_end is how far usage has been settled.
-- skipped_reason is non-empty when the period was closed without rebates.
CREATE TABLE IF NOT EXISTS mok_affiliate_usage_rebate_periods (
    period_start   TIMESTAMPTZ PRIMARY KEY,
    period_end     TIMESTAMPTZ NOT NULL,
    day            DATE NOT NULL,
    invitee_count  INT NOT NULL DEFAULT 0,
    usage_total    DECIMAL(20,10) NOT NULL DEFAULT 0,
    rebate_total   DECIMAL(20,8) NOT NULL DEFAULT 0,
    skipped_reason TEXT NOT NULL DEFAULT '',
    settled_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS mok_idx_affiliate_usage_rebate_periods_day
    ON mok_affiliate_usage_rebate_periods (day);

-- Carry the closed days of mok_002 over as periods that span their settled window.
INSERT INTO mok_affiliate_usage_rebate_periods
    (period_start, period_end, day, invitee_count, usage_total, rebate_total, skipped_reason, settled_at)
SELECT window_start, window_end, day, invitee_count, usage_total, rebate_total, skipped_reason, settled_at
FROM mok_affiliate_usage_rebate_days
ON CONFLICT (period_start) DO NOTHING;

INSERT INTO mok_affiliate_usage_rebate_items
    (period_start, period_end, day, invitee_id, inviter_id, usage_amount, rate_percent, rebate_amount, created_at)
SELECT d.window_start, d.window_end, r.day, r.invitee_id, r.inviter_id, r.usage_amount, r.rate_percent, r.rebate_amount, r.created_at
FROM mok_affiliate_usage_rebates r
JOIN mok_affiliate_usage_rebate_days d ON d.day = r.day
ON CONFLICT (period_start, invitee_id) DO NOTHING;

-- mok_affiliate_usage_rebates keeps only the rows of a day that was partly settled and not
-- closed when this migration ran (normally none). Those invitees were paid for that whole
-- day, so the hourly settlement skips them for that day (see MokSettleUsageRebate).
DELETE FROM mok_affiliate_usage_rebates r
USING mok_affiliate_usage_rebate_days d
WHERE d.day = r.day;

-- Dropped so that an older image started after this migration fails to settle instead of
-- paying days again that were already settled per hour.
DROP TABLE mok_affiliate_usage_rebate_days;
