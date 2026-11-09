//go:build unit

package service

// [mok] Fork-only tests for affiliate_service_mok.go (see FORK.md).

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// mokSettingRepo is an in-memory SettingRepository.
type mokSettingRepo struct {
	mu     sync.Mutex
	values map[string]string
	getErr error
}

func (r *mokSettingRepo) Get(ctx context.Context, key string) (*Setting, error) {
	value, err := r.GetValue(ctx, key)
	if err != nil {
		return nil, err
	}
	return &Setting{Key: key, Value: value}, nil
}

func (r *mokSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil {
		return "", r.getErr
	}
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *mokSettingRepo) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values[key] = value
	return nil
}

func (r *mokSettingRepo) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, key := range keys {
		if value, err := r.GetValue(ctx, key); err == nil {
			out[key] = value
		}
	}
	return out, nil
}

func (r *mokSettingRepo) SetMultiple(ctx context.Context, settings map[string]string) error {
	for key, value := range settings {
		_ = r.Set(ctx, key, value)
	}
	return nil
}

func (r *mokSettingRepo) GetAll(context.Context) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(r.values))
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *mokSettingRepo) Delete(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.values, key)
	return nil
}

type mokVolumeCall struct {
	inviterID      int64
	excludeOrderID *int64
}

// mokAffiliateRepo implements the parts of AffiliateRepository the rebate paths use, plus
// MokAffiliateTeamVolumeReader. Calling any other method panics through the nil embedding.
type mokAffiliateRepo struct {
	AffiliateRepository
	summaries   map[int64]*AffiliateSummary
	volume      float64
	volumeErr   error
	volumeCalls []mokVolumeCall
	accrued     []float64
	overview    *AffiliateUserOverview
	// existingRebate is what GetAccruedRebateFromInvitee reports for every pair.
	existingRebate float64
}

func (r *mokAffiliateRepo) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	if summary, ok := r.summaries[userID]; ok {
		copied := *summary
		return &copied, nil
	}
	return &AffiliateSummary{UserID: userID, AffCode: "CODE", CreatedAt: time.Now()}, nil
}

func (r *mokAffiliateRepo) AccrueQuota(_ context.Context, _, _ int64, amount float64, _ int, _ *int64) (bool, error) {
	r.accrued = append(r.accrued, amount)
	return true, nil
}

func (r *mokAffiliateRepo) GetAccruedRebateFromInvitee(context.Context, int64, int64) (float64, error) {
	return r.existingRebate, nil
}

func (r *mokAffiliateRepo) ThawFrozenQuota(context.Context, int64) (float64, error) {
	return 0, nil
}

func (r *mokAffiliateRepo) ListInvitees(context.Context, int64, int) ([]AffiliateInvitee, error) {
	return []AffiliateInvitee{}, nil
}

func (r *mokAffiliateRepo) GetAffiliateUserOverview(context.Context, int64) (*AffiliateUserOverview, error) {
	copied := *r.overview
	return &copied, nil
}

func (r *mokAffiliateRepo) MokTeamRechargeVolume(_ context.Context, inviterID int64, excludeOrderID *int64) (float64, error) {
	r.volumeCalls = append(r.volumeCalls, mokVolumeCall{inviterID: inviterID, excludeOrderID: excludeOrderID})
	return r.volume, r.volumeErr
}

const (
	mokInviterID = int64(10)
	mokInviteeID = int64(20)
	mokOrderID   = int64(900)
	// 10% globally, 12% from 50000, 15% from 100000.
	mokTiersJSON = `[{"min_amount":100000,"rate_percent":15},{"min_amount":50000,"rate_percent":12}]`
)

