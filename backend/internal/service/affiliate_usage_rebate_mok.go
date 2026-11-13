package service

// [mok] Fork-only file (see FORK.md): hourly affiliate rebates on invitees' balance usage.
//
// In usage mode recharges earn no rebate (mokTieredRebateRatePercent returns 0). Instead,
// once a clock hour has ended, every invitee's balance usage of that hour
// (usage_logs.actual_cost billed to the balance) is rebated to their inviter at the
// inviter's rate: the exclusive rate, else the tier reached by the team's recharge volume,
// else the global rate. Only usage inside the usage window counts: from the switch to usage
// mode until the switch back. The per-invitee cap, rebate duration and freeze period apply
// as for recharges, and hours that end while the affiliate feature is off are closed
// without rebates. Inviters outside the scope (affiliate_scope_mok.go) are left out.

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

	// MokUsageRebateSkippedAffiliateDisabled marks a period closed while the affiliate feature was off.
	MokUsageRebateSkippedAffiliateDisabled = "affiliate_disabled"

	mokUsageRebateDayLayout = "2006-01-02"
	// A period is settled this long after it ends, so usage logs written asynchronously are in.
	mokUsageRebateSettleDelay = 5 * time.Minute
	mokUsageRebateFirstTick   = time.Minute
	mokUsageRebateTick        = time.Minute
	mokUsageRebateRunTimeout  = 25 * time.Minute
	mokUsageRebateLockKey     = "mok:affiliate_usage_rebate"
	mokUsageRebateLockTTL     = 30 * time.Minute
	// A backlog longer than this many hours is worked off over several runs.
	mokUsageRebateMaxPeriodsPerRun = 31 * 24
	mokUsageRebateRecentDays       = 14
	mokUsageRebateDefaultPageSize  = 20
	mokUsageRebateMaxPageSize      = 100
	mokUsageRebateMaxSearchLength  = 100
)

// MokInviteeUsage is one invitee's balance usage over a settlement window.
type MokInviteeUsage struct {
	InviteeID int64
	InviterID int64
	Amount    float64
}

// MokAffiliateUsageRebateRow is one invitee's settled period.
type MokAffiliateUsageRebateRow struct {
	PeriodStart time.Time
	PeriodEnd   time.Time
	// Day is the date of PeriodStart in the server time zone, as YYYY-MM-DD.
	Day          string
	InviteeID    int64
	InviterID    int64
	UsageAmount  float64
	RatePercent  float64
	RebateAmount float64
}

// MokAffiliateUsageRebateDay sums the periods settled for one day of the server time zone.
type MokAffiliateUsageRebateDay struct {
	Day         string `json:"day"`
	PeriodCount int    `json:"period_count"`
	// SkippedCount is how many of the periods were closed without rebates.
	SkippedCount int     `json:"skipped_count"`
	InviteeCount int     `json:"invitee_count"`
	UsageTotal   float64 `json:"usage_total"`
	RebateTotal  float64 `json:"rebate_total"`
}

// MokAffiliateUsageRebateRecord is one invitee's settled period as the detail lists show it.
// The inviter fields and InviteeID are left out of a user's own list.
type MokAffiliateUsageRebateRecord struct {
	PeriodStart  time.Time `json:"period_start"`
	PeriodEnd    time.Time `json:"period_end"`
	InviterID    int64     `json:"inviter_id,omitempty"`
	InviterEmail string    `json:"inviter_email,omitempty"`
	InviteeID    int64     `json:"invitee_id,omitempty"`
	InviteeEmail string    `json:"invitee_email"`
	UsageAmount  float64   `json:"usage_amount"`
	RatePercent  float64   `json:"rate_percent"`
	RebateAmount float64   `json:"rebate_amount"`
	SettledAt    time.Time `json:"settled_at"`
}

// MokAffiliateUsageRebateFilter selects detail records, newest period first.
type MokAffiliateUsageRebateFilter struct {
	// InviterID > 0 keeps only that inviter's records.
	InviterID int64
	// Search keeps records whose inviter or invitee email or username contains it.
	Search   string
	Page     int
	PageSize int
}

