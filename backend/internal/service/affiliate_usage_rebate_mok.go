package service

// [mok] Fork-only file (see FORK.md): daily affiliate rebates on invitees' balance usage.
//
// In usage mode recharges earn no rebate (mokTieredRebateRatePercent returns 0). Instead,
// once a day has ended, every invitee's balance usage of that day (usage_logs.actual_cost
// billed to the balance) is rebated to their inviter at the inviter's rate: the exclusive
// rate, else the tier reached by the team's recharge volume, else the global rate. Only
// usage inside the usage window counts: from the switch to usage mode until the switch
// back. The per-invitee cap, rebate duration and freeze period apply as for recharges, and
// days that end while the affiliate feature is off are closed without rebates.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	MokAffiliateRebateModeRecharge = "recharge"
	MokAffiliateRebateModeUsage    = "usage"

	SettingKeyMokAffiliateRebateMode = "mok_affiliate_rebate_mode"
	// RFC 3339 times bounding the usage window; Until is only set after switching back.
	SettingKeyMokAffiliateUsageSince = "mok_affiliate_usage_rebate_since"
	SettingKeyMokAffiliateUsageUntil = "mok_affiliate_usage_rebate_until"

	// MokUsageRebateSkippedAffiliateDisabled marks a day closed while the affiliate feature was off.
	MokUsageRebateSkippedAffiliateDisabled = "affiliate_disabled"

	mokUsageRebateDayLayout = "2006-01-02"
	// A day is settled this long after it ends, so usage logs written asynchronously are in.
	mokUsageRebateSettleDelay   = 30 * time.Minute
	mokUsageRebateFirstTick     = time.Minute
	mokUsageRebateTick          = 5 * time.Minute
	mokUsageRebateRunTimeout    = 25 * time.Minute
	mokUsageRebateLockKey       = "mok:affiliate_usage_rebate"
	mokUsageRebateLockTTL       = 30 * time.Minute
	mokUsageRebateMaxDaysPerRun = 62
	mokUsageRebateRecentDays    = 14
)

// MokInviteeUsage is one invitee's balance usage over a settlement window.
type MokInviteeUsage struct {
	InviteeID int64
	InviterID int64
	Amount    float64
}

// MokAffiliateUsageRebateRow is one invitee's settled day.
type MokAffiliateUsageRebateRow struct {
	Day          string
	InviteeID    int64
	InviterID    int64
	UsageAmount  float64
	RatePercent  float64
	RebateAmount float64
}

// MokAffiliateUsageRebateDay summarises a closed day.
type MokAffiliateUsageRebateDay struct {
	Day           string    `json:"day"`
	WindowStart   time.Time `json:"window_start"`
	WindowEnd     time.Time `json:"window_end"`
	InviteeCount  int       `json:"invitee_count"`
	UsageTotal    float64   `json:"usage_total"`
	RebateTotal   float64   `json:"rebate_total"`
	SkippedReason string    `json:"skipped_reason,omitempty"`
	SettledAt     time.Time `json:"settled_at"`
}

// MokAffiliateUsageRebateStore is implemented by the affiliate repository in
// internal/repository/affiliate_repo_mok.go.
type MokAffiliateUsageRebateStore interface {
	// MokBalanceUsageByInvitee sums balance-billed usage with a positive actual cost in
	// [from, to) for every user who has an inviter. durationDays > 0 drops usage later than
	// that many days after the invitee's affiliate profile was created.
	MokBalanceUsageByInvitee(ctx context.Context, from, to time.Time, durationDays int) ([]MokInviteeUsage, error)
	// MokSettleUsageRebate records the row and books its rebate in one transaction, and
	// returns false when that invitee's day was already settled.
	MokSettleUsageRebate(ctx context.Context, row MokAffiliateUsageRebateRow, freezeHours int) (bool, error)
	// MokCloseUsageRebateDay records the day with totals over its settled rows.
	MokCloseUsageRebateDay(ctx context.Context, day string, windowStart, windowEnd time.Time, skippedReason string) error
	// MokUsageRebateClosedDays returns the closed days from fromDay on, as YYYY-MM-DD.
	MokUsageRebateClosedDays(ctx context.Context, fromDay string) (map[string]bool, error)
	// MokRecentUsageRebateDays returns the latest closed days, newest first.
	MokRecentUsageRebateDays(ctx context.Context, limit int) ([]MokAffiliateUsageRebateDay, error)
}

