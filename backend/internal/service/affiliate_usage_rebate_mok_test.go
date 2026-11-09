//go:build unit

package service

// [mok] Fork-only tests for affiliate_usage_rebate_mok.go (see FORK.md).

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var mokCST = time.FixedZone("CST", 8*3600)

func mokAt(day, clock string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", day+" "+clock, mokCST)
	if err != nil {
		panic(err)
	}
	return t
}

// requireWindow compares instants, since the window mixes the stored UTC switch time with
// local day boundaries.
func requireWindow(t *testing.T, got [2]time.Time, from, to time.Time, msg ...any) {
	t.Helper()
	require.True(t, got[0].Equal(from) && got[1].Equal(to), "window %v - %v, want %v - %v %v", got[0], got[1], from, to, fmt.Sprint(msg...))
}

type mokUsageCall struct {
	from, to     time.Time
	durationDays int
}

// mokUsageRepo adds an in-memory MokAffiliateUsageRebateStore to mokAffiliateRepo.
type mokUsageRepo struct {
	*mokAffiliateRepo
	usageByDay map[string][]MokInviteeUsage
	usageCalls []mokUsageCall
	settled    map[string]MokAffiliateUsageRebateRow
	settleErr  map[int64]error
	closed     map[string]string
	windows    map[string][2]time.Time
}

func (r *mokUsageRepo) MokBalanceUsageByInvitee(_ context.Context, from, to time.Time, durationDays int) ([]MokInviteeUsage, error) {
	r.usageCalls = append(r.usageCalls, mokUsageCall{from: from, to: to, durationDays: durationDays})
	return r.usageByDay[from.In(mokCST).Format(mokUsageRebateDayLayout)], nil
}

func (r *mokUsageRepo) MokSettleUsageRebate(ctx context.Context, row MokAffiliateUsageRebateRow, freezeHours int) (bool, error) {
	if err := r.settleErr[row.InviteeID]; err != nil {
		return false, err
	}
	key := fmt.Sprintf("%s/%d", row.Day, row.InviteeID)
	if _, done := r.settled[key]; done {
		return false, nil
	}
	r.settled[key] = row
	if row.RebateAmount > 0 {
		_, _ = r.AccrueQuota(ctx, row.InviterID, row.InviteeID, row.RebateAmount, freezeHours, nil)
	}
	return true, nil
}

func (r *mokUsageRepo) MokCloseUsageRebateDay(_ context.Context, day string, windowStart, windowEnd time.Time, skippedReason string) error {
	if _, done := r.closed[day]; !done {
		r.closed[day] = skippedReason
		r.windows[day] = [2]time.Time{windowStart, windowEnd}
	}
	return nil
}

func (r *mokUsageRepo) MokUsageRebateClosedDays(context.Context, string) (map[string]bool, error) {
	out := map[string]bool{}
	for day := range r.closed {
		out[day] = true
	}
	return out, nil
}

func (r *mokUsageRepo) MokRecentUsageRebateDays(context.Context, int) ([]MokAffiliateUsageRebateDay, error) {
	days := []MokAffiliateUsageRebateDay{}
	for day, reason := range r.closed {
		days = append(days, MokAffiliateUsageRebateDay{Day: day, SkippedReason: reason})
	}
	return days, nil
}

const mokOtherInviterID = int64(11)

// newMokUsageFixture is in usage mode since 2026-11-01 10:00 (UTC+8) with a 10% global rate
// and tiers of 12% / 15% at a recharge volume of 50000 / 100000. Inviter 10 has a team
// recharge volume of 60000 (12%); inviter 11 has an exclusive 25%.
func newMokUsageFixture(t *testing.T) (*MokAffiliateUsageRebateService, *mokUsageRepo, *mokSettingRepo) {
	t.Helper()
	settings := &mokSettingRepo{values: map[string]string{
		SettingKeyAffiliateEnabled:        "true",
		SettingKeyAffiliateRebateRate:     "10",
		SettingKeyMokAffiliateRebateTiers: mokTiersJSON,
		SettingKeyMokAffiliateRebateMode:  MokAffiliateRebateModeUsage,
		SettingKeyMokAffiliateUsageSince:  mokAt("2026-11-01", "10:00").UTC().Format(time.RFC3339Nano),
	}}
	exclusive := 25.0
	base := &mokAffiliateRepo{
		summaries: map[int64]*AffiliateSummary{
			mokInviterID:      {UserID: mokInviterID, AffCode: "INVITER"},
			mokOtherInviterID: {UserID: mokOtherInviterID, AffCode: "OTHER", AffRebateRatePercent: &exclusive},
		},
		volume: 60000,
	}
	repo := &mokUsageRepo{
		mokAffiliateRepo: base,
		usageByDay: map[string][]MokInviteeUsage{
			"2026-11-01": {{InviteeID: 20, InviterID: mokInviterID, Amount: 100}},
			"2026-11-02": {
				{InviteeID: 20, InviterID: mokInviterID, Amount: 200},
				{InviteeID: 21, InviterID: mokOtherInviterID, Amount: 40},
			},
			"2026-11-03": {{InviteeID: 20, InviterID: mokInviterID, Amount: 300}},
		},
		settled:   map[string]MokAffiliateUsageRebateRow{},
		settleErr: map[int64]error{},
		closed:    map[string]string{},
		windows:   map[string][2]time.Time{},
	}
	svc := NewMokAffiliateUsageRebateService(NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil), nil, nil)
	svc.location = func() *time.Location { return mokCST }
	return svc, repo, settings
}

