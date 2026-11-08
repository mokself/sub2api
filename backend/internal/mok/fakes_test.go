//go:build unit || integration

package mok

import (
	"context"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// fakeProfileFetcher answers with a Max 20x plan unless an error or another plan is set
// for the account, and records which accounts were fetched.
type fakeProfileFetcher struct {
	mu       sync.Mutex
	calls    []int64
	profiles map[int64]*service.MokClaudeProfile
	errs     map[int64]error
}

func (f *fakeProfileFetcher) MokFetchClaudeProfile(_ context.Context, id int64) (*service.MokClaudeProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, id)
	if err := f.errs[id]; err != nil {
		return nil, err
	}
	if p := f.profiles[id]; p != nil {
		return p, nil
	}
	return &service.MokClaudeProfile{OrganizationType: "claude_max", RateLimitTier: "default_claude_max_20x"}, nil
}

func (f *fakeProfileFetcher) called() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.calls...)
}