// MokAffiliateUsageWindow bounds the usage that earns rebates. Since is nil until usage mode
// was first switched on; Until is the last switch back to recharge mode.
type MokAffiliateUsageWindow struct {
	Since *time.Time
	Until *time.Time
}

// MokGetAffiliateRebateMode returns the rebate mode; anything but "usage" means recharge.
func (s *SettingService) MokGetAffiliateRebateMode(ctx context.Context) (string, error) {
	if s == nil || s.settingRepo == nil {
		return MokAffiliateRebateModeRecharge, nil
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyMokAffiliateRebateMode)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return MokAffiliateRebateModeRecharge, nil
		}
		return "", fmt.Errorf("read affiliate rebate mode: %w", err)
	}
	if strings.TrimSpace(raw) == MokAffiliateRebateModeUsage {
		return MokAffiliateRebateModeUsage, nil
	}
	return MokAffiliateRebateModeRecharge, nil
}

// MokGetAffiliateUsageWindow reads the usage window; unreadable times count as unset.
func (s *SettingService) MokGetAffiliateUsageWindow(ctx context.Context) (MokAffiliateUsageWindow, error) {
	var window MokAffiliateUsageWindow
	if s == nil || s.settingRepo == nil {
		return window, nil
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyMokAffiliateUsageSince, SettingKeyMokAffiliateUsageUntil})
	if err != nil {
		return window, fmt.Errorf("read affiliate usage window: %w", err)
	}
	window.Since = parseMokTime(values[SettingKeyMokAffiliateUsageSince])
	window.Until = parseMokTime(values[SettingKeyMokAffiliateUsageUntil])
	return window, nil
}

func parseMokTime(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		logger.LegacyPrintf("service.affiliate", "[Affiliate][mok] ignoring unreadable usage window time %q: %v", raw, err)
		return nil
	}
	return &t
}

// MokSetAffiliateRebateMode switches the rebate mode at now. Switching to usage starts a new
// usage window; switching back to recharge closes it. Setting the current mode is a no-op.
func (s *SettingService) MokSetAffiliateRebateMode(ctx context.Context, mode string, now time.Time) error {
	if mode != MokAffiliateRebateModeRecharge && mode != MokAffiliateRebateModeUsage {
		return infraerrors.BadRequest("MOK_AFFILIATE_REBATE_MODE_INVALID", "mode must be recharge or usage")
	}
	if s == nil || s.settingRepo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "setting service unavailable")
	}
	current, err := s.MokGetAffiliateRebateMode(ctx)
	if err != nil {
		return err
	}
	if current == mode {
		return nil
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	if mode == MokAffiliateRebateModeUsage {
		// Mode and start change together; a leftover end time is ignored in usage mode.
		if err := s.settingRepo.SetMultiple(ctx, map[string]string{
			SettingKeyMokAffiliateRebateMode: mode,
			SettingKeyMokAffiliateUsageSince: stamp,
		}); err != nil {
			return fmt.Errorf("switch affiliate rebate mode: %w", err)
		}
		if err := s.settingRepo.Delete(ctx, SettingKeyMokAffiliateUsageUntil); err != nil {
			return fmt.Errorf("clear affiliate usage window end: %w", err)
		}
		return nil
	}
	if err := s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyMokAffiliateRebateMode: mode,
		SettingKeyMokAffiliateUsageUntil: stamp,
	}); err != nil {
		return fmt.Errorf("switch affiliate rebate mode: %w", err)
	}
	return nil
}

// mokUsageWindowBounds returns the usage window to settle; a zero end means it is open.
// In recharge mode only a closed window (since < until) is left to settle.
func mokUsageWindowBounds(mode string, window MokAffiliateUsageWindow) (start, end time.Time, ok bool) {
	if window.Since == nil {
		return time.Time{}, time.Time{}, false
	}
	if mode == MokAffiliateRebateModeUsage {
		return *window.Since, time.Time{}, true
	}
	if window.Until == nil || !window.Until.After(*window.Since) {
		return time.Time{}, time.Time{}, false
	}
	return *window.Since, *window.Until, true
}

func mokStartOfDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// MokAffiliateRebateModeStatus is what the admin settings card shows.
type MokAffiliateRebateModeStatus struct {
	Mode       string                       `json:"mode"`
	UsageSince *time.Time                   `json:"usage_since,omitempty"`
	UsageUntil *time.Time                   `json:"usage_until,omitempty"`
	RecentDays []MokAffiliateUsageRebateDay `json:"recent_days"`
}

// MokAffiliateUsageRebateService settles usage rebates once a day has ended. It runs on
// every instance; a leader lock lets one of them work at a time.
type MokAffiliateUsageRebateService struct {
	affiliates *AffiliateService
	locks      LeaderLockCache
	db         *sql.DB
	now        func() time.Time
	location   func() *time.Location
	owner      string

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	start  sync.Once
	stop   sync.Once
}

func NewMokAffiliateUsageRebateService(affiliates *AffiliateService, locks LeaderLockCache, db *sql.DB) *MokAffiliateUsageRebateService {
	ctx, cancel := context.WithCancel(context.Background())
	return &MokAffiliateUsageRebateService{
		affiliates: affiliates,
		locks:      locks,
		db:         db,
		now:        time.Now,
		location:   timezone.Location,
		owner:      uuid.NewString(),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start runs the settlement loop in the background until Stop.
func (s *MokAffiliateUsageRebateService) Start() {
	if s == nil {
		return
	}
	s.start.Do(func() {
		s.wg.Add(1)
		go s.loop()
	})
}

// Stop ends the loop and waits for a running settlement to finish or give up.
func (s *MokAffiliateUsageRebateService) Stop() {
	if s == nil {
		return
	}
	s.stop.Do(func() {
		s.cancel()
		s.wg.Wait()
	})
}

func (s *MokAffiliateUsageRebateService) loop() {
	defer s.wg.Done()
	timer := time.NewTimer(mokUsageRebateFirstTick)
	defer timer.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-timer.C:
		}
		s.runOnce()
		timer.Reset(mokUsageRebateTick)
	}
}

func (s *MokAffiliateUsageRebateService) runOnce() {
	ctx, cancel := context.WithTimeout(s.ctx, mokUsageRebateRunTimeout)
	defer cancel()
	release, ok := tryAcquireSingletonLeaderLock(ctx, s.locks, s.db, mokUsageRebateLockKey, s.owner, mokUsageRebateLockTTL)
	if !ok {
		return
	}
	defer release()
	closed, err := s.SettleDueAt(ctx, s.now())
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Warn("mok_affiliate_usage_rebate_failed", "closed_days", closed, "error", err)
	}
}

func (s *MokAffiliateUsageRebateService) store() (MokAffiliateUsageRebateStore, error) {
	if s == nil || s.affiliates == nil || s.affiliates.settingService == nil {
		return nil, errors.New("affiliate usage rebate service is not configured")
	}
	store, ok := s.affiliates.repo.(MokAffiliateUsageRebateStore)
	if !ok {
		return nil, errors.New("affiliate repository cannot settle usage rebates")
	}
	return store, nil
}

// SettleDueAt closes every day of the usage window that ended at least the settle delay
// before now and is not closed yet, oldest first, and returns how many it closed. It stops
// at the first failure; settled invitees are not settled again when the day is retried.
func (s *MokAffiliateUsageRebateService) SettleDueAt(ctx context.Context, now time.Time) (int, error) {
	store, err := s.store()
	if err != nil {
		return 0, err
	}
	settings := s.affiliates.settingService
	mode, err := settings.MokGetAffiliateRebateMode(ctx)
	if err != nil {
		return 0, err
	}
	window, err := settings.MokGetAffiliateUsageWindow(ctx)
	if err != nil {
		return 0, err
	}
	start, end, ok := mokUsageWindowBounds(mode, window)
	if !ok {
		return 0, nil
	}
	loc := s.location()
	firstDay := mokStartOfDay(start, loc)
	closedDays, err := store.MokUsageRebateClosedDays(ctx, firstDay.Format(mokUsageRebateDayLayout))
	if err != nil {
		return 0, err
	}
	closed := 0
	for day := firstDay; closed < mokUsageRebateMaxDaysPerRun; day = day.AddDate(0, 0, 1) {
		dayEnd := day.AddDate(0, 0, 1)
		if now.Before(dayEnd.Add(mokUsageRebateSettleDelay)) || (!end.IsZero() && !day.Before(end)) {
			break
		}
		key := day.Format(mokUsageRebateDayLayout)
		if closedDays[key] {
			continue
		}
		from, to := day, dayEnd
		if start.After(from) {
			from = start
		}
		if !end.IsZero() && end.Before(to) {
			to = end
		}
		if err := s.settleDay(ctx, store, key, from, to); err != nil {
			return closed, fmt.Errorf("settle %s: %w", key, err)
		}
		closed++
	}
	return closed, nil
}

