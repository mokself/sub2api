package mok

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	// A plan rarely changes, so a successful fetch is reused for this long.
	claudePlanSuccessTTL = 12 * time.Hour
	// After a failed attempt, automatic retries wait this long; a manual refresh does not.
	claudePlanFailureRetry = 10 * time.Minute
	// Covers a token refresh plus the profile request (whose HTTP client allows 25s).
	claudePlanFetchTimeout = 45 * time.Second
	// Upper bound of concurrent profile requests across all callers.
	claudePlanFetchConcurrency = 8
	claudePlanMaxIDs           = 100
)

// Statuses of a ClaudePlanItem.
const (
	ClaudePlanStatusOK          = "ok"
	ClaudePlanStatusError       = "error"
	ClaudePlanStatusUnsupported = "unsupported"
)

// ClaudePlanItem is one account's plan as shown in the admin account list. With status
// "error" the plan fields still hold the last successful fetch, if there was one.
type ClaudePlanItem struct {
	AccountID          int64      `json:"account_id"`
	Status             string     `json:"status"`
	Reason             string     `json:"reason,omitempty"`
	Message            string     `json:"message,omitempty"`
	OrganizationType   string     `json:"organization_type"`
	RateLimitTier      string     `json:"rate_limit_tier"`
	SeatTier           string     `json:"seat_tier"`
	SubscriptionStatus string     `json:"subscription_status"`
	FetchedAt          *time.Time `json:"fetched_at"`
	AttemptedAt        *time.Time `json:"attempted_at"`
}

// claudePlanRecord is a row of mok_claude_plan_tiers.
type claudePlanRecord struct {
	AccountID    int64
	Profile      service.MokClaudeProfile
	FetchedAt    *time.Time
	ErrorReason  string
	ErrorMessage string
	AttemptedAt  time.Time
}

type claudeProfileFetcher interface {
	MokFetchClaudeProfile(ctx context.Context, accountID int64) (*service.MokClaudeProfile, error)
}

type claudePlanRepository interface {
	List(ctx context.Context, ids []int64) (map[int64]claudePlanRecord, error)
	SaveSuccess(ctx context.Context, id int64, p service.MokClaudeProfile, at time.Time) error
	SaveFailure(ctx context.Context, id int64, reason, message string, at time.Time) error
}

// ClaudePlanService serves cached Claude plans and refreshes the missing or stale ones.
type ClaudePlanService struct {
	fetcher claudeProfileFetcher
	repo    claudePlanRepository
	now     func() time.Time
	slots   chan struct{}
	flight  singleflight.Group
}

func NewClaudePlanService(fetcher *service.ClaudeResetCreditService, store *ClaudePlanStore) *ClaudePlanService {
	return newClaudePlanService(fetcher, store)
}

func newClaudePlanService(fetcher claudeProfileFetcher, repo claudePlanRepository) *ClaudePlanService {
	return &ClaudePlanService{
		fetcher: fetcher,
		repo:    repo,
		now:     time.Now,
		slots:   make(chan struct{}, claudePlanFetchConcurrency),
	}
}

// Get returns one item per id, in order. Missing or stale entries are fetched before
// returning; refresh fetches all of them.
func (s *ClaudePlanService) Get(ctx context.Context, ids []int64, refresh bool) ([]ClaudePlanItem, error) {
	cached, err := s.repo.List(ctx, ids)
	if err != nil {
		return nil, err
	}
	now := s.now()
	items := make([]ClaudePlanItem, len(ids))
	var g errgroup.Group
	for i, id := range ids {
		rec, ok := cached[id]
		if ok && !refresh && !claudePlanStale(rec, now) {
			items[i] = claudePlanItemFromRecord(rec)
			continue
		}
		var prev *claudePlanRecord
		if ok {
			prev = &rec
		}
		g.Go(func() error {
			items[i] = s.fetch(ctx, id, prev)
			return nil
		})
	}
	_ = g.Wait()
	return items, nil
}

// claudePlanStale reports whether a cached row should be fetched again automatically.
func claudePlanStale(rec claudePlanRecord, now time.Time) bool {
	if rec.ErrorReason != "" {
		return now.Sub(rec.AttemptedAt) >= claudePlanFailureRetry
	}
	return rec.FetchedAt == nil || now.Sub(*rec.FetchedAt) >= claudePlanSuccessTTL
}

