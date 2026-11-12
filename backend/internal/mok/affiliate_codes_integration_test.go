//go:build integration

package mok

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
)

// Letters without I and O, digits without 0 and 1: affiliateCodeCharset.
var fourCharacterInviteCode = regexp.MustCompile(`^[A-HJ-NP-Z2-9]{4}$`)

// mok_004 gives existing users 4-character invite codes. The test fills a migrated database
// with codes as they were before and runs mok_004 on it again.
func TestMigrationGivesExistingUsersFourCharacterInviteCodes(t *testing.T) {
	ctx := context.Background()
	env := newUsageRebateEnv(t)
	db := env.db
	f := &affiliateFixture{t: t, db: db}
	runMigration := func() {
		t.Helper()
		content, err := migrations.FS.ReadFile("mok_004_affiliate_codes_4_chars.sql")
		require.NoError(t, err)
		_, err = db.Exec(string(content))
		require.NoError(t, err)
	}
	setCode := func(userID int64, code string, custom bool) {
		t.Helper()
		_, err := db.Exec(`UPDATE user_affiliates SET aff_code = $2, aff_code_custom = $3 WHERE user_id = $1`, userID, code, custom)
		require.NoError(t, err)
	}
	codeOf := func(userID int64) string {
		t.Helper()
		var code string
		require.NoError(t, db.QueryRow(`SELECT aff_code FROM user_affiliates WHERE user_id = $1`, userID).Scan(&code))
		return code
	}
	inviterOf := func(userID int64) int64 {
		t.Helper()
		var inviter int64
		require.NoError(t, db.QueryRow(`SELECT COALESCE(inviter_id, 0) FROM user_affiliates WHERE user_id = $1`, userID).Scan(&inviter))
		return inviter
	}
	newcomer := func(email string) int64 {
		t.Helper()
		var id int64
		require.NoError(t, db.QueryRow(`INSERT INTO users (email, password_hash) VALUES ($1, 'x') RETURNING id`, email).Scan(&id))
		return id
	}

	inviter := f.user(0)
	invitee := f.user(inviter)
	custom, shortCustom, alreadyShort := f.user(0), f.user(0), f.user(0)
	setCode(inviter, "ABCDEFGHJKLM", false) // as upstream generates them
	setCode(custom, "VIP2026", true)
	setCode(shortCustom, "VIPS", true)
	setCode(alreadyShort, "ABCD", false)
	_, err := db.Exec(`
		WITH u AS (
			INSERT INTO users (email, password_hash)
			SELECT 'bulk' || i || '@example.com', 'x' FROM generate_series(1, 3000) AS i
			RETURNING id
		)
		INSERT INTO user_affiliates (user_id, aff_code)
		SELECT id, upper(substr(md5(id::text), 1, 12)) FROM u`)
	require.NoError(t, err)

	runMigration()

	require.Regexp(t, fourCharacterInviteCode, codeOf(inviter))
	require.Regexp(t, fourCharacterInviteCode, codeOf(invitee))
	require.Equal(t, "VIP2026", codeOf(custom), "codes set by an admin are kept")
	require.Equal(t, "VIPS", codeOf(shortCustom))
	require.Equal(t, "ABCD", codeOf(alreadyShort), "a system code that is already short is kept")
	require.Equal(t, inviter, inviterOf(invitee), "invite relations do not depend on the code")

	var total, distinct, notShort int
	require.NoError(t, db.QueryRow(`
		SELECT COUNT(*), COUNT(DISTINCT aff_code),
		       COUNT(*) FILTER (WHERE NOT aff_code_custom AND aff_code !~ '^[A-HJ-NP-Z2-9]{4}$')
		FROM user_affiliates`).Scan(&total, &distinct, &notShort))
	require.Equal(t, 3005, total)
	require.Equal(t, total, distinct, "every code is unique")
	require.Zero(t, notShort, "every system code is 4 characters from the code alphabet")

	// Running it again changes nothing.
	before := codeOf(inviter)
	runMigration()
	require.Equal(t, before, codeOf(inviter))

	// The new code binds, in any case; the old one is gone.
	first := newcomer("first@example.com")
	require.NoError(t, env.affiliates.BindInviterByCode(ctx, first, strings.ToLower(codeOf(inviter))))
	require.Equal(t, inviter, inviterOf(first))
	require.Regexp(t, fourCharacterInviteCode, codeOf(first), "new users get 4-character codes")
	second := newcomer("second@example.com")
	require.ErrorIs(t, env.affiliates.BindInviterByCode(ctx, second, "ABCDEFGHJKLM"), service.ErrAffiliateCodeInvalid)
	require.Zero(t, inviterOf(second))

	// An admin reset hands out a 4-character code as well.
	reset, err := env.affiliates.AdminResetUserAffCode(ctx, custom)
	require.NoError(t, err)
	require.Regexp(t, fourCharacterInviteCode, reset)
	require.Equal(t, reset, codeOf(custom))
}
