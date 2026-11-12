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

const mokClockLayout = "2006-01-02 15:04"

func mokAt(day, clock string) time.Time {
	t, err := time.ParseInLocation(mokClockLayout, day+" "+clock, mokCST)
	if err != nil {
		panic(err)
	}
	return t
}

type mokUsageCall struct {
	from, to     time.Time
	durationDays int
}

type mokClosedPeriod struct {
	day      string
	from, to time.Time
	reason   string
}

// mokUsageRepo adds an in-memory MokAffiliateUsageRebateStore to mokAffiliateRepo. Usage and
// settled rows are keyed by the period start on the UTC+8 clock, as "2006-01-02 15:04".
type mokUsageRepo struct {
	*mokAffiliateRepo
	usageByPeriod map[string][]MokInviteeUsage
	usageCalls    []mokUsageCall
	settled       map[string]MokAffiliateUsageRebateRow
	settleErr     map[int64]error
	periods       []mokClosedPeriod
	records       []MokAffiliateUsageRebateRecord
	listFilters   []MokAffiliateUsageRebateFilter
}

func mokPeriodKey(start time.Time) string {
	return start.In(mokCST).Format(mokClockLayout)
}

func (r *mokUsageRepo) MokBalanceUsageByInvitee(_ context.Context, from, to time.Time, durationDays int) ([]MokInviteeUsage, error) {
	r.usageCalls = append(r.usageCalls, mokUsageCall{from: from, to: to, durationDays: durationDays})
	return r.usageByPeriod[mokPeriodKey(from)], nil
}

func (r *mokUsageRepo) MokSettleUsageRebate(ctx context.Context, row MokAffiliateUsageRebateRow, freezeHours int) (bool, error) {
	if err := r.settleErr[row.InviteeID]; err != nil {
		return false, err
	}
	key := fmt.Sprintf("%s/%d", mokPeriodKey(row.PeriodStart), row.InviteeID)
	if _, done := r.settled[key]; done {
		return false, nil
	}
	r.settled[key] = row
	if row.RebateAmount > 0 {
		_, _ = r.AccrueQuota(ctx, row.InviterID, row.InviteeID, row.RebateAmount, freezeHours, nil)
	}
	return true, nil
}

func (r *mokUsageRepo) MokCloseUsageRebatePeriod(_ context.Context, day string, periodStart, periodEnd time.Time, skippedReason string) error {
	for _, period := range r.periods {
		if period.from.Equal(periodStart) {
			return nil
		}
	}
	r.periods = append(r.periods, mokClosedPeriod{day: day, from: periodStart, to: periodEnd, reason: skippedReason})
	return nil
}

func (r *mokUsageRepo) MokUsageRebateSettledUntil(context.Context) (*time.Time, error) {
	var until *time.Time
	for i := range r.periods {
		if until == nil || r.periods[i].to.After(*until) {
			until = &r.periods[i].to
		}
	}
	return until, nil
}

func (r *mokUsageRepo) MokRecentUsageRebateDays(context.Context, int) ([]MokAffiliateUsageRebateDay, error) {
	return nil, nil
}

func (r *mokUsageRepo) MokListUsageRebates(_ context.Context, filter MokAffiliateUsageRebateFilter) ([]MokAffiliateUsageRebateRecord, int64, error) {
	r.listFilters = append(r.listFilters, filter)
	return append([]MokAffiliateUsageRebateRecord(nil), r.records...), int64(len(r.records)), nil
}

// requirePeriods compares the closed periods with "day from-to" clock times on UTC+8.
func requirePeriods(t *testing.T, repo *mokUsageRepo, want ...string) {
	t.Helper()
	got := make([]string, 0, len(repo.periods))
	for _, period := range repo.periods {
		got = append(got, fmt.Sprintf("%s %s-%s", period.day, period.from.In(mokCST).Format("15:04"), period.to.In(mokCST).Format("15:04")))
	}
	require.Equal(t, want, got)
}

const mokOtherInviterID = int64(11)

