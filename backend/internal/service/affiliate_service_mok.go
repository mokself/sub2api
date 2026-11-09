package service

// [mok] Fork-only file (see FORK.md): tiered affiliate rebate rates.
//
// An inviter's rebate rate follows the cumulative recharge of the inviter and every
// user they invited ("team volume"). Once the volume, including the recharge being
// rebated, reaches a tier's MinAmount, the whole recharge is rebated at that tier's
// rate. Below the first tier the global rate applies, and inviters with an exclusive
// rate keep it. Without tiers everything behaves as upstream. Upstream reaches this
// file through [mok] seams in affiliate_service.go.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	// SettingKeyMokAffiliateRebateTiers stores the tiers as a JSON array; missing or empty disables them.
	SettingKeyMokAffiliateRebateTiers = "mok_affiliate_rebate_tiers"

	mokAffiliateRebateTiersMax = 20
	// Team volumes are sums of decimal(20,2) amounts; this absorbs float noise at a threshold.
	mokAffiliateVolumeEpsilon = 1e-6
)

// MokAffiliateRebateTier switches the inviter to RatePercent once the team volume reaches MinAmount.
type MokAffiliateRebateTier struct {
	MinAmount   float64 `json:"min_amount"`
	RatePercent float64 `json:"rate_percent"`
}

// MokAffiliateTeamVolumeReader is implemented by the affiliate repository in
// internal/repository/affiliate_repo_mok.go.
type MokAffiliateTeamVolumeReader interface {
	// MokTeamRechargeVolume sums the recharges of the inviter and every user they invited:
	// completed, unrefunded payment orders (balance orders without their bonus) plus used
	// balance redeem codes that payment orders did not issue. excludeOrderID leaves out a
	// payment order that is still being fulfilled.
	MokTeamRechargeVolume(ctx context.Context, inviterID int64, excludeOrderID *int64) (float64, error)
}

// MokAffiliateTierStatus is what an inviter sees about the tiers on their affiliate page.
type MokAffiliateTierStatus struct {
	// RebateMode is MokAffiliateRebateModeRecharge or MokAffiliateRebateModeUsage while the
	// affiliate feature is on, and empty while it is off.
	RebateMode string `json:"rebate_mode"`
	// Enabled is true when the affiliate feature is on and tiers are configured.
	Enabled bool `json:"enabled"`
	// Exclusive is true when the user has an exclusive rate, which tiers do not change.
	Exclusive          bool                     `json:"exclusive"`
	BaseRatePercent    float64                  `json:"base_rate_percent"`
	CurrentRatePercent float64                  `json:"current_rate_percent"`
	TeamVolume         float64                  `json:"team_volume"`
	Tiers              []MokAffiliateRebateTier `json:"tiers"`
	NextTier           *MokAffiliateRebateTier  `json:"next_tier,omitempty"`
	NextTierRemaining  float64                  `json:"next_tier_remaining"`
}

func mokInvalidAffiliateTiers(format string, args ...any) error {
	return infraerrors.Newf(http.StatusBadRequest, "MOK_AFFILIATE_REBATE_TIERS_INVALID", format, args...)
}

// NormalizeMokAffiliateRebateTiers validates tiers, rounds both fields to two decimals and
// sorts them by MinAmount. Row numbers in errors are 1-based positions in the input.
func NormalizeMokAffiliateRebateTiers(raw []MokAffiliateRebateTier) ([]MokAffiliateRebateTier, error) {
	if len(raw) > mokAffiliateRebateTiersMax {
		return nil, mokInvalidAffiliateTiers("at most %d tiers are allowed", mokAffiliateRebateTiersMax)
	}
	tiers := make([]MokAffiliateRebateTier, 0, len(raw))
	seen := make(map[float64]bool, len(raw))
	for i, tier := range raw {
		minAmount := tier.MinAmount
		if math.IsNaN(minAmount) || math.IsInf(minAmount, 0) || roundTo(minAmount, 2) <= 0 {
			return nil, mokInvalidAffiliateTiers("tier %d: min_amount must be greater than 0", i+1)
		}
		rate := tier.RatePercent
		if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < AffiliateRebateRateMin || rate > AffiliateRebateRateMax {
			return nil, mokInvalidAffiliateTiers("tier %d: rate_percent must be between %g and %g", i+1, AffiliateRebateRateMin, AffiliateRebateRateMax)
		}
		minAmount = roundTo(minAmount, 2)
		if seen[minAmount] {
			return nil, mokInvalidAffiliateTiers("tier %d: min_amount %.2f is used twice", i+1, minAmount)
		}
		seen[minAmount] = true
		tiers = append(tiers, MokAffiliateRebateTier{MinAmount: minAmount, RatePercent: roundTo(rate, 2)})
	}
	sort.Slice(tiers, func(i, j int) bool { return tiers[i].MinAmount < tiers[j].MinAmount })
	return tiers, nil
}

