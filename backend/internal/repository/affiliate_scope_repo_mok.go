package repository

// [mok] Fork-only file (see FORK.md): the users the affiliate feature is switched on for
// (service/affiliate_scope_mok.go, migrations/mok_005_affiliate_selected_users.sql).

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.MokAffiliateScopeStore = (*affiliateRepository)(nil)

func mokQueryBool(ctx context.Context, client affiliateQueryExecer, query string, args ...any) (bool, error) {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	var value bool
	if rows.Next() {
		if err := rows.Scan(&value); err != nil {
			return false, err
		}
	}
	return value, rows.Err()
}

func (r *affiliateRepository) MokIsAffiliateUserSelected(ctx context.Context, userID int64) (bool, error) {
	client := clientFromContext(ctx, r.client)
	selected, err := mokQueryBool(ctx, client,
		`SELECT EXISTS (SELECT 1 FROM mok_affiliate_selected_users WHERE user_id = $1)`, userID)
	if err != nil {
		return false, fmt.Errorf("query affiliate selected user: %w", err)
	}
	return selected, nil
}

func (r *affiliateRepository) MokSetAffiliateUserSelected(ctx context.Context, userID int64, selected bool) error {
	client := clientFromContext(ctx, r.client)
	if !selected {
		if _, err := client.ExecContext(ctx, `DELETE FROM mok_affiliate_selected_users WHERE user_id = $1`, userID); err != nil {
			return fmt.Errorf("unselect affiliate user: %w", err)
		}
		return nil
	}
	exists, err := mokQueryBool(ctx, client,
		`SELECT EXISTS (SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL)`, userID)
	if err != nil {
		return fmt.Errorf("query user: %w", err)
	}
	if !exists {
		return service.ErrUserNotFound
	}
	if _, err := client.ExecContext(ctx,
		`INSERT INTO mok_affiliate_selected_users (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		return fmt.Errorf("select affiliate user: %w", err)
	}
	return nil
}

func (r *affiliateRepository) MokAffiliateUserHasQuota(ctx context.Context, userID int64) (bool, error) {
	client := clientFromContext(ctx, r.client)
	hasQuota, err := mokQueryBool(ctx, client, `
SELECT EXISTS (
    SELECT 1 FROM user_affiliates
    WHERE user_id = $1 AND (aff_quota > 0 OR aff_frozen_quota > 0)
)`, userID)
	if err != nil {
		return false, fmt.Errorf("query affiliate quota: %w", err)
	}
	return hasQuota, nil
}

const mokAffiliateSelectedUsersFromSQL = `
FROM mok_affiliate_selected_users m
JOIN users u ON u.id = m.user_id AND u.deleted_at IS NULL`

func (r *affiliateRepository) MokListAffiliateSelectedUsers(ctx context.Context, page, pageSize int) ([]service.MokAffiliateSelectedUser, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	client := clientFromContext(ctx, r.client)
	total, err := queryAffiliateRecordCount(ctx, client, "SELECT COUNT(*) "+mokAffiliateSelectedUsersFromSQL)
	if err != nil {
		return nil, 0, fmt.Errorf("count affiliate selected users: %w", err)
	}
	rows, err := client.QueryContext(ctx, `
SELECT m.user_id, COALESCE(u.email, ''), COALESCE(u.username, ''), m.created_at
`+mokAffiliateSelectedUsersFromSQL+`
ORDER BY m.created_at DESC, m.user_id DESC
LIMIT $1 OFFSET $2`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("query affiliate selected users: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var users []service.MokAffiliateSelectedUser
	for rows.Next() {
		var user service.MokAffiliateSelectedUser
		if err := rows.Scan(&user.UserID, &user.Email, &user.Username, &user.SelectedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, user)
	}
	return users, total, rows.Err()
}
