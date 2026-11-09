//go:build integration

package mok

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/stretchr/testify/require"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime" // registers ent field validators and defaults
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type affiliateFixture struct {
	t   *testing.T
	db  *sql.DB
	seq int
}

func (f *affiliateFixture) user(inviterID int64) int64 {
	f.t.Helper()
	f.seq++
	var id int64
	err := f.db.QueryRow(`INSERT INTO users (email, password_hash) VALUES ($1, 'x') RETURNING id`,
		fmt.Sprintf("user%d@example.com", f.seq)).Scan(&id)
	require.NoError(f.t, err)
	var inviter any
	if inviterID > 0 {
		inviter = inviterID
	}
	_, err = f.db.Exec(`INSERT INTO user_affiliates (user_id, aff_code, inviter_id) VALUES ($1, $2, $3)`,
		id, fmt.Sprintf("CODE%04d", f.seq), inviter)
	require.NoError(f.t, err)
	return id
}

// order inserts a payment order; balance orders also get their fulfilment redeem code, used
// like payment fulfilment does before the order completes.
func (f *affiliateFixture) order(userID int64, orderType, status string, amount, bonus float64) int64 {
	f.t.Helper()
	f.seq++
	code := fmt.Sprintf("PAY-%d-%d", f.seq, f.seq)
	var id int64
	err := f.db.QueryRow(`
		INSERT INTO payment_orders (user_id, amount, pay_amount, bonus_amount, order_type, status, recharge_code, out_trade_no, expires_at)
		VALUES ($1, $2, $2, $3, $4, $5, $6, $7, NOW() + INTERVAL '1 hour') RETURNING id`,
		userID, amount, bonus, orderType, status, code, fmt.Sprintf("trade-%d", f.seq)).Scan(&id)
	require.NoError(f.t, err)
	if orderType == "balance" {
		f.redeem(userID, code, "balance", "used", amount)
	}
	return id
}

func (f *affiliateFixture) redeem(userID int64, code, codeType, status string, value float64) {
	f.t.Helper()
	var usedBy any
	if status == "used" {
		usedBy = userID
	}
	_, err := f.db.Exec(`INSERT INTO redeem_codes (code, type, value, status, used_by) VALUES ($1, $2, $3, $4, $5)`,
		code, codeType, value, status, usedBy)
	require.NoError(f.t, err)
}

func TestMokTeamRechargeVolume(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	reader, ok := repository.NewAffiliateRepository(client, db).(service.MokAffiliateTeamVolumeReader)
	require.True(t, ok, "the affiliate repository must report team volume")
	f := &affiliateFixture{t: t, db: db}

	inviter := f.user(0)
	invitee := f.user(inviter)
	other := f.user(inviter)
	grandchild := f.user(invitee) // invited by an invitee: not part of the inviter's team
	stranger := f.user(0)

	f.order(inviter, "balance", "COMPLETED", 1000, 100)         // 900: bonus is left out
	f.order(invitee, "subscription", "COMPLETED", 500, 0)       // 500
	f.order(invitee, "balance", "REFUND_REQUESTED", 300, 0)     // 300: refund not done yet
	f.order(invitee, "balance", "REFUNDED", 2000, 0)            // refunded
	f.order(invitee, "balance", "PARTIALLY_REFUNDED", 4000, 0)  // partly refunded
	pending := f.order(other, "balance", "RECHARGING", 700, 0)  // still being fulfilled
	completed := f.order(other, "balance", "COMPLETED", 800, 0) // 800
	f.order(grandchild, "balance", "COMPLETED", 5000, 0)
	f.order(stranger, "balance", "COMPLETED", 9000, 0)

	f.redeem(invitee, "CARD-1", "balance", "used", 250)       // 250
	f.redeem(inviter, "CARD-2", "balance", "unused", 999)     // not redeemed
	f.redeem(other, "CARD-3", "concurrency", "used", 5)       // not balance
	f.redeem(inviter, "ADMIN-1", "admin_balance", "used", 60) // admin top-up
	f.redeem(grandchild, "CARD-4", "balance", "used", 40)

	volume, err := reader.MokTeamRechargeVolume(ctx, inviter, nil)
	require.NoError(t, err)
	require.InDelta(t, 900+500+300+800+250, volume, 1e-9)

	volume, err = reader.MokTeamRechargeVolume(ctx, inviter, &completed)
	require.NoError(t, err)
	require.InDelta(t, 900+500+300+250, volume, 1e-9, "the excluded order is left out")

	volume, err = reader.MokTeamRechargeVolume(ctx, inviter, &pending)
	require.NoError(t, err)
	require.InDelta(t, 900+500+300+800+250, volume, 1e-9)

	volume, err = reader.MokTeamRechargeVolume(ctx, invitee, nil)
	require.NoError(t, err)
	require.InDelta(t, 500+300+250+5000+40, volume, 1e-9, "an invitee's own team includes the users they invited")

	volume, err = reader.MokTeamRechargeVolume(ctx, stranger, nil)
	require.NoError(t, err)
	require.InDelta(t, 9000, volume, 1e-9)
}

