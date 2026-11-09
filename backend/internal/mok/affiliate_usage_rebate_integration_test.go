//go:build integration

package mok

import (
	"context"
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

func TestDailyUsageRebateWithRealRepositories(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, timezone.Init("Asia/Shanghai"))
	loc := timezone.Location()
	at := func(day, hour, minute int) time.Time { return time.Date(2026, 11, day, hour, minute, 0, 0, loc) }

	db := openMigratedDB(t)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	settingRepo := repository.NewSettingRepository(client)
	settings := service.NewSettingService(settingRepo, nil)
	affiliates := service.NewAffiliateService(repository.NewAffiliateRepository(client, db), settings, nil, nil)
	usageRebate := service.NewMokAffiliateUsageRebateService(affiliates, nil, db)
	require.NoError(t, settingRepo.Set(ctx, service.SettingKeyAffiliateEnabled, "true"))
	require.NoError(t, settingRepo.Set(ctx, service.SettingKeyAffiliateRebateRate, "10"))
	_, err := settings.MokSetAffiliateRebateTiers(ctx, []service.MokAffiliateRebateTier{{MinAmount: 50000, RatePercent: 12}})
	require.NoError(t, err)
	require.NoError(t, settings.MokSetAffiliateRebateMode(ctx, service.MokAffiliateRebateModeUsage, at(1, 10, 0)))

	f := &affiliateFixture{t: t, db: db}
	inviter := f.user(0)
	invitee := f.user(inviter)
	stranger := f.user(0)
	f.order(inviter, "balance", "COMPLETED", 60000, 0) // team recharge volume 60000: 12%
	account := f.account()
	inviteeKey, strangerKey := f.apiKey(invitee), f.apiKey(stranger)

	f.usage(invitee, inviteeKey, account, 0, 50, at(1, 9, 0))     // before the switch to usage mode
	f.usage(invitee, inviteeKey, account, 0, 100, at(1, 15, 0))   // 11-01
	f.usage(invitee, inviteeKey, account, 1, 999, at(1, 16, 0))   // subscription usage
	f.usage(invitee, inviteeKey, account, 0, 30.5, at(2, 23, 59)) // 11-02
	f.usage(invitee, inviteeKey, account, 0, 70, at(3, 0, 5))     // 11-03, not over yet
	f.usage(stranger, strangerKey, account, 0, 500, at(1, 15, 0)) // nobody invited this user

	// Recharges earn nothing in usage mode.
	order := f.order(invitee, "balance", "RECHARGING", 1000, 0)
	rebate, err := affiliates.AccrueInviteRebateForOrder(ctx, invitee, 1000, &order)
	require.NoError(t, err)
	require.Zero(t, rebate)

	closed, err := usageRebate.SettleDueAt(ctx, at(3, 1, 0))
	require.NoError(t, err)
	require.Equal(t, 2, closed)

	ledger := func() []float64 {
		rows, err := db.Query(`SELECT amount::double precision FROM user_affiliate_ledger WHERE user_id = $1 AND action = 'accrue' ORDER BY id`, inviter)
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()
		var amounts []float64
		for rows.Next() {
			var amount float64
			require.NoError(t, rows.Scan(&amount))
			amounts = append(amounts, amount)
		}
		return amounts
	}
	require.Equal(t, []float64{12, 3.66}, ledger(), "12% of 100 on 11-01 and of 30.5 on 11-02")

	status, err := usageRebate.ModeStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, service.MokAffiliateRebateModeUsage, status.Mode)
	require.Len(t, status.RecentDays, 2)
	require.Equal(t, "2026-11-02", status.RecentDays[0].Day)
	require.Equal(t, 1, status.RecentDays[0].InviteeCount)
	require.InDelta(t, 30.5, status.RecentDays[0].UsageTotal, 1e-9)
	require.InDelta(t, 3.66, status.RecentDays[0].RebateTotal, 1e-9)
	require.True(t, status.RecentDays[1].WindowStart.Equal(at(1, 10, 0)))

	detail, err := affiliates.GetAffiliateDetail(ctx, inviter)
	require.NoError(t, err)
	require.InDelta(t, 15.66, detail.AffQuota, 1e-9)

	// Settling again, even after losing a day's summary, never pays an invitee twice.
	closed, err = usageRebate.SettleDueAt(ctx, at(3, 1, 0))
	require.NoError(t, err)
	require.Zero(t, closed)
	_, err = db.Exec(`DELETE FROM mok_affiliate_usage_rebate_days WHERE day = '2026-11-02'`)
	require.NoError(t, err)
	closed, err = usageRebate.SettleDueAt(ctx, at(3, 1, 0))
	require.NoError(t, err)
	require.Equal(t, 1, closed)
	require.Equal(t, []float64{12, 3.66}, ledger())

	// Back in recharge mode, usage up to the switch is still settled and later usage is not.
	require.NoError(t, settings.MokSetAffiliateRebateMode(ctx, service.MokAffiliateRebateModeRecharge, at(3, 12, 0)))
	f.usage(invitee, inviteeKey, account, 0, 1000, at(3, 13, 0))
	closed, err = usageRebate.SettleDueAt(ctx, at(5, 0, 0))
	require.NoError(t, err)
	require.Equal(t, 1, closed)
	require.Equal(t, []float64{12, 3.66, 8.4}, ledger(), "only the 70 used before the switch back counts on 11-03")
}
