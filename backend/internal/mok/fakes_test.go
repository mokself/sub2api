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

// memorySettings is an in-memory service.SettingRepository.
type memorySettings struct {
	mu   sync.Mutex
	data map[string]string
}

func newMemorySettings() *memorySettings { return &memorySettings{data: map[string]string{}} }

func (m *memorySettings) Get(_ context.Context, key string) (*service.Setting, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[key]
	if !ok {
		return nil, service.ErrSettingNotFound
	}
	return &service.Setting{Key: key, Value: v}, nil
}

func (m *memorySettings) GetValue(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return v, nil
}

func (m *memorySettings) Set(_ context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *memorySettings) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, k := range keys {
		if v, err := m.GetValue(ctx, k); err == nil {
			out[k] = v
		}
	}
	return out, nil
}

func (m *memorySettings) SetMultiple(ctx context.Context, settings map[string]string) error {
	for k, v := range settings {
		_ = m.Set(ctx, k, v)
	}
	return nil
}

func (m *memorySettings) GetAll(context.Context) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]string{}
	for k, v := range m.data {
		out[k] = v
	}
	return out, nil
}

func (m *memorySettings) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}