// MokAffiliateUsageRebateStore is implemented by the affiliate repository in
// internal/repository/affiliate_usage_rebate_repo_mok.go.
type MokAffiliateUsageRebateStore interface {
	// MokBalanceUsageByInvitee sums balance-billed usage with a positive actual cost in
	// [from, to) for every user who has an inviter. durationDays > 0 drops usage later than
	// that many days after the invitee's affiliate profile was created.
	MokBalanceUsageByInvitee(ctx context.Context, from, to time.Time, durationDays int) ([]MokInviteeUsage, error)
	// MokSettleUsageRebate records the row and books its rebate in one transaction, and
	// returns false when that invitee's period was already settled.
	MokSettleUsageRebate(ctx context.Context, row MokAffiliateUsageRebateRow, freezeHours int) (bool, error)
	// MokCloseUsageRebatePeriod records the period with totals over its settled rows.
	MokCloseUsageRebatePeriod(ctx context.Context, day string, periodStart, periodEnd time.Time, skippedReason string) error
	// MokUsageRebateSettledUntil returns the end of the latest closed period, nil without one.
	MokUsageRebateSettledUntil(ctx context.Context) (*time.Time, error)
	// MokRecentUsageRebateDays returns the latest days with closed periods, newest first.
	MokRecentUsageRebateDays(ctx context.Context, limit int) ([]MokAffiliateUsageRebateDay, error)
	// MokListUsageRebates returns one page of records, newest period first, and their total.
	MokListUsageRebates(ctx context.Context, filter MokAffiliateUsageRebateFilter) ([]MokAffiliateUsageRebateRecord, int64, error)
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
	// Period bounds are stored as timestamptz, which keeps microseconds.
	t = t.Truncate(time.Microsecond)
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
	stamp := now.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)
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

// mokNextHour returns the first full hour of loc's clock after t.
func mokNextHour(t time.Time, loc *time.Location) time.Time {
	local := t.In(loc)
	intoHour := time.Duration(local.Minute())*time.Minute +
		time.Duration(local.Second())*time.Second +
		time.Duration(local.Nanosecond())
	return local.Add(time.Hour - intoHour)
}

// MokAffiliateRebateModeStatus is what the admin settings card shows.
type MokAffiliateRebateModeStatus struct {
	Mode       string     `json:"mode"`
	UsageSince *time.Time `json:"usage_since,omitempty"`
	UsageUntil *time.Time `json:"usage_until,omitempty"`
	// SettledUntil is how far the current usage window has been settled.
	SettledUntil *time.Time                   `json:"settled_until,omitempty"`
	RecentDays   []MokAffiliateUsageRebateDay `json:"recent_days"`
}

// MokAffiliateUsageRebateService settles usage rebates once an hour has ended. It runs on
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
		slog.Warn("mok_affiliate_usage_rebate_failed", "closed_periods", closed, "error", err)
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

// SettleDueAt closes every period of the usage window that ended at least the settle delay
// before now and is not closed yet, oldest first, and returns how many it closed. Periods
// run from where the last one ended (or the window starts) to the next full hour of the
// server clock. It stops at the first failure; settled invitees are not settled again when
// the period is retried.
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
	from, end, ok := mokUsageWindowBounds(mode, window)
	if !ok {
		return 0, nil
	}
	settledUntil, err := store.MokUsageRebateSettledUntil(ctx)
	if err != nil {
		return 0, err
	}
	if settledUntil != nil && settledUntil.After(from) {
		from = *settledUntil
	}
	loc := s.location()
	// Rates and the scope are those at settlement time, so one lookup per inviter serves
	// the whole run.
	run := &mokUsageRebateRun{rates: make(map[int64]float64), inScope: make(map[int64]bool)}
	closed := 0
	for closed < mokUsageRebateMaxPeriodsPerRun {
		if !end.IsZero() && !from.Before(end) {
			break
		}
		to := mokNextHour(from, loc)
		if !end.IsZero() && end.Before(to) {
			to = end
		}
		if now.Before(to.Add(mokUsageRebateSettleDelay)) {
			break
		}
		if err := s.settlePeriod(ctx, store, from, to, loc, run); err != nil {
			return closed, fmt.Errorf("settle %s: %w", from.In(loc).Format(time.RFC3339), err)
		}
		closed++
		from = to
	}
	return closed, nil
}

// mokUsageRebateRun caches what one settlement run looked up per inviter.
type mokUsageRebateRun struct {
	rates   map[int64]float64
	inScope map[int64]bool
}

