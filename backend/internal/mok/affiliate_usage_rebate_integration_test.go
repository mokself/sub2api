//go:build integration

package mok

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/stretchr/testify/require"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime" // registers ent field validators and defaults
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
)

func (f *affiliateFixture) apiKey(userID int64) int64 {
	f.t.Helper()
	f.seq++
	var id int64
	err := f.db.QueryRow(`INSERT INTO api_keys (user_id, key, name) VALUES ($1, $2, 'test') RETURNING id`,
		userID, fmt.Sprintf("sk-test-%d", f.seq)).Scan(&id)
	require.NoError(f.t, err)
	return id
}

func (f *affiliateFixture) account() int64 {
	f.t.Helper()
	var id int64
	err := f.db.QueryRow(`INSERT INTO accounts (name, platform, type) VALUES ('test', 'anthropic', 'apikey') RETURNING id`).Scan(&id)
	require.NoError(f.t, err)
	return id
}

func (f *affiliateFixture) usage(userID, apiKeyID, accountID int64, billingType int, cost float64, at time.Time) {
	f.t.Helper()
	f.seq++
	_, err := f.db.Exec(`
		INSERT INTO usage_logs (user_id, api_key_id, account_id, request_id, model, billing_type, total_cost, actual_cost, created_at)
		VALUES ($1, $2, $3, $4, 'claude-test', $5, $6, $6, $7)`,
		userID, apiKeyID, accountID, fmt.Sprintf("req-%d", f.seq), billingType, cost, at)
	require.NoError(f.t, err)
}

func (f *affiliateFixture) email(userID int64) string {
	f.t.Helper()
	var email string
	require.NoError(f.t, f.db.QueryRow(`SELECT email FROM users WHERE id = $1`, userID).Scan(&email))
	return email
}

// accrued returns the inviter's rebate ledger entries, oldest first.
func (f *affiliateFixture) accrued(inviterID int64) []float64 {
	f.t.Helper()
	rows, err := f.db.Query(`SELECT amount::double precision FROM user_affiliate_ledger WHERE user_id = $1 AND action = 'accrue' ORDER BY id`, inviterID)
	require.NoError(f.t, err)
	defer func() { _ = rows.Close() }()
	var amounts []float64
	for rows.Next() {
		var amount float64
		require.NoError(f.t, rows.Scan(&amount))
		amounts = append(amounts, amount)
	}
	require.NoError(f.t, rows.Err())
	return amounts
}

type usageRebateEnv struct {
	db          *sql.DB
	settings    *service.SettingService
	affiliates  *service.AffiliateService
	usageRebate *service.MokAffiliateUsageRebateService
	loc         *time.Location
}

// newUsageRebateEnv has the affiliate feature on with a 10% global rate and a 12% tier from
// a team recharge volume of 50000. Hours follow the Asia/Shanghai clock.
func newUsageRebateEnv(t *testing.T) *usageRebateEnv {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, timezone.Init("Asia/Shanghai"))
	db := openMigratedDB(t)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	settingRepo := repository.NewSettingRepository(client)
	settings := service.NewSettingService(settingRepo, nil)
	affiliates := service.NewAffiliateService(repository.NewAffiliateRepository(client, db), settings, nil, nil)
	require.NoError(t, settingRepo.Set(ctx, service.SettingKeyAffiliateEnabled, "true"))
	require.NoError(t, settingRepo.Set(ctx, service.SettingKeyAffiliateRebateRate, "10"))
	_, err := settings.MokSetAffiliateRebateTiers(ctx, []service.MokAffiliateRebateTier{{MinAmount: 50000, RatePercent: 12}})
	require.NoError(t, err)
	return &usageRebateEnv{
		db:          db,
		settings:    settings,
		affiliates:  affiliates,
		usageRebate: service.NewMokAffiliateUsageRebateService(affiliates, nil, db),
		loc:         timezone.Location(),
	}
}

func (e *usageRebateEnv) at(day, hour, minute int) time.Time {
	return time.Date(2026, 11, day, hour, minute, 0, 0, e.loc)
}