func (s *MokAffiliateUsageRebateService) settleDay(ctx context.Context, store MokAffiliateUsageRebateStore, day string, from, to time.Time) error {
	affiliates := s.affiliates
	settings := affiliates.settingService
	if !affiliates.IsEnabled(ctx) {
		return store.MokCloseUsageRebateDay(ctx, day, from, to, MokUsageRebateSkippedAffiliateDisabled)
	}
	usages, err := store.MokBalanceUsageByInvitee(ctx, from, to, settings.GetAffiliateRebateDurationDays(ctx))
	if err != nil {
		return err
	}
	freezeHours := settings.GetAffiliateRebateFreezeHours(ctx)
	perInviteeCap := settings.GetAffiliateRebatePerInviteeCap(ctx)
	rates := make(map[int64]float64)
	for _, usage := range usages {
		rate, ok := rates[usage.InviterID]
		if !ok {
			inviter, err := affiliates.repo.EnsureUserAffiliate(ctx, usage.InviterID)
			if err != nil {
				return err
			}
			if rate, err = affiliates.mokTierRatePercent(ctx, inviter, affiliates.resolveRebateRatePercent(ctx, inviter), 0, nil); err != nil {
				return err
			}
			rates[usage.InviterID] = rate
		}
		rebate := roundTo(usage.Amount*rate/100, 8)
		if perInviteeCap > 0 && rebate > 0 {
			existing, err := affiliates.repo.GetAccruedRebateFromInvitee(ctx, usage.InviterID, usage.InviteeID)
			if err != nil {
				return err
			}
			rebate = math.Max(0, math.Min(rebate, roundTo(perInviteeCap-existing, 8)))
		}
		if _, err := store.MokSettleUsageRebate(ctx, MokAffiliateUsageRebateRow{
			Day:          day,
			InviteeID:    usage.InviteeID,
			InviterID:    usage.InviterID,
			UsageAmount:  usage.Amount,
			RatePercent:  rate,
			RebateAmount: rebate,
		}, freezeHours); err != nil {
			return err
		}
	}
	return store.MokCloseUsageRebateDay(ctx, day, from, to, "")
}

// ModeStatus reports the rebate mode, the usage window and the latest closed days.
func (s *MokAffiliateUsageRebateService) ModeStatus(ctx context.Context) (*MokAffiliateRebateModeStatus, error) {
	store, err := s.store()
	if err != nil {
		return nil, err
	}
	settings := s.affiliates.settingService
	mode, err := settings.MokGetAffiliateRebateMode(ctx)
	if err != nil {
		return nil, err
	}
	window, err := settings.MokGetAffiliateUsageWindow(ctx)
	if err != nil {
		return nil, err
	}
	days, err := store.MokRecentUsageRebateDays(ctx, mokUsageRebateRecentDays)
	if err != nil {
		return nil, err
	}
	if days == nil {
		days = []MokAffiliateUsageRebateDay{}
	}
	status := &MokAffiliateRebateModeStatus{Mode: mode, UsageSince: window.Since, RecentDays: days}
	if mode == MokAffiliateRebateModeRecharge {
		status.UsageUntil = window.Until
	}
	return status, nil
}

// SetMode switches the rebate mode now and returns the new status.
func (s *MokAffiliateUsageRebateService) SetMode(ctx context.Context, mode string) (*MokAffiliateRebateModeStatus, error) {
	if s == nil || s.affiliates == nil {
		return nil, errors.New("affiliate usage rebate service is not configured")
	}
	if err := s.affiliates.settingService.MokSetAffiliateRebateMode(ctx, mode, s.now()); err != nil {
		return nil, err
	}
	return s.ModeStatus(ctx)
}