func (s *MokAffiliateUsageRebateService) settlePeriod(ctx context.Context, store MokAffiliateUsageRebateStore, from, to time.Time, loc *time.Location, run *mokUsageRebateRun) error {
	affiliates := s.affiliates
	settings := affiliates.settingService
	day := from.In(loc).Format(mokUsageRebateDayLayout)
	if !affiliates.IsEnabled(ctx) {
		return store.MokCloseUsageRebatePeriod(ctx, day, from, to, MokUsageRebateSkippedAffiliateDisabled)
	}
	usages, err := store.MokBalanceUsageByInvitee(ctx, from, to, settings.GetAffiliateRebateDurationDays(ctx))
	if err != nil {
		return err
	}
	if len(usages) > 0 {
		freezeHours := settings.GetAffiliateRebateFreezeHours(ctx)
		perInviteeCap := settings.GetAffiliateRebatePerInviteeCap(ctx)
		for _, usage := range usages {
			inScope, ok := run.inScope[usage.InviterID]
			if !ok {
				if inScope, err = affiliates.mokInScope(ctx, usage.InviterID); err != nil {
					return err
				}
				run.inScope[usage.InviterID] = inScope
			}
			if !inScope {
				continue
			}
			rate, ok := run.rates[usage.InviterID]
			if !ok {
				inviter, err := affiliates.repo.EnsureUserAffiliate(ctx, usage.InviterID)
				if err != nil {
					return err
				}
				if rate, err = affiliates.mokTierRatePercent(ctx, inviter, affiliates.resolveRebateRatePercent(ctx, inviter), 0, nil); err != nil {
					return err
				}
				run.rates[usage.InviterID] = rate
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
				PeriodStart:  from,
				PeriodEnd:    to,
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
	}
	return store.MokCloseUsageRebatePeriod(ctx, day, from, to, "")
}

// ModeStatus reports the rebate mode, the usage window, how far it is settled and the
// latest settled days.
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
	settledUntil, err := store.MokUsageRebateSettledUntil(ctx)
	if err != nil {
		return nil, err
	}
	status := &MokAffiliateRebateModeStatus{Mode: mode, UsageSince: window.Since, RecentDays: days}
	if mode == MokAffiliateRebateModeRecharge {
		status.UsageUntil = window.Until
	}
	// Periods settled before the current usage window started say nothing about it.
	if settledUntil != nil && window.Since != nil && settledUntil.After(*window.Since) {
		status.SettledUntil = settledUntil
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

// ListRecords returns one page of settled records, newest period first, with their total.
func (s *MokAffiliateUsageRebateService) ListRecords(ctx context.Context, filter MokAffiliateUsageRebateFilter) ([]MokAffiliateUsageRebateRecord, int64, error) {
	store, err := s.store()
	if err != nil {
		return nil, 0, err
	}
	records, total, err := store.MokListUsageRebates(ctx, NormalizeMokAffiliateUsageRebateFilter(filter))
	if err != nil {
		return nil, 0, err
	}
	if records == nil {
		records = []MokAffiliateUsageRebateRecord{}
	}
	return records, total, nil
}

// ListRecordsForInviter returns the user's own records. As on the affiliate page, invitees
// are shown by masked email only.
func (s *MokAffiliateUsageRebateService) ListRecordsForInviter(ctx context.Context, inviterID int64, page, pageSize int) ([]MokAffiliateUsageRebateRecord, int64, error) {
	if inviterID <= 0 {
		return nil, 0, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	records, total, err := s.ListRecords(ctx, MokAffiliateUsageRebateFilter{InviterID: inviterID, Page: page, PageSize: pageSize})
	if err != nil {
		return nil, 0, err
	}
	for i := range records {
		records[i].InviterID = 0
		records[i].InviterEmail = ""
		records[i].InviteeID = 0
		records[i].InviteeEmail = maskEmail(records[i].InviteeEmail)
	}
	return records, total, nil
}

// NormalizeMokAffiliateUsageRebateFilter bounds the page, the page size and the search text.
func NormalizeMokAffiliateUsageRebateFilter(filter MokAffiliateUsageRebateFilter) MokAffiliateUsageRebateFilter {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = mokUsageRebateDefaultPageSize
	}
	if filter.PageSize > mokUsageRebateMaxPageSize {
		filter.PageSize = mokUsageRebateMaxPageSize
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if search := []rune(filter.Search); len(search) > mokUsageRebateMaxSearchLength {
		filter.Search = string(search[:mokUsageRebateMaxSearchLength])
	}
	return filter
}