// MokGetAffiliateRebateTiers returns the configured tiers sorted by MinAmount, or none.
// Read failures are returned so a rebate is retried instead of silently paid at the base
// rate; a stored value that no longer parses is logged and treated as no tiers.
func (s *SettingService) MokGetAffiliateRebateTiers(ctx context.Context) ([]MokAffiliateRebateTier, error) {
	if s == nil || s.settingRepo == nil {
		return nil, nil
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyMokAffiliateRebateTiers)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("read affiliate rebate tiers: %w", err)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var stored []MokAffiliateRebateTier
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		logger.LegacyPrintf("service.affiliate", "[Affiliate][mok] ignoring unreadable %s: %v", SettingKeyMokAffiliateRebateTiers, err)
		return nil, nil
	}
	tiers, err := NormalizeMokAffiliateRebateTiers(stored)
	if err != nil {
		logger.LegacyPrintf("service.affiliate", "[Affiliate][mok] ignoring invalid %s: %v", SettingKeyMokAffiliateRebateTiers, err)
		return nil, nil
	}
	return tiers, nil
}

// MokSetAffiliateRebateTiers validates and stores the tiers; an empty list disables them.
// It returns the tiers as stored.
func (s *SettingService) MokSetAffiliateRebateTiers(ctx context.Context, raw []MokAffiliateRebateTier) ([]MokAffiliateRebateTier, error) {
	if s == nil || s.settingRepo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "setting service unavailable")
	}
	tiers, err := NormalizeMokAffiliateRebateTiers(raw)
	if err != nil {
		return nil, err
	}
	if len(tiers) == 0 {
		if err := s.settingRepo.Delete(ctx, SettingKeyMokAffiliateRebateTiers); err != nil {
			return nil, fmt.Errorf("clear affiliate rebate tiers: %w", err)
		}
		return tiers, nil
	}
	data, err := json.Marshal(tiers)
	if err != nil {
		return nil, fmt.Errorf("encode affiliate rebate tiers: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyMokAffiliateRebateTiers, string(data)); err != nil {
		return nil, fmt.Errorf("save affiliate rebate tiers: %w", err)
	}
	return tiers, nil
}

// mokRateForVolume returns the rate of the highest tier the volume reaches, or baseRate
// below the first tier. tiers must be sorted by MinAmount.
func mokRateForVolume(tiers []MokAffiliateRebateTier, volume, baseRate float64) float64 {
	rate := baseRate
	for _, tier := range tiers {
		if volume+mokAffiliateVolumeEpsilon < tier.MinAmount {
			break
		}
		rate = clampAffiliateRebateRate(tier.RatePercent)
	}
	return rate
}

func (s *AffiliateService) mokTeamVolume(ctx context.Context, inviterID int64, excludeOrderID *int64) (float64, error) {
	reader, ok := s.repo.(MokAffiliateTeamVolumeReader)
	if !ok {
		return 0, errors.New("affiliate repository cannot report team recharge volume")
	}
	volume, err := reader.MokTeamRechargeVolume(ctx, inviterID, excludeOrderID)
	if err != nil {
		return 0, fmt.Errorf("query affiliate team recharge volume: %w", err)
	}
	return volume, nil
}

// mokTieredRebateRatePercent is called by AccrueInviteRebateForOrder through a [mok] seam,
// after upstream resolved baseRate (exclusive or global), and returns the rate for that
// recharge. In usage mode recharges earn nothing (rate 0, so upstream books no rebate);
// rebates come from the daily settlement in affiliate_usage_rebate_mok.go instead.
func (s *AffiliateService) mokTieredRebateRatePercent(ctx context.Context, inviter *AffiliateSummary, baseRate, rechargeAmount float64, sourceOrderID *int64) (float64, error) {
	if s != nil && s.settingService != nil {
		mode, err := s.settingService.MokGetAffiliateRebateMode(ctx)
		if err != nil {
			return 0, err
		}
		if mode == MokAffiliateRebateModeUsage {
			return 0, nil
		}
	}
	return s.mokTierRatePercent(ctx, inviter, baseRate, rechargeAmount, sourceOrderID)
}