// newMokUsageFixture is in usage mode since 2026-11-01 10:20 (UTC+8) with a 10% global rate
// and tiers of 12% / 15% at a recharge volume of 50000 / 100000. Inviter 10 has a team
// recharge volume of 60000 (12%); inviter 11 has an exclusive 25%.
func newMokUsageFixture(t *testing.T) (*MokAffiliateUsageRebateService, *mokUsageRepo, *mokSettingRepo) {
	t.Helper()
	settings := &mokSettingRepo{values: map[string]string{
		SettingKeyAffiliateEnabled:        "true",
		SettingKeyAffiliateRebateRate:     "10",
		SettingKeyMokAffiliateRebateTiers: mokTiersJSON,
		SettingKeyMokAffiliateRebateMode:  MokAffiliateRebateModeUsage,
		SettingKeyMokAffiliateUsageSince:  mokAt("2026-11-01", "10:20").UTC().Format(time.RFC3339Nano),
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
		usageByPeriod: map[string][]MokInviteeUsage{
			"2026-11-01 10:20": {{InviteeID: 20, InviterID: mokInviterID, Amount: 100}},
			"2026-11-01 11:00": {
				{InviteeID: 20, InviterID: mokInviterID, Amount: 200},
				{InviteeID: 21, InviterID: mokOtherInviterID, Amount: 40},
			},
			"2026-11-01 12:00": {{InviteeID: 20, InviterID: mokInviterID, Amount: 300}},
		},
		settled:   map[string]MokAffiliateUsageRebateRow{},
		settleErr: map[int64]error{},
	}
	svc := NewMokAffiliateUsageRebateService(NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil), nil, nil)
	svc.location = func() *time.Location { return mokCST }
	return svc, repo, settings
}

func TestMokUsageRebateSettlesEndedHoursOnce(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newMokUsageFixture(t)
	now := mokAt("2026-11-01", "12:06")

	closed, err := svc.SettleDueAt(ctx, now)
	require.NoError(t, err)
	require.Equal(t, 2, closed, "the hour from 12:00 has not ended yet")
	requirePeriods(t, repo, "2026-11-01 10:20-11:00", "2026-11-01 11:00-12:00")

	first := repo.settled["2026-11-01 10:20/20"]
	require.True(t, first.PeriodStart.Equal(mokAt("2026-11-01", "10:20")), "the first period starts at the switch to usage mode")
	require.True(t, first.PeriodEnd.Equal(mokAt("2026-11-01", "11:00")))
	require.Equal(t, "2026-11-01", first.Day)
	require.Equal(t, []float64{100, 12, 12}, []float64{first.UsageAmount, first.RatePercent, first.RebateAmount},
		"the tier follows the team's recharge volume")
	require.Equal(t, 24.0, repo.settled["2026-11-01 11:00/20"].RebateAmount)
	other := repo.settled["2026-11-01 11:00/21"]
	require.Equal(t, []float64{40, 25, 10}, []float64{other.UsageAmount, other.RatePercent, other.RebateAmount},
		"an exclusive rate applies to usage as well")
	require.Equal(t, mokOtherInviterID, other.InviterID)
	require.Equal(t, []float64{12, 24, 10}, repo.accrued)

	closed, err = svc.SettleDueAt(ctx, now)
	require.NoError(t, err)
	require.Zero(t, closed)
	require.Len(t, repo.accrued, 3, "a closed period is never settled again")

	closed, err = svc.SettleDueAt(ctx, mokAt("2026-11-01", "13:05"))
	require.NoError(t, err)
	require.Equal(t, 1, closed)
	require.Equal(t, []float64{12, 24, 10, 36}, repo.accrued)
}

func TestMokUsageRebateWaitsForTheSettleDelay(t *testing.T) {
	svc, repo, _ := newMokUsageFixture(t)

	closed, err := svc.SettleDueAt(context.Background(), mokAt("2026-11-01", "12:03"))

	require.NoError(t, err)
	require.Equal(t, 1, closed)
	requirePeriods(t, repo, "2026-11-01 10:20-11:00")
}

func TestMokUsageRebateLabelsPeriodsWithTheLocalDay(t *testing.T) {
	svc, repo, _ := newMokUsageFixture(t)
	repo.usageByPeriod["2026-11-02 00:00"] = []MokInviteeUsage{{InviteeID: 20, InviterID: mokInviterID, Amount: 50}}

	closed, err := svc.SettleDueAt(context.Background(), mokAt("2026-11-02", "01:05"))

	require.NoError(t, err)
	require.Equal(t, 15, closed)
	require.Equal(t, mokClosedPeriod{day: "2026-11-01", from: mokAt("2026-11-01", "23:00"), to: mokAt("2026-11-02", "00:00")}, repo.periods[13])
	require.Equal(t, "2026-11-02", repo.periods[14].day)
	require.Equal(t, "2026-11-02", repo.settled["2026-11-02 00:00/20"].Day)
}

func TestMokUsageRebateAppliesCapAndDuration(t *testing.T) {
	svc, repo, settings := newMokUsageFixture(t)
	settings.values[SettingKeyAffiliateRebatePerInviteeCap] = "30"
	settings.values[SettingKeyAffiliateRebateDurationDays] = "90"
	repo.existingRebate = 25

	_, err := svc.SettleDueAt(context.Background(), mokAt("2026-11-01", "11:06"))

	require.NoError(t, err)
	require.Equal(t, 5.0, repo.settled["2026-11-01 10:20/20"].RebateAmount, "12 is capped to the 5 left under the per-invitee cap")
	require.Equal(t, 90, repo.usageCalls[0].durationDays)
}