func TestHourlyUsageRebateWithRealRepositories(t *testing.T) {
	ctx := context.Background()
	env := newUsageRebateEnv(t)
	at, db, usageRebate := env.at, env.db, env.usageRebate
	require.NoError(t, env.settings.MokSetAffiliateRebateMode(ctx, service.MokAffiliateRebateModeUsage, at(1, 10, 20)))

	f := &affiliateFixture{t: t, db: db}
	inviter := f.user(0)
	invitee := f.user(inviter)
	stranger := f.user(0)
	second := f.user(inviter)
	f.order(inviter, "balance", "COMPLETED", 60000, 0) // team recharge volume 60000: 12%
	account := f.account()
	inviteeKey, strangerKey, secondKey := f.apiKey(invitee), f.apiKey(stranger), f.apiKey(second)

	f.usage(invitee, inviteeKey, account, 0, 50, at(1, 9, 0))       // before the switch to usage mode
	f.usage(invitee, inviteeKey, account, 0, 100, at(1, 10, 30))    // 10:20 - 11:00
	f.usage(invitee, inviteeKey, account, 1, 999, at(1, 10, 40))    // subscription usage
	f.usage(stranger, strangerKey, account, 0, 500, at(1, 10, 30))  // nobody invited this user
	f.usage(invitee, inviteeKey, account, 0, 30.5, at(1, 11, 59))   // 11:00 - 12:00
	f.usage(second, secondKey, account, 0, 20, at(1, 11, 10))       // 11:00 - 12:00
	f.usage(invitee, inviteeKey, account, 0, 70, at(1, 12, 5))      // 12:00 - 13:00, not over yet
	f.usage(invitee, inviteeKey, account, 0, 0.0004, at(1, 12, 10)) // small amounts keep 8 decimals

	// Recharges earn nothing in usage mode.
	order := f.order(invitee, "balance", "RECHARGING", 1000, 0)
	rebate, err := env.affiliates.AccrueInviteRebateForOrder(ctx, invitee, 1000, &order)
	require.NoError(t, err)
	require.Zero(t, rebate)

	closed, err := usageRebate.SettleDueAt(ctx, at(1, 12, 6))
	require.NoError(t, err)
	require.Equal(t, 2, closed)
	require.Equal(t, []float64{12, 3.66, 2.4}, f.accrued(inviter), "12% of 100, then of 30.5 and 20 in the next hour")

	status, err := usageRebate.ModeStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, service.MokAffiliateRebateModeUsage, status.Mode)
	require.True(t, status.SettledUntil.Equal(at(1, 12, 0)))
	require.Equal(t, []service.MokAffiliateUsageRebateDay{{
		Day: "2026-11-01", PeriodCount: 2, InviteeCount: 2, UsageTotal: 150.5, RebateTotal: 18.06,
	}}, status.RecentDays)

	detail, err := env.affiliates.GetAffiliateDetail(ctx, inviter)
	require.NoError(t, err)
	require.InDelta(t, 18.06, detail.AffQuota, 1e-9)

	// The admin list has every record, newest period first, and searches emails literally.
	records, total, err := usageRebate.ListRecords(ctx, service.MokAffiliateUsageRebateFilter{Page: 1, PageSize: 2})
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, records, 2)
	require.Equal(t, second, records[0].InviteeID)
	require.Equal(t, f.email(second), records[0].InviteeEmail)
	require.Equal(t, f.email(inviter), records[0].InviterEmail)
	require.True(t, records[0].PeriodStart.Equal(at(1, 11, 0)) && records[0].PeriodEnd.Equal(at(1, 12, 0)))
	require.Equal(t, []float64{20, 12, 2.4}, []float64{records[0].UsageAmount, records[0].RatePercent, records[0].RebateAmount})
	require.Equal(t, invitee, records[1].InviteeID)

	records, total, err = usageRebate.ListRecords(ctx, service.MokAffiliateUsageRebateFilter{Page: 2, PageSize: 2})
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, records, 1)
	require.True(t, records[0].PeriodStart.Equal(at(1, 10, 20)), "the first period starts at the switch to usage mode")

	for search, want := range map[string]int64{
		f.email(second):   1,
		"USER":            3, // every email is userN@example.com
		f.email(stranger): 0,
		"%":               0,
		"user_@":          0,
	} {
		_, total, err = usageRebate.ListRecords(ctx, service.MokAffiliateUsageRebateFilter{Search: search})
		require.NoError(t, err)
		require.Equal(t, want, total, "search %q", search)
	}

	// An inviter sees only their own records, with masked invitee emails.
	own, total, err := usageRebate.ListRecordsForInviter(ctx, inviter, 1, 20)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.NotEqual(t, f.email(second), own[0].InviteeEmail)
	require.Contains(t, own[0].InviteeEmail, "***")
	require.Zero(t, own[0].InviteeID)
	require.Empty(t, own[0].InviterEmail)
	_, total, err = usageRebate.ListRecordsForInviter(ctx, invitee, 1, 20)
	require.NoError(t, err)
	require.Zero(t, total)

	// Settling again, even after losing a period's summary, never pays an invitee twice.
	closed, err = usageRebate.SettleDueAt(ctx, at(1, 12, 6))
	require.NoError(t, err)
	require.Zero(t, closed)
	_, err = db.Exec(`DELETE FROM mok_affiliate_usage_rebate_periods WHERE period_start = $1`, at(1, 11, 0))
	require.NoError(t, err)
	closed, err = usageRebate.SettleDueAt(ctx, at(1, 12, 6))
	require.NoError(t, err)
	require.Equal(t, 1, closed)
	require.Equal(t, []float64{12, 3.66, 2.4}, f.accrued(inviter))

	// Back in recharge mode, usage up to the switch is still settled and later usage is not.
	require.NoError(t, env.settings.MokSetAffiliateRebateMode(ctx, service.MokAffiliateRebateModeRecharge, at(1, 12, 30)))
	f.usage(invitee, inviteeKey, account, 0, 1000, at(1, 12, 40))
	closed, err = usageRebate.SettleDueAt(ctx, at(2, 0, 0))
	require.NoError(t, err)
	require.Equal(t, 1, closed)
	require.Equal(t, []float64{12, 3.66, 2.4, 8.400048}, f.accrued(inviter), "only the 70.0004 used before the switch back counts")
	closed, err = usageRebate.SettleDueAt(ctx, at(2, 0, 0))
	require.NoError(t, err)
	require.Zero(t, closed)

	status, err = usageRebate.ModeStatus(ctx)
	require.NoError(t, err)
	require.True(t, status.UsageUntil.Equal(at(1, 12, 30)))
	require.True(t, status.SettledUntil.Equal(at(1, 12, 30)))
	require.Equal(t, 3, status.RecentDays[0].PeriodCount)
}