// mokTierRatePercent returns the inviter's rate: baseRate for an exclusive rate or without
// tiers, otherwise the tier reached by the team's recharge volume.
//
// For a payment order being rebated (sourceOrderID set) the order is not yet completed, so
// its amount is added to the volume. A redeem code is already marked used when its rebate
// runs and is part of the volume; admin top-ups are not part of the volume at all.
func (s *AffiliateService) mokTierRatePercent(ctx context.Context, inviter *AffiliateSummary, baseRate, rechargeAmount float64, sourceOrderID *int64) (float64, error) {
	if s == nil || s.settingService == nil || inviter == nil || inviter.UserID <= 0 || inviter.AffRebateRatePercent != nil {
		return baseRate, nil
	}
	tiers, err := s.settingService.MokGetAffiliateRebateTiers(ctx)
	if err != nil {
		return 0, err
	}
	if len(tiers) == 0 {
		return baseRate, nil
	}
	volume, err := s.mokTeamVolume(ctx, inviter.UserID, sourceOrderID)
	if err != nil {
		return 0, err
	}
	if sourceOrderID != nil && rechargeAmount > 0 {
		volume += rechargeAmount
	}
	return mokRateForVolume(tiers, volume, baseRate), nil
}

// mokEffectiveRebateRatePercent replaces resolveRebateRatePercent on the user's affiliate
// page, so inviters without an exclusive rate see the rate of their current tier.
func (s *AffiliateService) mokEffectiveRebateRatePercent(ctx context.Context, summary *AffiliateSummary) float64 {
	base := s.resolveRebateRatePercent(ctx, summary)
	rate, err := s.mokTierRatePercent(ctx, summary, base, 0, nil)
	if err != nil {
		logger.LegacyPrintf("service.affiliate", "[Affiliate][mok] tier rate unavailable for user %d, showing base rate: %v", affiliateSummaryUserID(summary), err)
		return base
	}
	return rate
}

// mokTieredDisplayRatePercent is used by AdminGetUserOverview for users without an
// exclusive rate, so the admin sees the rate of the user's current tier.
func (s *AffiliateService) mokTieredDisplayRatePercent(ctx context.Context, userID int64, globalRate float64) float64 {
	rate, err := s.mokTierRatePercent(ctx, &AffiliateSummary{UserID: userID}, globalRate, 0, nil)
	if err != nil {
		logger.LegacyPrintf("service.affiliate", "[Affiliate][mok] tier rate unavailable for user %d, showing global rate: %v", userID, err)
		return globalRate
	}
	return rate
}

func affiliateSummaryUserID(summary *AffiliateSummary) int64 {
	if summary == nil {
		return 0
	}
	return summary.UserID
}

// MokAffiliateTierStatus reports the tiers and the user's progress through them.
func (s *AffiliateService) MokAffiliateTierStatus(ctx context.Context, userID int64) (*MokAffiliateTierStatus, error) {
	status := &MokAffiliateTierStatus{Tiers: []MokAffiliateRebateTier{}}
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil || s.settingService == nil || !s.IsEnabled(ctx) {
		return status, nil
	}
	mode, err := s.settingService.MokGetAffiliateRebateMode(ctx)
	if err != nil {
		return nil, err
	}
	status.RebateMode = mode
	tiers, err := s.settingService.MokGetAffiliateRebateTiers(ctx)
	if err != nil {
		return nil, err
	}
	if len(tiers) == 0 {
		return status, nil
	}
	summary, err := s.repo.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return nil, err
	}
	status.Enabled = true
	status.Tiers = tiers
	status.BaseRatePercent = s.globalRebateRatePercent(ctx)
	if summary.AffRebateRatePercent != nil {
		status.Exclusive = true
		status.CurrentRatePercent = s.resolveRebateRatePercent(ctx, summary)
		return status, nil
	}
	volume, err := s.mokTeamVolume(ctx, userID, nil)
	if err != nil {
		return nil, err
	}
	status.TeamVolume = roundTo(volume, 2)
	status.CurrentRatePercent = mokRateForVolume(tiers, volume, status.BaseRatePercent)
	for _, tier := range tiers {
		if volume+mokAffiliateVolumeEpsilon < tier.MinAmount {
			next := tier
			status.NextTier = &next
			status.NextTierRemaining = roundTo(tier.MinAmount-volume, 2)
			break
		}
	}
	return status, nil
}
