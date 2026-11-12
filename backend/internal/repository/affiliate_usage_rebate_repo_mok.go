package repository

// [mok] Fork-only file (see FORK.md): storage for hourly affiliate usage rebates
// (service/affiliate_usage_rebate_mok.go, migrations/mok_003_affiliate_usage_rebate_periods.sql).

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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

// The NOT EXISTS skips invitees whom the daily settlement of mok_002 already paid for that
// whole day: mok_003 leaves such rows in mok_affiliate_usage_rebates only for a day that was
// partly settled when the hourly settlement took over.
const mokSettleUsageRebateSQL = `
INSERT INTO mok_affiliate_usage_rebate_items
    (period_start, period_end, day, invitee_id, inviter_id, usage_amount, rate_percent, rebate_amount)
SELECT $1::timestamptz, $2::timestamptz, $3::date, $4::bigint, $5::bigint, $6::numeric, $7::numeric, $8::numeric
WHERE NOT EXISTS (
    SELECT 1 FROM mok_affiliate_usage_rebates daily
    WHERE daily.day = $3::date AND daily.invitee_id = $4::bigint
)
ON CONFLICT (period_start, invitee_id) DO NOTHING`

func (r *affiliateRepository) MokSettleUsageRebate(ctx context.Context, row service.MokAffiliateUsageRebateRow, freezeHours int) (bool, error) {
	var settled bool
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		res, err := txClient.ExecContext(txCtx, mokSettleUsageRebateSQL,
			row.PeriodStart, row.PeriodEnd, row.Day, row.InviteeID, row.InviterID, row.UsageAmount, row.RatePercent, row.RebateAmount)
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
				`UPDATE mok_affiliate_usage_rebate_items SET rebate_amount = 0 WHERE period_start = $1::timestamptz AND invitee_id = $2`,
				row.PeriodStart, row.InviteeID)
			return err
		}
		return nil
	})
	return settled, err
}

func (r *affiliateRepository) MokCloseUsageRebatePeriod(ctx context.Context, day string, periodStart, periodEnd time.Time, skippedReason string) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.ExecContext(ctx, `
INSERT INTO mok_affiliate_usage_rebate_periods
    (period_start, period_end, day, invitee_count, usage_total, rebate_total, skipped_reason)
SELECT $1::timestamptz, $2::timestamptz, $3::date, COUNT(*), COALESCE(SUM(usage_amount), 0), COALESCE(SUM(rebate_amount), 0), $4::text
FROM mok_affiliate_usage_rebate_items WHERE period_start = $1::timestamptz
ON CONFLICT (period_start) DO NOTHING`, periodStart, periodEnd, day, skippedReason)
	if err != nil {
		return fmt.Errorf("close usage rebate period: %w", err)
	}
	return nil
}

func (r *affiliateRepository) MokUsageRebateSettledUntil(ctx context.Context) (*time.Time, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `SELECT MAX(period_end) FROM mok_affiliate_usage_rebate_periods`)
	if err != nil {
		return nil, fmt.Errorf("query settled usage rebate periods: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var until sql.NullTime
	if rows.Next() {
		if err := rows.Scan(&until); err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if !until.Valid {
		return nil, nil
	}
	return &until.Time, nil
}

func (r *affiliateRepository) MokRecentUsageRebateDays(ctx context.Context, limit int) ([]service.MokAffiliateUsageRebateDay, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT to_char(d.day, 'YYYY-MM-DD'), d.period_count, d.skipped_count,
       (SELECT COUNT(DISTINCT i.invitee_id) FROM mok_affiliate_usage_rebate_items i WHERE i.day = d.day),
       d.usage_total, d.rebate_total
FROM (
    SELECT day,
           COUNT(*) AS period_count,
           COUNT(*) FILTER (WHERE skipped_reason <> '') AS skipped_count,
           COALESCE(SUM(usage_total), 0)::double precision AS usage_total,
           COALESCE(SUM(rebate_total), 0)::double precision AS rebate_total
    FROM mok_affiliate_usage_rebate_periods
    GROUP BY day
    ORDER BY day DESC
    LIMIT $1
) d
ORDER BY d.day DESC`, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent usage rebate days: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var days []service.MokAffiliateUsageRebateDay
	for rows.Next() {
		var day service.MokAffiliateUsageRebateDay
		if err := rows.Scan(&day.Day, &day.PeriodCount, &day.SkippedCount, &day.InviteeCount, &day.UsageTotal, &day.RebateTotal); err != nil {
			return nil, err
		}
		days = append(days, day)
	}
	return days, rows.Err()
}

// mokLikeEscaper makes a search text match literally inside a LIKE pattern.
var mokLikeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

const mokUsageRebateRecordsFromSQL = `
FROM mok_affiliate_usage_rebate_items i
LEFT JOIN users inviter ON inviter.id = i.inviter_id
LEFT JOIN users invitee ON invitee.id = i.invitee_id
WHERE ($1::bigint = 0 OR i.inviter_id = $1::bigint)
  AND ($2::text = '' OR LOWER(inviter.email) LIKE $2::text OR LOWER(inviter.username) LIKE $2::text
       OR LOWER(invitee.email) LIKE $2::text OR LOWER(invitee.username) LIKE $2::text)`

func (r *affiliateRepository) MokListUsageRebates(ctx context.Context, filter service.MokAffiliateUsageRebateFilter) ([]service.MokAffiliateUsageRebateRecord, int64, error) {
	filter = service.NormalizeMokAffiliateUsageRebateFilter(filter)
	client := clientFromContext(ctx, r.client)
	pattern := ""
	if filter.Search != "" {
		pattern = "%" + mokLikeEscaper.Replace(strings.ToLower(filter.Search)) + "%"
	}
	total, err := queryAffiliateRecordCount(ctx, client, "SELECT COUNT(*) "+mokUsageRebateRecordsFromSQL, filter.InviterID, pattern)
	if err != nil {
		return nil, 0, fmt.Errorf("count usage rebate records: %w", err)
	}
	rows, err := client.QueryContext(ctx, `
SELECT i.period_start, i.period_end, i.inviter_id, COALESCE(inviter.email, ''),
       i.invitee_id, COALESCE(invitee.email, ''),
       i.usage_amount::double precision, i.rate_percent::double precision, i.rebate_amount::double precision,
       i.created_at
`+mokUsageRebateRecordsFromSQL+`
ORDER BY i.period_start DESC, i.id DESC
LIMIT $3 OFFSET $4`, filter.InviterID, pattern, filter.PageSize, (filter.Page-1)*filter.PageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("query usage rebate records: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var records []service.MokAffiliateUsageRebateRecord
	for rows.Next() {
		var record service.MokAffiliateUsageRebateRecord
		if err := rows.Scan(&record.PeriodStart, &record.PeriodEnd, &record.InviterID, &record.InviterEmail,
			&record.InviteeID, &record.InviteeEmail,
			&record.UsageAmount, &record.RatePercent, &record.RebateAmount, &record.SettledAt); err != nil {
			return nil, 0, err
		}
		records = append(records, record)
	}
	return records, total, rows.Err()
}