func TestMokUsageRebateClosesPeriodsWithoutRebatesWhileAffiliateIsOff(t *testing.T) {
	svc, repo, settings := newMokUsageFixture(t)
	settings.values[SettingKeyAffiliateEnabled] = "false"

	closed, err := svc.SettleDueAt(context.Background(), mokAt("2026-11-01", "12:06"))

	require.NoError(t, err)
	require.Equal(t, 2, closed)
	require.Equal(t, MokUsageRebateSkippedAffiliateDisabled, repo.periods[0].reason)
	require.Equal(t, MokUsageRebateSkippedAffiliateDisabled, repo.periods[1].reason)
	require.Empty(t, repo.usageCalls)
	require.Empty(t, repo.accrued)
}

func TestMokUsageRebateRetriesAFailedPeriodWithoutPayingTwice(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newMokUsageFixture(t)
	repo.settleErr[21] = errors.New("connection reset")
	now := mokAt("2026-11-01", "12:06")

	closed, err := svc.SettleDueAt(ctx, now)
	require.ErrorContains(t, err, "connection reset")
	require.Equal(t, 1, closed)
	requirePeriods(t, repo, "2026-11-01 10:20-11:00")
	require.Equal(t, []float64{12, 24}, repo.accrued)

	delete(repo.settleErr, 21)
	closed, err = svc.SettleDueAt(ctx, now)
	require.NoError(t, err)
	require.Equal(t, 1, closed)
	require.Equal(t, []float64{12, 24, 10}, repo.accrued, "invitee 20 is not paid again for the hour from 11:00")
}

func TestMokUsageRebateAfterSwitchingBackToRecharge(t *testing.T) {
	ctx := context.Background()
	svc, repo, settings := newMokUsageFixture(t)
	settings.values[SettingKeyMokAffiliateRebateMode] = MokAffiliateRebateModeRecharge
	settings.values[SettingKeyMokAffiliateUsageUntil] = mokAt("2026-11-01", "11:30").UTC().Format(time.RFC3339Nano)
	now := mokAt("2026-11-10", "08:00")

	closed, err := svc.SettleDueAt(ctx, now)
	require.NoError(t, err)
	require.Equal(t, 2, closed, "only the closed usage window is settled")
	requirePeriods(t, repo, "2026-11-01 10:20-11:00", "2026-11-01 11:00-11:30")

	closed, err = svc.SettleDueAt(ctx, now)
	require.NoError(t, err)
	require.Zero(t, closed, "the window is settled to its end")

	repo.periods = nil
	delete(settings.values, SettingKeyMokAffiliateUsageUntil)
	closed, err = svc.SettleDueAt(ctx, now)
	require.NoError(t, err)
	require.Zero(t, closed, "recharge mode without a closed window settles nothing")
}

func TestMokUsageRebateStartsANewWindowAfterSwitchingBackToUsage(t *testing.T) {
	svc, repo, settings := newMokUsageFixture(t)
	// An earlier window was settled up to 09:00; usage mode was switched on again at 10:20.
	repo.periods = []mokClosedPeriod{{day: "2026-11-01", from: mokAt("2026-11-01", "08:00"), to: mokAt("2026-11-01", "09:00")}}

	closed, err := svc.SettleDueAt(context.Background(), mokAt("2026-11-01", "11:06"))

	require.NoError(t, err)
	require.Equal(t, 1, closed)
	require.True(t, repo.usageCalls[0].from.Equal(mokAt("2026-11-01", "10:20")), "usage between the windows is not rebated")

	status, err := svc.ModeStatus(context.Background())
	require.NoError(t, err)
	require.True(t, status.SettledUntil.Equal(mokAt("2026-11-01", "11:00")))

	settings.values[SettingKeyMokAffiliateUsageSince] = mokAt("2026-11-01", "11:40").UTC().Format(time.RFC3339Nano)
	status, err = svc.ModeStatus(context.Background())
	require.NoError(t, err)
	require.Nil(t, status.SettledUntil, "nothing of the window from 11:40 is settled yet")
}

func TestMokUsageRebateWorksOffABacklogOverSeveralRuns(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newMokUsageFixture(t)
	now := mokAt("2026-12-20", "00:30")

	closed, err := svc.SettleDueAt(ctx, now)
	require.NoError(t, err)
	require.Equal(t, mokUsageRebateMaxPeriodsPerRun, closed)
	last := repo.periods[len(repo.periods)-1].to

	closed, err = svc.SettleDueAt(ctx, now)
	require.NoError(t, err)
	require.Positive(t, closed)
	require.True(t, repo.periods[mokUsageRebateMaxPeriodsPerRun].from.Equal(last), "the next run continues where the last one stopped")
	require.True(t, repo.periods[len(repo.periods)-1].to.Equal(mokAt("2026-12-20", "00:00")))
	require.Equal(t, []float64{12, 24, 10, 36}, repo.accrued)
}