// fetch runs one profile request per account at a time. The request is detached from
// the caller, so an admin closing the page neither fails a fetch shared with others nor
// loses its result.
func (s *ClaudePlanService) fetch(ctx context.Context, id int64, prev *claudePlanRecord) ClaudePlanItem {
	v, _, _ := s.flight.Do(strconv.FormatInt(id, 10), func() (any, error) {
		fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), claudePlanFetchTimeout)
		defer cancel()
		select {
		case s.slots <- struct{}{}:
			defer func() { <-s.slots }()
		case <-fctx.Done():
			return claudePlanFailure(id, prev, "MOK_CLAUDE_PROFILE_BUSY", "too many profile requests in progress", s.now()), nil
		}
		return s.fetchOnce(fctx, id, prev), nil
	})
	item, ok := v.(ClaudePlanItem)
	if !ok {
		return claudePlanFailure(id, prev, "MOK_CLAUDE_PROFILE_FAILED", "OAuth profile unavailable", s.now())
	}
	return item
}

func (s *ClaudePlanService) fetchOnce(ctx context.Context, id int64, prev *claudePlanRecord) ClaudePlanItem {
	profile, err := s.fetcher.MokFetchClaudeProfile(ctx, id)
	now := s.now()
	if err == nil {
		if saveErr := s.repo.SaveSuccess(ctx, id, *profile, now); saveErr != nil {
			slog.Warn("mok: save claude plan failed", "account_id", id, "error", saveErr)
		}
		return claudePlanItemFromRecord(claudePlanRecord{AccountID: id, Profile: *profile, FetchedAt: &now, AttemptedAt: now})
	}
	if reason, ok := claudePlanUnsupportedReason(err); ok {
		return ClaudePlanItem{AccountID: id, Status: ClaudePlanStatusUnsupported, Reason: reason}
	}
	appErr := infraerrors.FromError(err)
	if saveErr := s.repo.SaveFailure(ctx, id, appErr.Reason, appErr.Message, now); saveErr != nil {
		slog.Warn("mok: save claude plan failure failed", "account_id", id, "error", saveErr)
	}
	return claudePlanFailure(id, prev, appErr.Reason, appErr.Message, now)
}

// claudePlanFailure keeps the plan of the last successful fetch next to the new error.
func claudePlanFailure(id int64, prev *claudePlanRecord, reason, message string, at time.Time) ClaudePlanItem {
	rec := claudePlanRecord{AccountID: id, ErrorReason: reason, ErrorMessage: message, AttemptedAt: at}
	if prev != nil {
		rec.Profile = prev.Profile
		rec.FetchedAt = prev.FetchedAt
	}
	return claudePlanItemFromRecord(rec)
}

// claudePlanUnsupportedReason recognizes accounts whose plan cannot be read at all. The
// reasons are the ones upstream's ClaudeResetCreditService.account raises before any
// network call; an unrecognized reason is reported as an ordinary error instead.
func claudePlanUnsupportedReason(err error) (string, bool) {
	switch {
	case infraerrors.Reason(err) == "CLAUDE_RESET_OAUTH_REQUIRED":
		return "not_claude_oauth", true
	case infraerrors.Reason(err) == "CLAUDE_RESET_PROFILE_SCOPE_REQUIRED":
		return "missing_profile_scope", true
	case errors.Is(err, service.ErrAccountNotFound):
		return "account_not_found", true
	}
	return "", false
}

func claudePlanItemFromRecord(rec claudePlanRecord) ClaudePlanItem {
	item := ClaudePlanItem{
		AccountID:          rec.AccountID,
		Status:             ClaudePlanStatusOK,
		OrganizationType:   rec.Profile.OrganizationType,
		RateLimitTier:      rec.Profile.RateLimitTier,
		SeatTier:           rec.Profile.SeatTier,
		SubscriptionStatus: rec.Profile.SubscriptionStatus,
		FetchedAt:          rec.FetchedAt,
	}
	if !rec.AttemptedAt.IsZero() {
		at := rec.AttemptedAt
		item.AttemptedAt = &at
	}
	if rec.ErrorReason != "" {
		item.Status = ClaudePlanStatusError
		item.Reason = rec.ErrorReason
		item.Message = rec.ErrorMessage
	}
	return item
}
