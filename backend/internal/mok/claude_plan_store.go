package mok

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ClaudePlanStore persists plans in mok_claude_plan_tiers
// (migrations/mok_001_claude_plan_tiers.sql).
type ClaudePlanStore struct {
	db *sql.DB
}

func NewClaudePlanStore(db *sql.DB) *ClaudePlanStore {
	return &ClaudePlanStore{db: db}
}

func (s *ClaudePlanStore) List(ctx context.Context, ids []int64) (map[int64]claudePlanRecord, error) {
	out := make(map[int64]claudePlanRecord, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT account_id, organization_type, rate_limit_tier, seat_tier, subscription_status,
		       fetched_at, error_reason, error_message, attempted_at
		FROM mok_claude_plan_tiers
		WHERE account_id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var rec claudePlanRecord
		var fetchedAt sql.NullTime
		if err := rows.Scan(&rec.AccountID, &rec.Profile.OrganizationType, &rec.Profile.RateLimitTier,
			&rec.Profile.SeatTier, &rec.Profile.SubscriptionStatus, &fetchedAt,
			&rec.ErrorReason, &rec.ErrorMessage, &rec.AttemptedAt); err != nil {
			return nil, err
		}
		if fetchedAt.Valid {
			t := fetchedAt.Time
			rec.FetchedAt = &t
		}
		out[rec.AccountID] = rec
	}
	return out, rows.Err()
}

// SaveSuccess stores a fetched plan and clears the error of earlier attempts.
func (s *ClaudePlanStore) SaveSuccess(ctx context.Context, id int64, p service.MokClaudeProfile, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mok_claude_plan_tiers
			(account_id, organization_type, rate_limit_tier, seat_tier, subscription_status,
			 fetched_at, error_reason, error_message, attempted_at)
		VALUES ($1, $2, $3, $4, $5, $6, '', '', $6)
		ON CONFLICT (account_id) DO UPDATE SET
			organization_type   = EXCLUDED.organization_type,
			rate_limit_tier     = EXCLUDED.rate_limit_tier,
			seat_tier           = EXCLUDED.seat_tier,
			subscription_status = EXCLUDED.subscription_status,
			fetched_at          = EXCLUDED.fetched_at,
			error_reason        = '',
			error_message       = '',
			attempted_at        = EXCLUDED.attempted_at`,
		id, p.OrganizationType, p.RateLimitTier, p.SeatTier, p.SubscriptionStatus, at)
	return err
}

// SaveFailure records a failed attempt and keeps the plan of the last successful fetch.
func (s *ClaudePlanStore) SaveFailure(ctx context.Context, id int64, reason, message string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mok_claude_plan_tiers (account_id, error_reason, error_message, attempted_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (account_id) DO UPDATE SET
			error_reason  = EXCLUDED.error_reason,
			error_message = EXCLUDED.error_message,
			attempted_at  = EXCLUDED.attempted_at`,
		id, reason, message, at)
	return err
}
