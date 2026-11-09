package repository

// [mok] Fork-only file (see FORK.md): storage for daily affiliate usage rebates
// (service/affiliate_usage_rebate_mok.go, migrations/mok_002_affiliate_usage_rebates.sql).

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.MokAffiliateUsageRebateStore = (*affiliateRepository)(nil)

// The rebate duration counts from the invitee's affiliate profile creation, as upstream's
// recharge rebate does with AffiliateSummary.CreatedAt.
const mokBalanceUsageByInviteeSQL = `
SELECT ul.user_id, ua.inviter_id, SUM(ul.actual_cost)::double precision
FROM usage_logs ul
JOIN user_affiliates ua ON ua.user_id = ul.user_id
WHERE ua.inviter_id IS NOT NULL
  AND ul.billing_type = $4
  AND ul.actual_cost > 0
  AND ul.created_at >= $1
  AND ul.created_at < $2
  AND ($3::int = 0 OR ul.created_at < ua.created_at + make_interval(days => $3::int))
GROUP BY ul.user_id, ua.inviter_id
ORDER BY ul.user_id`

func (r *affiliateRepository) MokBalanceUsageByInvitee(ctx context.Context, from, to time.Time, durationDays int) ([]service.MokInviteeUsage, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, mokBalanceUsageByInviteeSQL, from, to, durationDays, service.BillingTypeBalance)
	if err != nil {
		return nil, fmt.Errorf("query invitee balance usage: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var usages []service.MokInviteeUsage
	for rows.Next() {
		var usage service.MokInviteeUsage
		if err := rows.Scan(&usage.InviteeID, &usage.InviterID, &usage.Amount); err != nil {
			return nil, err
		}
		usages = append(usages, usage)
	}
	return usages, rows.Err()
}

func (r *affiliateRepository) MokSettleUsageRebate(ctx context.Context, row service.MokAffiliateUsageRebateRow, freezeHours int) (bool, error) {
	var settled bool
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		res, err := txClient.ExecContext(txCtx, `
INSERT INTO mok_affiliate_usage_rebates (day, invitee_id, inviter_id, usage_amount, rate_percent, rebate_amount)
VALUES ($1::date, $2, $3, $4, $5, $6)
ON CONFLICT (day, invitee_id) DO NOTHING`,
			row.Day, row.InviteeID, row.InviterID, row.UsageAmount, row.RatePercent, row.RebateAmount)
		if err != nil {
			return fmt.Errorf("record usage rebate: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		settled = true
		if row.RebateAmount <= 0 {
			return nil
		}
		applied, err := r.AccrueQuota(txCtx, row.InviterID, row.InviteeID, row.RebateAmount, freezeHours, nil)
		if err != nil {
			return err
		}
		if !applied {
			// The inviter's affiliate profile is gone: keep the record, without a rebate.
			_, err = txClient.ExecContext(txCtx,
				`UPDATE mok_affiliate_usage_rebates SET rebate_amount = 0 WHERE day = $1::date AND invitee_id = $2`,
				row.Day, row.InviteeID)
			return err
		}
		return nil
	})
	return settled, err
}

func (r *affiliateRepository) MokCloseUsageRebateDay(ctx context.Context, day string, windowStart, windowEnd time.Time, skippedReason string) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.ExecContext(ctx, `
INSERT INTO mok_affiliate_usage_rebate_days (day, window_start, window_end, invitee_count, usage_total, rebate_total, skipped_reason)
SELECT $1::date, $2, $3, COUNT(*), COALESCE(SUM(usage_amount), 0), COALESCE(SUM(rebate_amount), 0), $4
FROM mok_affiliate_usage_rebates WHERE day = $1::date
ON CONFLICT (day) DO NOTHING`, day, windowStart, windowEnd, skippedReason)
	if err != nil {
		return fmt.Errorf("close usage rebate day: %w", err)
	}
	return nil
}

func (r *affiliateRepository) MokUsageRebateClosedDays(ctx context.Context, fromDay string) (map[string]bool, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx,
		`SELECT to_char(day, 'YYYY-MM-DD') FROM mok_affiliate_usage_rebate_days WHERE day >= $1::date`, fromDay)
	if err != nil {
		return nil, fmt.Errorf("query closed usage rebate days: %w", err)
	}
	defer func() { _ = rows.Close() }()
	days := make(map[string]bool)
	for rows.Next() {
		var day string
		if err := rows.Scan(&day); err != nil {
			return nil, err
		}
		days[day] = true
	}
	return days, rows.Err()
}

func (r *affiliateRepository) MokRecentUsageRebateDays(ctx context.Context, limit int) ([]service.MokAffiliateUsageRebateDay, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT to_char(day, 'YYYY-MM-DD'), window_start, window_end, invitee_count,
       usage_total::double precision, rebate_total::double precision, skipped_reason, settled_at
FROM mok_affiliate_usage_rebate_days
ORDER BY day DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent usage rebate days: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var days []service.MokAffiliateUsageRebateDay
	for rows.Next() {
		var day service.MokAffiliateUsageRebateDay
		if err := rows.Scan(&day.Day, &day.WindowStart, &day.WindowEnd, &day.InviteeCount,
			&day.UsageTotal, &day.RebateTotal, &day.SkippedReason, &day.SettledAt); err != nil {
			return nil, err
		}
		days = append(days, day)
	}
	return days, rows.Err()
}
