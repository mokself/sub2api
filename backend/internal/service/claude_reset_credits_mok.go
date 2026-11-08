package service

// [mok] Fork-only file (see FORK.md). It reuses ClaudeResetCreditService's account
// checks, token, proxy and request headers to read the plan fields of a Claude OAuth
// profile, so it stays in step with how upstream calls the profile endpoint.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// MokClaudeProfile holds the plan fields of GET /api/oauth/profile. It deliberately
// carries no account, email or organization identifiers.
type MokClaudeProfile struct {
	OrganizationType   string
	RateLimitTier      string
	SeatTier           string
	SubscriptionStatus string
}

// MokFetchClaudeProfile reads the plan fields of one Claude OAuth account. Accounts that
// are not Claude OAuth or lack the user:profile scope fail before any network call.
func (s *ClaudeResetCreditService) MokFetchClaudeProfile(ctx context.Context, accountID int64) (*MokClaudeProfile, error) {
	_, token, proxy, err := s.account(ctx, accountID)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, claudeResetProfileURL, nil)
	if err != nil {
		return nil, err
	}
	s.headers(ctx, req, token)
	resp, err := s.do(req, proxy)
	if err != nil {
		return nil, infraerrors.ServiceUnavailable("MOK_CLAUDE_PROFILE_FAILED", "OAuth profile request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, infraerrors.New(http.StatusBadGateway, "MOK_CLAUDE_PROFILE_FAILED", fmt.Sprintf("OAuth profile upstream HTTP %d", resp.StatusCode))
	}
	var body map[string]json.RawMessage
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body) != nil || body == nil {
		return nil, infraerrors.New(http.StatusBadGateway, "MOK_CLAUDE_PROFILE_INVALID", "invalid OAuth profile")
	}
	return parseMokClaudeProfile(body)
}

// parseMokClaudeProfile reads the plan fields from the "organization" object and falls
// back to top-level fields. Null or non-string values count as empty.
func parseMokClaudeProfile(body map[string]json.RawMessage) (*MokClaudeProfile, error) {
	var org map[string]json.RawMessage
	if raw, ok := body["organization"]; ok {
		_ = json.Unmarshal(raw, &org)
	}
	field := func(key string) string {
		for _, m := range []map[string]json.RawMessage{org, body} {
			var v string
			if raw, ok := m[key]; ok && json.Unmarshal(raw, &v) == nil && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
		return ""
	}
	p := &MokClaudeProfile{
		OrganizationType:   field("organization_type"),
		RateLimitTier:      field("rate_limit_tier"),
		SeatTier:           field("seat_tier"),
		SubscriptionStatus: field("subscription_status"),
	}
	if org == nil && p.OrganizationType == "" && p.RateLimitTier == "" {
		return nil, infraerrors.New(http.StatusBadGateway, "MOK_CLAUDE_PROFILE_INVALID", "invalid OAuth profile")
	}
	return p, nil
}
