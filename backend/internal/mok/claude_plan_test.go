//go:build unit

package mok

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var testNow = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

// memPlanRepo mirrors the SQL semantics of ClaudePlanStore.
type memPlanRepo struct {
	mu   sync.Mutex
	rows map[int64]claudePlanRecord
}

func newMemPlanRepo(rows ...claudePlanRecord) *memPlanRepo {
	r := &memPlanRepo{rows: map[int64]claudePlanRecord{}}
	for _, row := range rows {
		r.rows[row.AccountID] = row
	}
	return r
}

func (r *memPlanRepo) List(_ context.Context, ids []int64) (map[int64]claudePlanRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[int64]claudePlanRecord{}
	for _, id := range ids {
		if row, ok := r.rows[id]; ok {
			out[id] = row
		}
	}
	return out, nil
}

func (r *memPlanRepo) SaveSuccess(_ context.Context, id int64, p service.MokClaudeProfile, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[id] = claudePlanRecord{AccountID: id, Profile: p, FetchedAt: &at, AttemptedAt: at}
	return nil
}

func (r *memPlanRepo) SaveFailure(_ context.Context, id int64, reason, message string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	row := r.rows[id]
	row.AccountID, row.ErrorReason, row.ErrorMessage, row.AttemptedAt = id, reason, message, at
	r.rows[id] = row
	return nil
}

func (r *memPlanRepo) get(id int64) (claudePlanRecord, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[id]
	return row, ok
}

func newTestPlanService(fetcher *fakeProfileFetcher, repo *memPlanRepo) *ClaudePlanService {
	s := newClaudePlanService(fetcher, repo)
	s.now = func() time.Time { return testNow }
	return s
}

func at(ago time.Duration) *time.Time {
	t := testNow.Add(-ago)
	return &t
}

func TestClaudePlanServesFreshCacheWithoutFetching(t *testing.T) {
	fetcher := &fakeProfileFetcher{}
	repo := newMemPlanRepo(claudePlanRecord{
		AccountID: 1, FetchedAt: at(time.Hour), AttemptedAt: *at(time.Hour),
		Profile: service.MokClaudeProfile{OrganizationType: "claude_pro", RateLimitTier: "default_claude_ai"},
	})

	items, err := newTestPlanService(fetcher, repo).Get(context.Background(), []int64{1}, false)

	require.NoError(t, err)
	require.Empty(t, fetcher.called())
	require.Equal(t, ClaudePlanStatusOK, items[0].Status)
	require.Equal(t, "default_claude_ai", items[0].RateLimitTier)
}

func TestClaudePlanFetchesMissingAndStaleInRequestOrder(t *testing.T) {
	fetcher := &fakeProfileFetcher{profiles: map[int64]*service.MokClaudeProfile{
		3: {OrganizationType: "claude_max", RateLimitTier: "default_claude_max_5x"},
	}}
	repo := newMemPlanRepo(claudePlanRecord{AccountID: 3, FetchedAt: at(13 * time.Hour), AttemptedAt: *at(13 * time.Hour)})

	items, err := newTestPlanService(fetcher, repo).Get(context.Background(), []int64{3, 2}, false)

	require.NoError(t, err)
	require.ElementsMatch(t, []int64{2, 3}, fetcher.called())
	require.Equal(t, []int64{3, 2}, []int64{items[0].AccountID, items[1].AccountID})
	require.Equal(t, "default_claude_max_5x", items[0].RateLimitTier)
	require.Equal(t, "default_claude_max_20x", items[1].RateLimitTier)
	require.Equal(t, testNow, *items[1].FetchedAt)
	row, ok := repo.get(2)
	require.True(t, ok)
	require.Equal(t, "claude_max", row.Profile.OrganizationType)
}

func TestClaudePlanRefreshForcesFetch(t *testing.T) {
	fetcher := &fakeProfileFetcher{}
	repo := newMemPlanRepo(claudePlanRecord{AccountID: 1, FetchedAt: at(time.Minute), AttemptedAt: *at(time.Minute)})

	_, err := newTestPlanService(fetcher, repo).Get(context.Background(), []int64{1}, true)

	require.NoError(t, err)
	require.Equal(t, []int64{1}, fetcher.called())
}