func TestMokUsageRebateSettlesEndedDaysOnce(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newMokUsageFixture(t)
	now := mokAt("2026-11-03", "00:40")

	closed, err := svc.SettleDueAt(ctx, now)
	require.NoError(t, err)
	require.Equal(t, 2, closed, "11-03 has not ended yet")

	require.Equal(t, map[string]string{"2026-11-01": "", "2026-11-02": ""}, repo.closed)
	requireWindow(t, repo.windows["2026-11-01"], mokAt("2026-11-01", "10:00"), mokAt("2026-11-02", "00:00"),
		"the first day starts at the switch to usage mode")
	requireWindow(t, repo.windows["2026-11-02"], mokAt("2026-11-02", "00:00"), mokAt("2026-11-03", "00:00"))

	require.Equal(t, MokAffiliateUsageRebateRow{Day: "2026-11-01", InviteeID: 20, InviterID: mokInviterID, UsageAmount: 100, RatePercent: 12, RebateAmount: 12},
		repo.settled["2026-11-01/20"], "the tier follows the team's recharge volume")
	require.Equal(t, 24.0, repo.settled["2026-11-02/20"].RebateAmount)
	require.Equal(t, MokAffiliateUsageRebateRow{Day: "2026-11-02", InviteeID: 21, InviterID: mokOtherInviterID, UsageAmount: 40, RatePercent: 25, RebateAmount: 10},
		repo.settled["2026-11-02/21"], "an exclusive rate applies to usage as well")
	require.Equal(t, []float64{12, 24, 10}, repo.accrued)

	closed, err = svc.SettleDueAt(ctx, now)
	require.NoError(t, err)
	require.Zero(t, closed)
	require.Len(t, repo.accrued, 3, "a closed day is never settled again")
}

func TestMokUsageRebateWaitsForTheSettleDelay(t *testing.T) {
	svc, repo, _ := newMokUsageFixture(t)

	closed, err := svc.SettleDueAt(context.Background(), mokAt("2026-11-03", "00:20"))

	require.NoError(t, err)
	require.Equal(t, 1, closed)
	require.Contains(t, repo.closed, "2026-11-01")
	require.NotContains(t, repo.closed, "2026-11-02", "11-02 ended only 20 minutes ago")
}

func TestMokUsageRebateAppliesCapAndDuration(t *testing.T) {
	svc, repo, settings := newMokUsageFixture(t)
	settings.values[SettingKeyAffiliateRebatePerInviteeCap] = "30"
	settings.values[SettingKeyAffiliateRebateDurationDays] = "90"
	repo.existingRebate = 25

	_, err := svc.SettleDueAt(context.Background(), mokAt("2026-11-02", "01:00"))

	require.NoError(t, err)
	require.Equal(t, 5.0, repo.settled["2026-11-01/20"].RebateAmount, "12 is capped to the 5 left under the per-invitee cap")
	require.Equal(t, 90, repo.usageCalls[0].durationDays)
}

func TestMokUsageRebateClosesDaysWithoutRebatesWhileAffiliateIsOff(t *testing.T) {
	svc, repo, settings := newMokUsageFixture(t)
	settings.values[SettingKeyAffiliateEnabled] = "false"

	closed, err := svc.SettleDueAt(context.Background(), mokAt("2026-11-03", "01:00"))

	require.NoError(t, err)
	require.Equal(t, 2, closed)
	require.Equal(t, MokUsageRebateSkippedAffiliateDisabled, repo.closed["2026-11-01"])
	require.Empty(t, repo.usageCalls)
	require.Empty(t, repo.accrued)
}

func TestMokUsageRebateRetriesAFailedDayWithoutPayingTwice(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newMokUsageFixture(t)
	repo.settleErr[21] = errors.New("connection reset")

	closed, err := svc.SettleDueAt(ctx, mokAt("2026-11-03", "01:00"))
	require.ErrorContains(t, err, "connection reset")
	require.Equal(t, 1, closed)
	require.NotContains(t, repo.closed, "2026-11-02")
	require.Equal(t, []float64{12, 24}, repo.accrued)

	delete(repo.settleErr, 21)
	closed, err = svc.SettleDueAt(ctx, mokAt("2026-11-03", "01:00"))
	require.NoError(t, err)
	require.Equal(t, 1, closed)
	require.Equal(t, []float64{12, 24, 10}, repo.accrued, "invitee 20 is not paid again for 11-02")
}