func TestMokNextHour(t *testing.T) {
	require.True(t, mokNextHour(mokAt("2026-11-01", "10:20"), mokCST).Equal(mokAt("2026-11-01", "11:00")))
	require.True(t, mokNextHour(mokAt("2026-11-01", "11:00"), mokCST).Equal(mokAt("2026-11-01", "12:00")), "a full hour is followed by the next one")
	require.True(t, mokNextHour(mokAt("2026-11-01", "23:59").Add(59*time.Second+999*time.Millisecond), mokCST).Equal(mokAt("2026-11-02", "00:00")))

	// Hours follow the clock of the time zone, also where it is not a whole hour off UTC.
	india := time.FixedZone("IST", 5*3600+30*60)
	at := time.Date(2026, 11, 1, 10, 20, 0, 0, india)
	require.True(t, mokNextHour(at.UTC(), india).Equal(time.Date(2026, 11, 1, 11, 0, 0, 0, india)))
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

	require.NoError(t, settings.MokSetAffiliateRebateMode(ctx, MokAffiliateRebateModeUsage, switchOn.Add(1234567*time.Nanosecond)))
	require.Equal(t, MokAffiliateRebateModeUsage, repo.values[SettingKeyMokAffiliateRebateMode])
	require.NotContains(t, repo.values, SettingKeyMokAffiliateUsageUntil)
	window, err := settings.MokGetAffiliateUsageWindow(ctx)
	require.NoError(t, err)
	require.True(t, window.Since.Equal(switchOn.Add(1234*time.Microsecond)), "the database keeps microseconds, so the window does too")
	require.Nil(t, window.Until)

	require.NoError(t, settings.MokSetAffiliateRebateMode(ctx, MokAffiliateRebateModeUsage, switchOff))
	window, _ = settings.MokGetAffiliateUsageWindow(ctx)
	require.True(t, window.Since.Equal(switchOn.Add(1234*time.Microsecond)), "setting the current mode again keeps the window")

	require.NoError(t, settings.MokSetAffiliateRebateMode(ctx, MokAffiliateRebateModeRecharge, switchOff))
	window, _ = settings.MokGetAffiliateUsageWindow(ctx)
	require.True(t, window.Until.Equal(switchOff))

	err = settings.MokSetAffiliateRebateMode(ctx, "monthly", switchOff)
	require.Equal(t, "MOK_AFFILIATE_REBATE_MODE_INVALID", infraerrors.Reason(err))

	// A window stored by an earlier version with more digits is read with microseconds.
	repo.values[SettingKeyMokAffiliateUsageSince] = "2026-11-09T09:14:06.6734146Z"
	window, _ = settings.MokGetAffiliateUsageWindow(ctx)
	require.Equal(t, 673414000, window.Since.Nanosecond())
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

func TestMokUsageRebateRecordLists(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newMokUsageFixture(t)
	repo.records = []MokAffiliateUsageRebateRecord{{
		PeriodStart:  mokAt("2026-11-01", "11:00"),
		PeriodEnd:    mokAt("2026-11-01", "12:00"),
		InviterID:    mokInviterID,
		InviterEmail: "inviter@example.com",
		InviteeID:    20,
		InviteeEmail: "invitee@example.com",
		UsageAmount:  200,
		RatePercent:  12,
		RebateAmount: 24,
	}}

	records, total, err := svc.ListRecords(ctx, MokAffiliateUsageRebateFilter{Search: "  invitee  ", PageSize: 1000})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "invitee@example.com", records[0].InviteeEmail, "admins see who it is")
	require.Equal(t, MokAffiliateUsageRebateFilter{Search: "invitee", Page: 1, PageSize: mokUsageRebateMaxPageSize}, repo.listFilters[0])

	records, total, err = svc.ListRecordsForInviter(ctx, mokInviterID, 2, 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, MokAffiliateUsageRebateFilter{InviterID: mokInviterID, Page: 2, PageSize: mokUsageRebateDefaultPageSize}, repo.listFilters[1])
	own := records[0]
	require.Equal(t, maskEmail("invitee@example.com"), own.InviteeEmail)
	require.NotEqual(t, "invitee@example.com", own.InviteeEmail)
	require.Zero(t, own.InviteeID)
	require.Zero(t, own.InviterID)
	require.Empty(t, own.InviterEmail)
	require.Equal(t, 24.0, own.RebateAmount)

	_, _, err = svc.ListRecordsForInviter(ctx, 0, 1, 20)
	require.Equal(t, "INVALID_USER", infraerrors.Reason(err))

	repo.records = nil
	records, _, err = svc.ListRecords(ctx, MokAffiliateUsageRebateFilter{})
	require.NoError(t, err)
	require.NotNil(t, records, "an empty list is sent as an empty array")
}