func newMokAffiliateFixture(t *testing.T, tiersJSON string) (*AffiliateService, *mokAffiliateRepo, *mokSettingRepo) {
	t.Helper()
	settings := &mokSettingRepo{values: map[string]string{
		SettingKeyAffiliateEnabled:    "true",
		SettingKeyAffiliateRebateRate: "10",
	}}
	if tiersJSON != "" {
		settings.values[SettingKeyMokAffiliateRebateTiers] = tiersJSON
	}
	inviterID := mokInviterID
	repo := &mokAffiliateRepo{summaries: map[int64]*AffiliateSummary{
		mokInviterID: {UserID: mokInviterID, AffCode: "INVITER", CreatedAt: time.Now()},
		mokInviteeID: {UserID: mokInviteeID, AffCode: "INVITEE", InviterID: &inviterID, CreatedAt: time.Now()},
	}}
	return NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil), repo, settings
}

func TestNormalizeMokAffiliateRebateTiers(t *testing.T) {
	tiers, err := NormalizeMokAffiliateRebateTiers([]MokAffiliateRebateTier{
		{MinAmount: 100000, RatePercent: 15},
		{MinAmount: 50000.004, RatePercent: 12.345},
	})
	require.NoError(t, err)
	require.Equal(t, []MokAffiliateRebateTier{
		{MinAmount: 50000, RatePercent: 12.35},
		{MinAmount: 100000, RatePercent: 15},
	}, tiers)

	empty, err := NormalizeMokAffiliateRebateTiers(nil)
	require.NoError(t, err)
	require.NotNil(t, empty)
	require.Empty(t, empty)

	tooMany := make([]MokAffiliateRebateTier, mokAffiliateRebateTiersMax+1)
	for i := range tooMany {
		tooMany[i] = MokAffiliateRebateTier{MinAmount: float64(i + 1), RatePercent: 1}
	}
	for name, input := range map[string][]MokAffiliateRebateTier{
		"zero threshold":      {{MinAmount: 0, RatePercent: 5}},
		"rounds to zero":      {{MinAmount: 0.004, RatePercent: 5}},
		"negative threshold":  {{MinAmount: -1, RatePercent: 5}},
		"NaN threshold":       {{MinAmount: math.NaN(), RatePercent: 5}},
		"rate above 100":      {{MinAmount: 1, RatePercent: 100.01}},
		"negative rate":       {{MinAmount: 1, RatePercent: -1}},
		"infinite rate":       {{MinAmount: 1, RatePercent: math.Inf(1)}},
		"duplicate threshold": {{MinAmount: 500, RatePercent: 5}, {MinAmount: 500.001, RatePercent: 6}},
		"too many tiers":      tooMany,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NormalizeMokAffiliateRebateTiers(input)
			require.Equal(t, "MOK_AFFILIATE_REBATE_TIERS_INVALID", infraerrors.Reason(err))
		})
	}
}

func TestMokRateForVolume(t *testing.T) {
	tiers := []MokAffiliateRebateTier{{MinAmount: 50000, RatePercent: 12}, {MinAmount: 100000, RatePercent: 15}}
	for _, tc := range []struct {
		volume float64
		want   float64
	}{
		{0, 10}, {49999.99, 10}, {50000, 12}, {49999.9999999, 12}, {99999.99, 12}, {100000, 15}, {1e9, 15},
	} {
		require.Equal(t, tc.want, mokRateForVolume(tiers, tc.volume, 10), "volume %v", tc.volume)
	}
	require.Equal(t, 10.0, mokRateForVolume(nil, 1e9, 10))
}

func TestMokAffiliateRebateTiersSettingRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := &mokSettingRepo{values: map[string]string{}}
	settings := NewSettingService(repo, nil)

	tiers, err := settings.MokGetAffiliateRebateTiers(ctx)
	require.NoError(t, err)
	require.Empty(t, tiers)

	saved, err := settings.MokSetAffiliateRebateTiers(ctx, []MokAffiliateRebateTier{{MinAmount: 100000, RatePercent: 15}, {MinAmount: 50000, RatePercent: 12}})
	require.NoError(t, err)
	require.Equal(t, `[{"min_amount":50000,"rate_percent":12},{"min_amount":100000,"rate_percent":15}]`, repo.values[SettingKeyMokAffiliateRebateTiers])
	loaded, err := settings.MokGetAffiliateRebateTiers(ctx)
	require.NoError(t, err)
	require.Equal(t, saved, loaded)

	_, err = settings.MokSetAffiliateRebateTiers(ctx, []MokAffiliateRebateTier{{MinAmount: 1, RatePercent: 101}})
	require.Error(t, err)
	require.Contains(t, repo.values, SettingKeyMokAffiliateRebateTiers, "an invalid update must keep the stored tiers")

	cleared, err := settings.MokSetAffiliateRebateTiers(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, cleared)
	require.NotContains(t, repo.values, SettingKeyMokAffiliateRebateTiers)

	repo.values[SettingKeyMokAffiliateRebateTiers] = `{not json`
	tiers, err = settings.MokGetAffiliateRebateTiers(ctx)
	require.NoError(t, err, "an unreadable stored value must not block rebates")
	require.Empty(t, tiers)

	repo.getErr = errors.New("connection reset")
	_, err = settings.MokGetAffiliateRebateTiers(ctx)
	require.ErrorContains(t, err, "connection reset")
}

