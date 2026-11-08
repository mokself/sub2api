-- [mok] Fork-only table (see FORK.md): last known plan fields of Claude OAuth accounts,
-- read from GET https://api.anthropic.com/api/oauth/profile.
-- No foreign key to accounts, so upstream code and tests that truncate accounts are unaffected.
CREATE TABLE IF NOT EXISTS mok_claude_plan_tiers (
    account_id          BIGINT PRIMARY KEY,
    organization_type   TEXT NOT NULL DEFAULT '',
    rate_limit_tier     TEXT NOT NULL DEFAULT '',
    seat_tier           TEXT NOT NULL DEFAULT '',
    subscription_status TEXT NOT NULL DEFAULT '',
    -- Time of the last successful fetch; NULL until one succeeds.
    fetched_at          TIMESTAMPTZ,
    -- Error of the last attempt; empty when the last attempt succeeded.
    error_reason        TEXT NOT NULL DEFAULT '',
    error_message       TEXT NOT NULL DEFAULT '',
    attempted_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
