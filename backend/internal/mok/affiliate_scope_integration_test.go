//go:build integration

package mok

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestAffiliateScopeSelectedUsersWithRealRepositories(t *testing.T) {
	ctx := context.Background()
	env := newUsageRebateEnv(t)
	at, db, affiliates := env.at, env.db, env.affiliates
	f := &affiliateFixture{t: t, db: db}

	chosen := f.user(0)
	chosenInvitee := f.user(chosen)
	other := f.user(0)
	otherInvitee := f.user(other)
	gone := f.user(0)
	_, err := db.Exec(`UPDATE users SET deleted_at = NOW() WHERE id = $1`, gone)
	require.NoError(t, err)
	codeOf := func(userID int64) string {
		t.Helper()
		var code string
		require.NoError(t, db.QueryRow(`SELECT aff_code FROM user_affiliates WHERE user_id = $1`, userID).Scan(&code))
		return code
	}
	newcomer := func(email string) int64 {
		t.Helper()
		var id int64
		require.NoError(t, db.QueryRow(`INSERT INTO users (email, password_hash) VALUES ($1, 'x') RETURNING id`, email).Scan(&id))
		return id
	}
	inviterOf := func(userID int64) int64 {
		t.Helper()
		var inviter int64
		require.NoError(t, db.QueryRow(`SELECT COALESCE(inviter_id, 0) FROM user_affiliates WHERE user_id = $1`, userID).Scan(&inviter))
		return inviter
	}

	// With the default scope the feature is on for everyone and the switch changes nothing.
	status, err := affiliates.MokAffiliateScopeStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, &service.MokAffiliateScopeStatus{AffiliateEnabled: true, Scope: service.MokAffiliateScopeAll}, status)
	access, err := affiliates.MokAffiliateUserAccess(ctx, other)
	require.NoError(t, err)
	require.True(t, access.Allowed)

	// Only existing users can be switched on; doing it twice is fine.
	_, err = affiliates.MokSetAffiliateUserSelected(ctx, 987654, true)
	require.ErrorIs(t, err, service.ErrUserNotFound)
	_, err = affiliates.MokSetAffiliateUserSelected(ctx, gone, true)
	require.ErrorIs(t, err, service.ErrUserNotFound, "a deleted user")
	for i := 0; i < 2; i++ {
		access, err = affiliates.MokSetAffiliateUserSelected(ctx, chosen, true)
		require.NoError(t, err)
		require.True(t, access.Selected)
	}

	status, err = affiliates.MokSetAffiliateScope(ctx, service.MokAffiliateScopeSelected)
	require.NoError(t, err)
	require.Equal(t, int64(1), status.SelectedCount)
	users, total, err := affiliates.MokListAffiliateSelectedUsers(ctx, 1, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, chosen, users[0].UserID)
	require.Equal(t, f.email(chosen), users[0].Email)
	require.False(t, users[0].SelectedAt.IsZero())

	for user, want := range map[int64]bool{chosen: true, other: false, chosenInvitee: false} {
		allowed, err := affiliates.MokAffiliateAllowed(ctx, user)
		require.NoError(t, err)
		require.Equal(t, want, allowed, "user %d", user)
	}

	// Invite codes: only the selected user's code binds.
	first := newcomer("first@example.com")
	require.ErrorIs(t, affiliates.BindInviterByCode(ctx, first, codeOf(other)), service.ErrAffiliateCodeInvalid)
	require.Zero(t, inviterOf(first))
	require.NoError(t, affiliates.BindInviterByCode(ctx, first, codeOf(chosen)))
	require.Equal(t, chosen, inviterOf(first))

	// Recharges: only the selected inviter earns.
	otherOrder := f.order(otherInvitee, "balance", "RECHARGING", 1000, 0)
	rebate, err := affiliates.AccrueInviteRebateForOrder(ctx, otherInvitee, 1000, &otherOrder)
	require.NoError(t, err)
	require.Zero(t, rebate)
	chosenOrder := f.order(chosenInvitee, "balance", "RECHARGING", 1000, 0)
	rebate, err = affiliates.AccrueInviteRebateForOrder(ctx, chosenInvitee, 1000, &chosenOrder)
	require.NoError(t, err)
	require.Equal(t, 100.0, rebate)
	require.Empty(t, f.accrued(other))
	require.Equal(t, []float64{100}, f.accrued(chosen))

	// Usage rebates: the same, hour by hour. Selecting the other inviter later pays from
	// then on, not for hours already closed.
	require.NoError(t, env.settings.MokSetAffiliateRebateMode(ctx, service.MokAffiliateRebateModeUsage, at(1, 10, 0)))
	account := f.account()
	chosenKey, otherKey := f.apiKey(chosenInvitee), f.apiKey(otherInvitee)
	f.usage(chosenInvitee, chosenKey, account, 0, 50, at(1, 10, 30))
	f.usage(otherInvitee, otherKey, account, 0, 80, at(1, 10, 40))
	f.usage(otherInvitee, otherKey, account, 0, 20, at(1, 11, 10))
	closed, err := env.usageRebate.SettleDueAt(ctx, at(1, 11, 6))
	require.NoError(t, err)
	require.Equal(t, 1, closed)
	require.Equal(t, []float64{100, 5}, f.accrued(chosen))
	require.Empty(t, f.accrued(other))
	records, total, err := env.usageRebate.ListRecords(ctx, service.MokAffiliateUsageRebateFilter{})
	require.NoError(t, err)
	require.Equal(t, int64(1), total, "no record for the inviter outside the scope")
	require.Equal(t, chosen, records[0].InviterID)

	_, err = affiliates.MokSetAffiliateUserSelected(ctx, other, true)
	require.NoError(t, err)
	closed, err = env.usageRebate.SettleDueAt(ctx, at(1, 12, 6))
	require.NoError(t, err)
	require.Equal(t, 1, closed)
	require.Equal(t, []float64{2}, f.accrued(other), "10% of the 20 used after 11:00")

	// A user the feature is off for keeps the menu entry while they have rebate quota.
	for user, want := range map[int64]service.MokAffiliateOwnAccess{
		chosen:        {Allowed: true},
		other:         {Allowed: true},
		chosenInvitee: {},
	} {
		own, err := affiliates.MokAffiliateOwnAccess(ctx, user)
		require.NoError(t, err)
		require.Equal(t, want, *own, "user %d", user)
	}
	_, err = affiliates.MokSetAffiliateUserSelected(ctx, other, false)
	require.NoError(t, err)
	own, err := affiliates.MokAffiliateOwnAccess(ctx, other)
	require.NoError(t, err)
	require.Equal(t, service.MokAffiliateOwnAccess{HasQuota: true}, *own, "the 2 earned above")
	_, _, err = affiliates.TransferAffiliateQuota(ctx, other)
	require.NoError(t, err)
	own, err = affiliates.MokAffiliateOwnAccess(ctx, other)
	require.NoError(t, err)
	require.Equal(t, service.MokAffiliateOwnAccess{}, *own, "nothing left after the transfer")
	_, err = affiliates.MokSetAffiliateUserSelected(ctx, other, true)
	require.NoError(t, err)

	// Switching a user off, or losing the account, takes them off the list.
	access, err = affiliates.MokSetAffiliateUserSelected(ctx, other, false)
	require.NoError(t, err)
	require.False(t, access.Selected)
	require.False(t, access.Allowed)
	_, err = db.Exec(`UPDATE users SET deleted_at = NOW() WHERE id = $1`, chosen)
	require.NoError(t, err)
	users, total, err = affiliates.MokListAffiliateSelectedUsers(ctx, 1, 20)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, users)

	// Back to everyone.
	status, err = affiliates.MokSetAffiliateScope(ctx, service.MokAffiliateScopeAll)
	require.NoError(t, err)
	require.Equal(t, service.MokAffiliateScopeAll, status.Scope)
	allowed, err := affiliates.MokAffiliateAllowed(ctx, other)
	require.NoError(t, err)
	require.True(t, allowed)
}