func TestAccrueInviteRebateUsesTierRate(t *testing.T) {
	ctx := context.Background()
	orderID := mokOrderID
	for _, tc := range []struct {
		name          string
		volume        float64
		amount        float64
		sourceOrderID *int64
		wantRebate    float64
	}{
		// The order is still being fulfilled, so it is added to the volume: 45000 + 10000 crosses 50000.
		{name: "order that crosses the first tier earns it whole", volume: 45000, amount: 10000, sourceOrderID: &orderID, wantRebate: 1200},
		{name: "order below the first tier earns the global rate", volume: 30000, amount: 10000, sourceOrderID: &orderID, wantRebate: 1000},
		{name: "order reaching the top tier", volume: 95000, amount: 20000, sourceOrderID: &orderID, wantRebate: 3000},
		// A redeem code is already part of the volume when its rebate runs.
		{name: "redeem code is not added twice", volume: 50000, amount: 1000, wantRebate: 120},
		{name: "redeem code below the tier", volume: 49000, amount: 1000, wantRebate: 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, _ := newMokAffiliateFixture(t, mokTiersJSON)
			repo.volume = tc.volume
			rebate, err := svc.AccrueInviteRebateForOrder(ctx, mokInviteeID, tc.amount, tc.sourceOrderID)
			require.NoError(t, err)
			require.InDelta(t, tc.wantRebate, rebate, 1e-9)
			require.Equal(t, []float64{rebate}, repo.accrued)
			require.Len(t, repo.volumeCalls, 1)
			require.Equal(t, mokInviterID, repo.volumeCalls[0].inviterID, "the volume is the inviter's team, not the invitee's")
			require.Equal(t, tc.sourceOrderID, repo.volumeCalls[0].excludeOrderID)
		})
	}
}

func TestAccrueInviteRebateKeepsExclusiveRateAndUpstreamBehaviour(t *testing.T) {
	ctx := context.Background()
	orderID := mokOrderID

	svc, repo, _ := newMokAffiliateFixture(t, mokTiersJSON)
	exclusive := 25.0
	repo.summaries[mokInviterID].AffRebateRatePercent = &exclusive
	repo.volume = 200000
	rebate, err := svc.AccrueInviteRebateForOrder(ctx, mokInviteeID, 1000, &orderID)
	require.NoError(t, err)
	require.InDelta(t, 250, rebate, 1e-9, "an exclusive rate wins over the tiers")
	require.Empty(t, repo.volumeCalls)

	svc, repo, _ = newMokAffiliateFixture(t, "")
	repo.volume = 200000
	rebate, err = svc.AccrueInviteRebateForOrder(ctx, mokInviteeID, 1000, &orderID)
	require.NoError(t, err)
	require.InDelta(t, 100, rebate, 1e-9, "without tiers the global rate applies")
	require.Empty(t, repo.volumeCalls)
}

func TestAccrueInviteRebateFailsWhenVolumeIsUnavailable(t *testing.T) {
	svc, repo, _ := newMokAffiliateFixture(t, mokTiersJSON)
	repo.volumeErr = errors.New("statement timeout")
	orderID := mokOrderID

	_, err := svc.AccrueInviteRebateForOrder(context.Background(), mokInviteeID, 1000, &orderID)

	require.ErrorContains(t, err, "statement timeout")
	require.Empty(t, repo.accrued, "no rebate may be booked at a guessed rate")
}

