//go:build integration

package mok

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestClaudePlanStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := NewClaudePlanStore(openMigratedDB(t))
	first := time.Date(2026, 11, 8, 1, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)
	third := second.Add(time.Hour)

	rows, err := store.List(ctx, []int64{1})
	require.NoError(t, err)
	require.Empty(t, rows)

	// A failure before any success leaves the plan empty.
	require.NoError(t, store.SaveFailure(ctx, 1, "MOK_CLAUDE_PROFILE_FAILED", "OAuth profile request failed", first))
	rows, err = store.List(ctx, []int64{1, 2})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Nil(t, rows[1].FetchedAt)
	require.Equal(t, "MOK_CLAUDE_PROFILE_FAILED", rows[1].ErrorReason)
	require.True(t, first.Equal(rows[1].AttemptedAt))

	// A success stores the plan and clears the error.
	plan := service.MokClaudeProfile{OrganizationType: "claude_max", RateLimitTier: "default_claude_max_20x", SubscriptionStatus: "active"}
	require.NoError(t, store.SaveSuccess(ctx, 1, plan, second))
	rows, err = store.List(ctx, []int64{1})
	require.NoError(t, err)
	require.Equal(t, plan, rows[1].Profile)
	require.True(t, second.Equal(*rows[1].FetchedAt))
	require.Empty(t, rows[1].ErrorReason)
	require.Empty(t, rows[1].ErrorMessage)

	// A later failure keeps the plan and the time of the last success.
	require.NoError(t, store.SaveFailure(ctx, 1, "CLAUDE_RESET_PROXY_UNAVAILABLE", "account proxy unavailable", third))
	rows, err = store.List(ctx, []int64{1})
	require.NoError(t, err)
	require.Equal(t, plan, rows[1].Profile)
	require.True(t, second.Equal(*rows[1].FetchedAt))
	require.Equal(t, "CLAUDE_RESET_PROXY_UNAVAILABLE", rows[1].ErrorReason)
	require.True(t, third.Equal(rows[1].AttemptedAt))
}

func TestClaudePlanServiceWithStore(t *testing.T) {
	ctx := context.Background()
	fetcher := &fakeProfileFetcher{errs: map[int64]error{
		2: infraerrors.ServiceUnavailable("MOK_CLAUDE_PROFILE_FAILED", "OAuth profile request failed"),
	}}
	svc := newClaudePlanService(fetcher, NewClaudePlanStore(openMigratedDB(t)))

	items, err := svc.Get(ctx, []int64{1, 2}, false)
	require.NoError(t, err)
	require.Equal(t, ClaudePlanStatusOK, items[0].Status)
	require.Equal(t, ClaudePlanStatusError, items[1].Status)

	// Both results are cached now: the success for 12h, the failure for 10 minutes.
	items, err = svc.Get(ctx, []int64{1, 2}, false)
	require.NoError(t, err)
	require.Len(t, fetcher.called(), 2)
	require.Equal(t, "default_claude_max_20x", items[0].RateLimitTier)
	require.Equal(t, "OAuth profile request failed", items[1].Message)
}
