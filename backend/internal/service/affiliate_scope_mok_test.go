//go:build unit

package service

// [mok] Fork-only tests for affiliate_scope_mok.go (see FORK.md).

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (r *mokAffiliateRepo) MokIsAffiliateUserSelected(_ context.Context, userID int64) (bool, error) {
	return r.selected[userID], nil
}

func (r *mokAffiliateRepo) MokSetAffiliateUserSelected(_ context.Context, userID int64, selected bool) error {
	if r.selected == nil {
		r.selected = map[int64]bool{}
	}
	if !selected {
		delete(r.selected, userID)
		return nil
	}
	if _, known := r.summaries[userID]; !known {
		return ErrUserNotFound
	}
	r.selected[userID] = true
	return nil
}

func (r *mokAffiliateRepo) MokListAffiliateSelectedUsers(_ context.Context, page, pageSize int) ([]MokAffiliateSelectedUser, int64, error) {
	ids := make([]int64, 0, len(r.selected))
	for id := range r.selected {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var users []MokAffiliateSelectedUser
	for i := (page - 1) * pageSize; i < len(ids) && i < page*pageSize; i++ {
		users = append(users, MokAffiliateSelectedUser{UserID: ids[i]})
	}
	return users, int64(len(ids)), nil
}

func (r *mokAffiliateRepo) MokAffiliateUserHasQuota(_ context.Context, userID int64) (bool, error) {
	return r.withQuota[userID], nil
}

func (r *mokAffiliateRepo) GetAffiliateByCode(_ context.Context, code string) (*AffiliateSummary, error) {
	for _, summary := range r.summaries {
		if summary.AffCode == code {
			copied := *summary
			return &copied, nil
		}
	}
	return nil, ErrAffiliateProfileNotFound
}

func (r *mokAffiliateRepo) BindInviter(_ context.Context, userID, inviterID int64) (bool, error) {
	r.bound = append(r.bound, [2]int64{userID, inviterID})
	return true, nil
}

const mokNewcomerID = int64(30)

func TestMokAffiliateScopeDefaultsToEveryone(t *testing.T) {
	ctx := context.Background()
	svc, repo, settings := newMokAffiliateFixture(t, "")

	allowed, err := svc.MokAffiliateAllowed(ctx, mokInviterID)
	require.NoError(t, err)
	require.True(t, allowed, "without a scope setting the feature is on for everyone, as upstream has it")

	settings.values[SettingKeyMokAffiliateScope] = "nonsense"
	allowed, err = svc.MokAffiliateAllowed(ctx, mokInviterID)
	require.NoError(t, err)
	require.True(t, allowed, "an unknown scope counts as everyone")

	settings.values[SettingKeyAffiliateEnabled] = "false"
	repo.selected = map[int64]bool{mokInviterID: true}
	allowed, err = svc.MokAffiliateAllowed(ctx, mokInviterID)
	require.NoError(t, err)
	require.False(t, allowed, "upstream's switch still turns the feature off for everyone")
}

func TestMokAffiliateScopeSelectedLetsOnlySelectedUsersIn(t *testing.T) {
	ctx := context.Background()
	svc, repo, settings := newMokAffiliateFixture(t, "")
	settings.values[SettingKeyMokAffiliateScope] = MokAffiliateScopeSelected

	allowed, err := svc.MokAffiliateAllowed(ctx, mokInviterID)
	require.NoError(t, err)
	require.False(t, allowed)

	repo.selected = map[int64]bool{mokInviterID: true}
	allowed, err = svc.MokAffiliateAllowed(ctx, mokInviterID)
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, err = svc.MokAffiliateAllowed(ctx, mokInviteeID)
	require.NoError(t, err)
	require.False(t, allowed, "being invited by a selected user does not select the invitee")
}

func TestMokAffiliateOwnAccessKeepsTheEntryForUsersWithQuota(t *testing.T) {
	ctx := context.Background()
	svc, repo, settings := newMokAffiliateFixture(t, "")
	repo.withQuota = map[int64]bool{mokInviterID: true}

	access, err := svc.MokAffiliateOwnAccess(ctx, mokInviterID)
	require.NoError(t, err)
	require.Equal(t, &MokAffiliateOwnAccess{Allowed: true}, access, "quota does not matter for a user the feature is on for")

	settings.values[SettingKeyMokAffiliateScope] = MokAffiliateScopeSelected
	access, err = svc.MokAffiliateOwnAccess(ctx, mokInviterID)
	require.NoError(t, err)
	require.Equal(t, &MokAffiliateOwnAccess{HasQuota: true}, access)
	access, err = svc.MokAffiliateOwnAccess(ctx, mokInviteeID)
	require.NoError(t, err)
	require.Equal(t, &MokAffiliateOwnAccess{}, access)

	settings.values[SettingKeyAffiliateEnabled] = "false"
	access, err = svc.MokAffiliateOwnAccess(ctx, mokInviterID)
	require.NoError(t, err)
	require.Equal(t, &MokAffiliateOwnAccess{}, access, "with the feature off altogether upstream hides the entry for everyone")
}

func TestMokAffiliateScopeGatesRechargeRebatesByInviter(t *testing.T) {
	ctx := context.Background()
	svc, repo, settings := newMokAffiliateFixture(t, "")
	settings.values[SettingKeyMokAffiliateScope] = MokAffiliateScopeSelected
	orderID := mokOrderID

	rebate, err := svc.AccrueInviteRebateForOrder(ctx, mokInviteeID, 1000, &orderID)
	require.NoError(t, err)
	require.Zero(t, rebate, "the inviter is not selected")
	require.Empty(t, repo.accrued)

	// Selecting the invitee changes nothing: it is the inviter who earns.
	repo.selected = map[int64]bool{mokInviteeID: true}
	rebate, err = svc.AccrueInviteRebateForOrder(ctx, mokInviteeID, 1000, &orderID)
	require.NoError(t, err)
	require.Zero(t, rebate)

	repo.selected = map[int64]bool{mokInviterID: true}
	rebate, err = svc.AccrueInviteRebateForOrder(ctx, mokInviteeID, 1000, &orderID)
	require.NoError(t, err)
	require.Equal(t, 100.0, rebate, "10% once the inviter is selected")
	require.Equal(t, []float64{100}, repo.accrued)
}

func TestMokAffiliateScopeGatesInviteCodes(t *testing.T) {
	ctx := context.Background()
	svc, repo, settings := newMokAffiliateFixture(t, "")

	require.NoError(t, svc.BindInviterByCode(ctx, mokNewcomerID, "inviter"))
	require.Equal(t, [][2]int64{{mokNewcomerID, mokInviterID}}, repo.bound, "with scope all every code binds")

	repo.bound = nil
	settings.values[SettingKeyMokAffiliateScope] = MokAffiliateScopeSelected
	err := svc.BindInviterByCode(ctx, mokNewcomerID, "INVITER")
	require.ErrorIs(t, err, ErrAffiliateCodeInvalid, "the code of a user who is not selected is not valid")
	require.Empty(t, repo.bound)

	repo.selected = map[int64]bool{mokInviterID: true}
	require.NoError(t, svc.BindInviterByCode(ctx, mokNewcomerID, "INVITER"))
	require.Equal(t, [][2]int64{{mokNewcomerID, mokInviterID}}, repo.bound)
}

func TestMokAffiliateScopeGatesUsageRebatesByInviter(t *testing.T) {
	ctx := context.Background()
	svc, repo, settings := newMokUsageFixture(t)
	settings.values[SettingKeyMokAffiliateScope] = MokAffiliateScopeSelected
	repo.selected = map[int64]bool{mokOtherInviterID: true}

	closed, err := svc.SettleDueAt(ctx, mokAt("2026-11-01", "12:06"))
	require.NoError(t, err)
	require.Equal(t, 2, closed, "the hours are closed either way")
	require.Equal(t, []float64{10}, repo.accrued, "only the selected inviter earns")
	require.NotContains(t, repo.settled, "2026-11-01 10:20/20")
	require.Contains(t, repo.settled, "2026-11-01 11:00/21")

	// Selecting the other inviter later pays from the next hour on, not for closed hours.
	repo.selected[mokInviterID] = true
	closed, err = svc.SettleDueAt(ctx, mokAt("2026-11-01", "13:06"))
	require.NoError(t, err)
	require.Equal(t, 1, closed)
	require.Equal(t, []float64{10, 36}, repo.accrued)
}

func TestMokAffiliateTierStatusReportsUsersOutsideTheScope(t *testing.T) {
	ctx := context.Background()
	svc, repo, settings := newMokAffiliateFixture(t, mokTiersJSON)

	status, err := svc.MokAffiliateTierStatus(ctx, mokInviterID)
	require.NoError(t, err)
	require.True(t, status.Allowed)
	require.True(t, status.Enabled)

	settings.values[SettingKeyMokAffiliateScope] = MokAffiliateScopeSelected
	repo.volumeCalls = nil
	status, err = svc.MokAffiliateTierStatus(ctx, mokInviterID)
	require.NoError(t, err)
	require.False(t, status.Allowed)
	require.False(t, status.Enabled, "no tier card for a user the feature is off for")
	require.Equal(t, MokAffiliateRebateModeRecharge, status.RebateMode)
	require.Empty(t, repo.volumeCalls, "the team volume is not looked up for such a user")

	repo.selected = map[int64]bool{mokInviterID: true}
	status, err = svc.MokAffiliateTierStatus(ctx, mokInviterID)
	require.NoError(t, err)
	require.True(t, status.Allowed)
	require.True(t, status.Enabled)
}

func TestMokAffiliateScopeAdminOperations(t *testing.T) {
	ctx := context.Background()
	svc, _, settings := newMokAffiliateFixture(t, "")

	status, err := svc.MokAffiliateScopeStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, &MokAffiliateScopeStatus{AffiliateEnabled: true, Scope: MokAffiliateScopeAll}, status)

	// A user can be switched on ahead of narrowing the scope; until then it changes nothing.
	access, err := svc.MokSetAffiliateUserSelected(ctx, mokInviterID, true)
	require.NoError(t, err)
	require.Equal(t, &MokAffiliateUserAccess{UserID: mokInviterID, Selected: true, Scope: MokAffiliateScopeAll, AffiliateEnabled: true, Allowed: true}, access)
	access, err = svc.MokAffiliateUserAccess(ctx, mokInviteeID)
	require.NoError(t, err)
	require.False(t, access.Selected)
	require.True(t, access.Allowed, "with scope all the feature is on without the switch")

	status, err = svc.MokSetAffiliateScope(ctx, MokAffiliateScopeSelected)
	require.NoError(t, err)
	require.Equal(t, &MokAffiliateScopeStatus{AffiliateEnabled: true, Scope: MokAffiliateScopeSelected, SelectedCount: 1}, status)
	require.Equal(t, MokAffiliateScopeSelected, settings.values[SettingKeyMokAffiliateScope])
	access, err = svc.MokAffiliateUserAccess(ctx, mokInviteeID)
	require.NoError(t, err)
	require.False(t, access.Allowed)

	users, total, err := svc.MokListAffiliateSelectedUsers(ctx, 1, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, mokInviterID, users[0].UserID)

	access, err = svc.MokSetAffiliateUserSelected(ctx, mokInviterID, false)
	require.NoError(t, err)
	require.False(t, access.Selected)
	require.False(t, access.Allowed)
	users, total, err = svc.MokListAffiliateSelectedUsers(ctx, 1, 20)
	require.NoError(t, err)
	require.Zero(t, total)
	require.NotNil(t, users, "an empty list is sent as an empty array")

	_, err = svc.MokSetAffiliateUserSelected(ctx, 9999, true)
	require.True(t, errors.Is(err, ErrUserNotFound))
	_, err = svc.MokSetAffiliateUserSelected(ctx, 0, true)
	require.Equal(t, "INVALID_USER", infraerrors.Reason(err))
	_, err = svc.MokSetAffiliateScope(ctx, "some")
	require.Equal(t, "MOK_AFFILIATE_SCOPE_INVALID", infraerrors.Reason(err))

	settings.values[SettingKeyAffiliateEnabled] = "false"
	access, err = svc.MokAffiliateUserAccess(ctx, mokInviterID)
	require.NoError(t, err)
	require.False(t, access.AffiliateEnabled)
	require.False(t, access.Allowed)
}
