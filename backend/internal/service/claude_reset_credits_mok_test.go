//go:build unit

package service

// [mok] Fork-only tests for claude_reset_credits_mok.go (see FORK.md).

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type mokProfileAccounts struct{ account *Account }

func (m mokProfileAccounts) GetByID(context.Context, int64) (*Account, error) {
	if m.account == nil {
		return nil, ErrAccountNotFound
	}
	return m.account, nil
}

type mokProfileTokens struct{}

func (mokProfileTokens) GetAccessToken(context.Context, *Account) (string, error) {
	return "synthetic-token", nil
}

func newMokProfileService(account *Account, do func(*http.Request, string) (*http.Response, error)) *ClaudeResetCreditService {
	return &ClaudeResetCreditService{accounts: mokProfileAccounts{account}, tokens: mokProfileTokens{}, now: time.Now, do: do}
}

func mokClaudeOAuthAccount(scope string) *Account {
	return &Account{ID: 7, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Credentials: map[string]any{"scope": scope}}
}

func mokProfileResponse(status int, body string) func(*http.Request, string) (*http.Response, error) {
	return func(*http.Request, string) (*http.Response, error) {
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}
}

func TestMokFetchClaudeProfileReadsOrganizationPlan(t *testing.T) {
	var got *http.Request
	s := newMokProfileService(mokClaudeOAuthAccount("user:inference user:profile"), func(req *http.Request, proxy string) (*http.Response, error) {
		got = req
		return mokProfileResponse(http.StatusOK, `{
			"account": {"uuid": "a-1", "email": "someone@example.com"},
			"organization": {"uuid": "o-1", "organization_type": "claude_max", "rate_limit_tier": "default_claude_max_20x",
				"seat_tier": null, "subscription_status": ""}
		}`)(req, proxy)
	})

	profile, err := s.MokFetchClaudeProfile(context.Background(), 7)

	require.NoError(t, err)
	require.Equal(t, &MokClaudeProfile{OrganizationType: "claude_max", RateLimitTier: "default_claude_max_20x"}, profile)
	require.Equal(t, claudeResetProfileURL, got.URL.String())
	require.Equal(t, "Bearer synthetic-token", got.Header.Get("Authorization"))
	require.Equal(t, "oauth-2025-04-20", got.Header.Get("anthropic-beta"))
}

func TestMokFetchClaudeProfileFallsBackToTopLevelFields(t *testing.T) {
	s := newMokProfileService(mokClaudeOAuthAccount("user:profile"), mokProfileResponse(http.StatusOK,
		`{"organization_type": "claude_pro", "rate_limit_tier": "default_claude_ai", "subscription_status": "active"}`))

	profile, err := s.MokFetchClaudeProfile(context.Background(), 7)

	require.NoError(t, err)
	require.Equal(t, &MokClaudeProfile{OrganizationType: "claude_pro", RateLimitTier: "default_claude_ai", SubscriptionStatus: "active"}, profile)
}

func TestMokFetchClaudeProfileRejectsUnexpectedBody(t *testing.T) {
	for name, body := range map[string]string{"no plan fields": `{"account": {}}`, "not json": `<html>`, "null": `null`} {
		t.Run(name, func(t *testing.T) {
			s := newMokProfileService(mokClaudeOAuthAccount("user:profile"), mokProfileResponse(http.StatusOK, body))
			_, err := s.MokFetchClaudeProfile(context.Background(), 7)
			require.Equal(t, "MOK_CLAUDE_PROFILE_INVALID", infraerrors.Reason(err))
		})
	}
}

func TestMokFetchClaudeProfileReportsUpstreamFailures(t *testing.T) {
	s := newMokProfileService(mokClaudeOAuthAccount("user:profile"), mokProfileResponse(http.StatusUnauthorized, `{"error": {}}`))
	_, err := s.MokFetchClaudeProfile(context.Background(), 7)
	require.Equal(t, "MOK_CLAUDE_PROFILE_FAILED", infraerrors.Reason(err))
	require.Contains(t, infraerrors.Message(err), "HTTP 401")

	s = newMokProfileService(mokClaudeOAuthAccount("user:profile"), func(*http.Request, string) (*http.Response, error) {
		return nil, errors.New("dial tcp: connection refused")
	})
	_, err = s.MokFetchClaudeProfile(context.Background(), 7)
	require.Equal(t, "MOK_CLAUDE_PROFILE_FAILED", infraerrors.Reason(err))
	require.NotContains(t, infraerrors.Message(err), "dial tcp", "transport details must not reach the admin UI")
}

func TestMokFetchClaudeProfileChecksAccountBeforeNetwork(t *testing.T) {
	noNetwork := func(*http.Request, string) (*http.Response, error) {
		t.Fatal("network called")
		return nil, nil
	}

	_, err := newMokProfileService(mokClaudeOAuthAccount("user:inference"), noNetwork).MokFetchClaudeProfile(context.Background(), 7)
	require.Equal(t, "CLAUDE_RESET_PROFILE_SCOPE_REQUIRED", infraerrors.Reason(err))

	setupToken := &Account{Platform: PlatformAnthropic, Type: AccountTypeSetupToken, Credentials: map[string]any{"scope": "user:profile"}}
	_, err = newMokProfileService(setupToken, noNetwork).MokFetchClaudeProfile(context.Background(), 7)
	require.Equal(t, "CLAUDE_RESET_OAUTH_REQUIRED", infraerrors.Reason(err))

	_, err = newMokProfileService(nil, noNetwork).MokFetchClaudeProfile(context.Background(), 7)
	require.ErrorIs(t, err, ErrAccountNotFound)
}
