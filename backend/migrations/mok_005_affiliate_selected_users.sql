-- [mok] Fork-only table (see FORK.md): the users the affiliate feature is switched on for.
-- It only matters while the scope (settings key mok_affiliate_scope) is "selected"; with
-- the default scope "all" the feature is on for everyone, as upstream has it.
-- No foreign key, so upstream code and tests that delete users are unaffected.
CREATE TABLE IF NOT EXISTS mok_affiliate_selected_users (
    user_id    BIGINT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