func TestUsageRebateSkipsHoursWhileAffiliateIsOff(t *testing.T) {
	ctx := context.Background()
	env := newUsageRebateEnv(t)
	at := env.at
	require.NoError(t, env.settings.MokSetAffiliateRebateMode(ctx, service.MokAffiliateRebateModeUsage, at(1, 10, 0)))
	f := &affiliateFixture{t: t, db: env.db}
	inviter := f.user(0)
	invitee := f.user(inviter)
	account, key := f.account(), f.apiKey(invitee)
	f.usage(invitee, key, account, 0, 100, at(1, 10, 30))
	f.usage(invitee, key, account, 0, 100, at(1, 11, 30))

	closed, err := env.usageRebate.SettleDueAt(ctx, at(1, 11, 6))
	require.NoError(t, err)
	require.Equal(t, 1, closed)
	_, err = env.db.Exec(`UPDATE settings SET value = 'false' WHERE key = $1`, service.SettingKeyAffiliateEnabled)
	require.NoError(t, err)
	closed, err = env.usageRebate.SettleDueAt(ctx, at(1, 12, 6))
	require.NoError(t, err)
	require.Equal(t, 1, closed)

	require.Equal(t, []float64{10}, f.accrued(inviter), "the hour that ended while the feature was off earns nothing")
	status, err := env.usageRebate.ModeStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, []service.MokAffiliateUsageRebateDay{{
		Day: "2026-11-01", PeriodCount: 2, SkippedCount: 1, InviteeCount: 1, UsageTotal: 100, RebateTotal: 10,
	}}, status.RecentDays)
}