func TestMokTierRateOnDisplayPaths(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newMokAffiliateFixture(t, mokTiersJSON)
	repo.volume = 60000

	detail, err := svc.GetAffiliateDetail(ctx, mokInviterID)
	require.NoError(t, err)
	require.Equal(t, 12.0, detail.EffectiveRebateRatePercent)
	require.Nil(t, repo.volumeCalls[0].excludeOrderID)

	repo.overview = &AffiliateUserOverview{UserID: mokInviterID, RebateRatePercent: 0}
	overview, err := svc.AdminGetUserOverview(ctx, mokInviterID)
	require.NoError(t, err)
	require.Equal(t, 12.0, overview.RebateRatePercent)

	repo.overview = &AffiliateUserOverview{UserID: mokInviterID, RebateRatePercent: 30, RebateRateCustom: true}
	overview, err = svc.AdminGetUserOverview(ctx, mokInviterID)
	require.NoError(t, err)
	require.Equal(t, 30.0, overview.RebateRatePercent, "a custom rate is shown as is")

	repo.volumeErr = errors.New("statement timeout")
	detail, err = svc.GetAffiliateDetail(ctx, mokInviterID)
	require.NoError(t, err, "a display must not fail because the volume is unavailable")
	require.Equal(t, 10.0, detail.EffectiveRebateRatePercent)
}

func TestMokAffiliateTierStatus(t *testing.T) {
	ctx := context.Background()

	svc, repo, _ := newMokAffiliateFixture(t, mokTiersJSON)
	repo.volume = 60000.5
	status, err := svc.MokAffiliateTierStatus(ctx, mokInviterID)
	require.NoError(t, err)
	require.True(t, status.Enabled)
	require.False(t, status.Exclusive)
	require.Equal(t, 10.0, status.BaseRatePercent)
	require.Equal(t, 12.0, status.CurrentRatePercent)
	require.Equal(t, 60000.5, status.TeamVolume)
	require.Equal(t, &MokAffiliateRebateTier{MinAmount: 100000, RatePercent: 15}, status.NextTier)
	require.Equal(t, 39999.5, status.NextTierRemaining)

	repo.volume = 150000
	status, err = svc.MokAffiliateTierStatus(ctx, mokInviterID)
	require.NoError(t, err)
	require.Equal(t, 15.0, status.CurrentRatePercent)
	require.Nil(t, status.NextTier)

	exclusive := 25.0
	repo.summaries[mokInviterID].AffRebateRatePercent = &exclusive
	status, err = svc.MokAffiliateTierStatus(ctx, mokInviterID)
	require.NoError(t, err)
	require.True(t, status.Exclusive)
	require.Equal(t, 25.0, status.CurrentRatePercent)

	svc, _, settings := newMokAffiliateFixture(t, mokTiersJSON)
	settings.values[SettingKeyAffiliateEnabled] = "false"
	status, err = svc.MokAffiliateTierStatus(ctx, mokInviterID)
	require.NoError(t, err)
	require.False(t, status.Enabled)
	require.NotNil(t, status.Tiers)

	svc, _, _ = newMokAffiliateFixture(t, "")
	status, err = svc.MokAffiliateTierStatus(ctx, mokInviterID)
	require.NoError(t, err)
	require.False(t, status.Enabled)
}

func TestMokTeamVolumeRequiresRepositorySupport(t *testing.T) {
	settings := &mokSettingRepo{values: map[string]string{
		SettingKeyAffiliateEnabled:        "true",
		SettingKeyMokAffiliateRebateTiers: mokTiersJSON,
	}}
	svc := NewAffiliateService(&struct{ AffiliateRepository }{}, NewSettingService(settings, nil), nil, nil)

	_, err := svc.mokTieredRebateRatePercent(context.Background(), &AffiliateSummary{UserID: mokInviterID}, 10, 1000, nil)

	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "team recharge volume"))
}
