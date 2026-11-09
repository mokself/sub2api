package repository

// [mok] Fork-only file (see FORK.md): team recharge volume for tiered affiliate rebates.

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.MokAffiliateTeamVolumeReader = (*affiliateRepository)(nil)

// The team is the inviter plus everyone they invited directly. Payment orders count once
// completed and not refunded (a pending refund request still counts); balance orders count
// without their bonus, like the rebate base. Payment orders fulfil balance through a redeem
// code of their own, so those codes are left out of the redeem part. Amounts are summed as
// numeric and converted once at the end.
const mokTeamRechargeVolumeSQL = `
WITH team AS (
	SELECT $1::bigint AS user_id
	UNION
	SELECT user_id FROM user_affiliates WHERE inviter_id = $1::bigint
),
orders AS (
	SELECT COALESCE(SUM(CASE WHEN po.order_type = 'balance'
	                         THEN GREATEST(po.amount - po.bonus_amount, 0)
	                         ELSE po.amount END), 0) AS total
	FROM payment_orders po
	JOIN team t ON t.user_id = po.user_id
	WHERE po.order_type IN ('balance', 'subscription')
	  AND po.status IN ('COMPLETED', 'REFUND_REQUESTED', 'REFUND_FAILED')
	  AND ($2::bigint IS NULL OR po.id <> $2::bigint)
),
codes AS (
	SELECT COALESCE(SUM(rc.value), 0) AS total
	FROM redeem_codes rc
	JOIN team t ON t.user_id = rc.used_by
	WHERE rc.type = 'balance'
	  AND rc.status = 'used'
	  AND rc.value > 0
	  AND NOT EXISTS (
	      SELECT 1 FROM payment_orders po
	      WHERE po.user_id = rc.used_by AND po.recharge_code = rc.code)
)
SELECT (orders.total + codes.total)::double precision FROM orders, codes`

// MokTeamRechargeVolume implements service.MokAffiliateTeamVolumeReader. It runs inside the
// caller's transaction when the context carries one.
func (r *affiliateRepository) MokTeamRechargeVolume(ctx context.Context, inviterID int64, excludeOrderID *int64) (float64, error) {
	var exclude any
	if excludeOrderID != nil {
		exclude = *excludeOrderID
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, mokTeamRechargeVolumeSQL, inviterID, exclude)
	if err != nil {
		return 0, fmt.Errorf("query team recharge volume: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var volume float64
	if rows.Next() {
		if err := rows.Scan(&volume); err != nil {
			return 0, err
		}
	}
	return volume, rows.Close()
}