func TestClaudePlanWaitsBeforeRetryingAFailure(t *testing.T) {
	fetcher := &fakeProfileFetcher{}
	failed := func(ago time.Duration) claudePlanRecord {
		return claudePlanRecord{AccountID: 1, ErrorReason: "MOK_CLAUDE_PROFILE_FAILED", ErrorMessage: "OAuth profile upstream HTTP 401", AttemptedAt: *at(ago)}
	}

	items, err := newTestPlanService(fetcher, newMemPlanRepo(failed(5*time.Minute))).Get(context.Background(), []int64{1}, false)
	require.NoError(t, err)
	require.Empty(t, fetcher.called())
	require.Equal(t, ClaudePlanStatusError, items[0].Status)
	require.Equal(t, "OAuth profile upstream HTTP 401", items[0].Message)

	items, err = newTestPlanService(fetcher, newMemPlanRepo(failed(11*time.Minute))).Get(context.Background(), []int64{1}, false)
	require.NoError(t, err)
	require.Equal(t, []int64{1}, fetcher.called())
	require.Equal(t, ClaudePlanStatusOK, items[0].Status)
}

func TestClaudePlanErrorKeepsLastKnownPlan(t *testing.T) {
	fetcher := &fakeProfileFetcher{errs: map[int64]error{
		1: infraerrors.ServiceUnavailable("CLAUDE_RESET_PROXY_UNAVAILABLE", "account proxy unavailable"),
	}}
	repo := newMemPlanRepo(claudePlanRecord{
		AccountID: 1, FetchedAt: at(20 * time.Hour), AttemptedAt: *at(20 * time.Hour),
		Profile: service.MokClaudeProfile{OrganizationType: "claude_max", RateLimitTier: "default_claude_max_20x"},
	})

	items, err := newTestPlanService(fetcher, repo).Get(context.Background(), []int64{1}, false)

	require.NoError(t, err)
	item := items[0]
	require.Equal(t, ClaudePlanStatusError, item.Status)
	require.Equal(t, "CLAUDE_RESET_PROXY_UNAVAILABLE", item.Reason)
	require.Equal(t, "default_claude_max_20x", item.RateLimitTier)
	require.Equal(t, *at(20 * time.Hour), *item.FetchedAt)
	require.Equal(t, testNow, *item.AttemptedAt)
	row, _ := repo.get(1)
	require.Equal(t, "CLAUDE_RESET_PROXY_UNAVAILABLE", row.ErrorReason)
	require.Equal(t, "default_claude_max_20x", row.Profile.RateLimitTier)
}

func TestClaudePlanUnsupportedAccountsAreNotStored(t *testing.T) {
	fetcher := &fakeProfileFetcher{errs: map[int64]error{
		1: infraerrors.BadRequest("CLAUDE_RESET_PROFILE_SCOPE_REQUIRED", "user:profile scope required"),
		2: infraerrors.BadRequest("CLAUDE_RESET_OAUTH_REQUIRED", "Claude OAuth account required"),
		3: service.ErrAccountNotFound,
	}}
	repo := newMemPlanRepo()

	items, err := newTestPlanService(fetcher, repo).Get(context.Background(), []int64{1, 2, 3}, false)

	require.NoError(t, err)
	reasons := []string{}
	for _, item := range items {
		require.Equal(t, ClaudePlanStatusUnsupported, item.Status)
		reasons = append(reasons, item.Reason)
	}
	require.Equal(t, []string{"missing_profile_scope", "not_claude_oauth", "account_not_found"}, reasons)
	require.Empty(t, repo.rows)
}

func TestParseClaudePlanIDs(t *testing.T) {
	ids, err := parseClaudePlanIDs(" 3, 1,3,,2 ")
	require.NoError(t, err)
	require.Equal(t, []int64{3, 1, 2}, ids)

	for _, raw := range []string{"", " , ", "a", "0", "-4", "1.5"} {
		_, err := parseClaudePlanIDs(raw)
		require.Error(t, err, "input %q", raw)
	}

	tooMany := make([]string, claudePlanMaxIDs+1)
	for i := range tooMany {
		tooMany[i] = strconv.Itoa(i + 1)
	}
	_, err = parseClaudePlanIDs(strings.Join(tooMany, ","))
	require.Error(t, err)
}