// mok_003 moves the daily records of mok_002 into the period tables. The test puts a
// database back into the state of mok_002 with data and runs mok_003 on it again.
func TestUsageRebateMigrationCarriesDailyRecordsOver(t *testing.T) {
	ctx := context.Background()
	env := newUsageRebateEnv(t)
	at, db := env.at, env.db
	migration := func(name string) {
		t.Helper()
		content, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		_, err = db.Exec(string(content))
		require.NoError(t, err, name)
	}

	f := &affiliateFixture{t: t, db: db}
	inviter := f.user(0)
	invitee := f.user(inviter)
	second := f.user(inviter)
	account := f.account()
	inviteeKey, secondKey := f.apiKey(invitee), f.apiKey(second)
	require.NoError(t, env.settings.MokSetAffiliateRebateMode(ctx, service.MokAffiliateRebateModeUsage, at(1, 10, 0)))

	// The daily settlement closed 11-01. Of 11-02 it settled the first invitee and then stopped.
	migration("mok_002_affiliate_usage_rebates.sql")
	_, err := db.Exec(`
		INSERT INTO mok_affiliate_usage_rebate_days (day, window_start, window_end, invitee_count, usage_total, rebate_total)
		VALUES ('2026-11-01', $1, $2, 1, 100, 10)`, at(1, 10, 0), at(2, 0, 0))
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO mok_affiliate_usage_rebates (day, invitee_id, inviter_id, usage_amount, rate_percent, rebate_amount)
		VALUES ('2026-11-01', $1, $2, 100, 10, 10), ('2026-11-02', $1, $2, 30, 10, 3)`, invitee, inviter)
	require.NoError(t, err)

	migration("mok_003_affiliate_usage_rebate_periods.sql")

	var daysTable sql.NullString
	require.NoError(t, db.QueryRow(`SELECT to_regclass('mok_affiliate_usage_rebate_days')::text`).Scan(&daysTable))
	require.False(t, daysTable.Valid, "the day table is gone")
	var leftDays string
	require.NoError(t, db.QueryRow(`SELECT string_agg(to_char(day, 'YYYY-MM-DD'), ',') FROM mok_affiliate_usage_rebates`).Scan(&leftDays))
	require.Equal(t, "2026-11-02", leftDays, "only the day that was not closed stays behind")

	records, total, err := env.usageRebate.ListRecords(ctx, service.MokAffiliateUsageRebateFilter{})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.True(t, records[0].PeriodStart.Equal(at(1, 10, 0)) && records[0].PeriodEnd.Equal(at(2, 0, 0)), "a carried-over day keeps its window")
	require.Equal(t, []float64{100, 10, 10}, []float64{records[0].UsageAmount, records[0].RatePercent, records[0].RebateAmount})
	status, err := env.usageRebate.ModeStatus(ctx)
	require.NoError(t, err)
	require.True(t, status.SettledUntil.Equal(at(2, 0, 0)))
	require.Equal(t, []service.MokAffiliateUsageRebateDay{{
		Day: "2026-11-01", PeriodCount: 1, InviteeCount: 1, UsageTotal: 100, RebateTotal: 10,
	}}, status.RecentDays)

	// The hourly settlement takes over at the end of the last closed day. The invitee the
	// daily settlement already paid for 11-02 is left out for that day only.
	f.usage(invitee, inviteeKey, account, 0, 40, at(2, 3, 10))
	f.usage(second, secondKey, account, 0, 10, at(2, 3, 20))
	f.usage(invitee, inviteeKey, account, 0, 50, at(3, 0, 10))
	closed, err := env.usageRebate.SettleDueAt(ctx, at(2, 4, 6))
	require.NoError(t, err)
	require.Equal(t, 4, closed, "the hours from 00:00 to 04:00")
	require.Equal(t, []float64{1}, f.accrued(inviter), "10% of the second invitee's 10")

	closed, err = env.usageRebate.SettleDueAt(ctx, at(3, 1, 6))
	require.NoError(t, err)
	require.Equal(t, 21, closed)
	require.Equal(t, []float64{1, 5}, f.accrued(inviter), "from 11-03 on the first invitee earns rebates again")
}