func TestMokUsageRebateAfterSwitchingBackToRecharge(t *testing.T) {
	svc, repo, settings := newMokUsageFixture(t)
	settings.values[SettingKeyMokAffiliateRebateMode] = MokAffiliateRebateModeRecharge
	settings.values[SettingKeyMokAffiliateUsageUntil] = mokAt("2026-11-02", "12:00").UTC().Format(time.RFC3339Nano)

	closed, err := svc.SettleDueAt(context.Background(), mokAt("2026-11-10", "08:00"))

	require.NoError(t, err)
	require.Equal(t, 2, closed, "only the days of the closed usage window are settled")
	requireWindow(t, repo.windows["2026-11-02"], mokAt("2026-11-02", "00:00"), mokAt("2026-11-02", "12:00"))
	require.NotContains(t, repo.closed, "2026-11-03")

	delete(settings.values, SettingKeyMokAffiliateUsageUntil)
	closed, err = svc.SettleDueAt(context.Background(), mokAt("2026-11-10", "08:00"))
	require.NoError(t, err)
	require.Zero(t, closed, "recharge mode without a closed window settles nothing")
}

func TestMokUsageModeStopsRechargeRebates(t *testing.T) {
	ctx := context.Background()
	svc, repo, settings := newMokAffiliateFixture(t, mokTiersJSON)
	settings.values[SettingKeyMokAffiliateRebateMode] = MokAffiliateRebateModeUsage
	repo.volume = 60000
	orderID := mokOrderID

	rebate, err := svc.AccrueInviteRebateForOrder(ctx, mokInviteeID, 1000, &orderID)
	require.NoError(t, err)
	require.Zero(t, rebate)
	require.Empty(t, repo.accrued)

	detail, err := svc.GetAffiliateDetail(ctx, mokInviterID)
	require.NoError(t, err)
	require.Equal(t, 12.0, detail.EffectiveRebateRatePercent, "the page still shows the rate applied to usage")

	status, err := svc.MokAffiliateTierStatus(ctx, mokInviterID)
	require.NoError(t, err)
	require.Equal(t, MokAffiliateRebateModeUsage, status.RebateMode)
}

func TestMokSetAffiliateRebateMode(t *testing.T) {
	ctx := context.Background()
	repo := &mokSettingRepo{values: map[string]string{SettingKeyMokAffiliateUsageUntil: "old"}}
	settings := NewSettingService(repo, nil)
	switchOn := mokAt("2026-11-01", "10:00")
	switchOff := mokAt("2026-11-05", "18:30")

	require.NoError(t, settings.MokSetAffiliateRebateMode(ctx, MokAffiliateRebateModeUsage, switchOn))
	require.Equal(t, MokAffiliateRebateModeUsage, repo.values[SettingKeyMokAffiliateRebateMode])
	require.NotContains(t, repo.values, SettingKeyMokAffiliateUsageUntil)
	window, err := settings.MokGetAffiliateUsageWindow(ctx)
	require.NoError(t, err)
	require.True(t, window.Since.Equal(switchOn))
	require.Nil(t, window.Until)

	require.NoError(t, settings.MokSetAffiliateRebateMode(ctx, MokAffiliateRebateModeUsage, switchOff))
	window, _ = settings.MokGetAffiliateUsageWindow(ctx)
	require.True(t, window.Since.Equal(switchOn), "setting the current mode again keeps the window")

	require.NoError(t, settings.MokSetAffiliateRebateMode(ctx, MokAffiliateRebateModeRecharge, switchOff))
	window, _ = settings.MokGetAffiliateUsageWindow(ctx)
	require.True(t, window.Since.Equal(switchOn))
	require.True(t, window.Until.Equal(switchOff))

	err = settings.MokSetAffiliateRebateMode(ctx, "monthly", switchOff)
	require.Equal(t, "MOK_AFFILIATE_REBATE_MODE_INVALID", infraerrors.Reason(err))
}

func TestMokUsageWindowBounds(t *testing.T) {
	since := mokAt("2026-11-01", "10:00")
	until := mokAt("2026-11-02", "12:00")

	_, _, ok := mokUsageWindowBounds(MokAffiliateRebateModeUsage, MokAffiliateUsageWindow{})
	require.False(t, ok, "never switched on")

	start, end, ok := mokUsageWindowBounds(MokAffiliateRebateModeUsage, MokAffiliateUsageWindow{Since: &since, Until: &until})
	require.True(t, ok)
	require.Equal(t, since, start)
	require.True(t, end.IsZero(), "a stale end is ignored in usage mode")

	_, end, ok = mokUsageWindowBounds(MokAffiliateRebateModeRecharge, MokAffiliateUsageWindow{Since: &since, Until: &until})
	require.True(t, ok)
	require.Equal(t, until, end)

	_, _, ok = mokUsageWindowBounds(MokAffiliateRebateModeRecharge, MokAffiliateUsageWindow{Since: &until, Until: &since})
	require.False(t, ok, "an end before the start is no window")
}