func TestTieredRebateWithRealRepositories(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	settingRepo := repository.NewSettingRepository(client)
	settings := service.NewSettingService(settingRepo, nil)
	affiliates := service.NewAffiliateService(repository.NewAffiliateRepository(client, db), settings, nil, nil)
	require.NoError(t, settingRepo.Set(ctx, service.SettingKeyAffiliateEnabled, "true"))
	require.NoError(t, settingRepo.Set(ctx, service.SettingKeyAffiliateRebateRate, "10"))
	_, err := settings.MokSetAffiliateRebateTiers(ctx, []service.MokAffiliateRebateTier{
		{MinAmount: 50000, RatePercent: 12},
		{MinAmount: 100000, RatePercent: 15},
	})
	require.NoError(t, err)

	f := &affiliateFixture{t: t, db: db}
	inviter := f.user(0)
	invitee := f.user(inviter)
	f.order(inviter, "balance", "COMPLETED", 30000, 0)
	f.order(invitee, "balance", "COMPLETED", 15000, 0)

	// 45000 so far; this 10000 order crosses 50000, so it is rebated whole at 12%.
	order := f.order(invitee, "balance", "RECHARGING", 10000, 0)
	rebate, err := affiliates.AccrueInviteRebateForOrder(ctx, invitee, 10000, &order)
	require.NoError(t, err)
	require.InDelta(t, 1200, rebate, 1e-9)
	_, err = db.Exec(`UPDATE payment_orders SET status = 'COMPLETED' WHERE id = $1`, order)
	require.NoError(t, err)

	// A redeem code is already used when its rebate runs: 55000 + 2000 = 57000, still 12%.
	f.redeem(invitee, "CARD-10", "balance", "used", 2000)
	rebate, err = affiliates.AccrueInviteRebate(ctx, invitee, 2000)
	require.NoError(t, err)
	require.InDelta(t, 240, rebate, 1e-9)

	detail, err := affiliates.GetAffiliateDetail(ctx, inviter)
	require.NoError(t, err)
	require.Equal(t, 12.0, detail.EffectiveRebateRatePercent)
	require.InDelta(t, 1440, detail.AffQuota, 1e-9)

	status, err := affiliates.MokAffiliateTierStatus(ctx, inviter)
	require.NoError(t, err)
	require.Equal(t, 57000.0, status.TeamVolume)
	require.Equal(t, 43000.0, status.NextTierRemaining)

	// An exclusive rate wins over the tiers.
	exclusive := 20.0
	require.NoError(t, affiliates.AdminSetUserRebateRate(ctx, inviter, &exclusive))
	order = f.order(invitee, "balance", "RECHARGING", 1000, 0)
	rebate, err = affiliates.AccrueInviteRebateForOrder(ctx, invitee, 1000, &order)
	require.NoError(t, err)
	require.InDelta(t, 200, rebate, 1e-9)
}
