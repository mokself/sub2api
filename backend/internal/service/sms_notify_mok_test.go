//go:build unit

package service

// [mok] Fork-only tests for sms_notify_mok.go (see FORK.md).

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mokSMSRecorder records the notices it is handed.
type mokSMSRecorder struct {
	mu      sync.Mutex
	wanted  map[string]bool
	asked   []string
	notices []MokSMSNotice
}

func (r *mokSMSRecorder) MokSMSWanted(_ context.Context, event string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.asked = append(r.asked, event)
	return r.wanted[event]
}

func (r *mokSMSRecorder) MokNotifySMS(_ context.Context, notice MokSMSNotice) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notices = append(r.notices, notice)
}

func installMokSMSRecorder(t *testing.T, events ...string) *mokSMSRecorder {
	t.Helper()
	r := &mokSMSRecorder{wanted: map[string]bool{}}
	for _, e := range events {
		r.wanted[e] = true
	}
	MokSetSMSNotifier(r)
	t.Cleanup(func() { MokSetSMSNotifier(nil) })
	return r
}

func TestMokSMSBalanceLowGoesOutOnTheCrossingWithoutAnyEmail(t *testing.T) {
	r := installMokSMSRecorder(t, MokSMSEventBalanceLow)
	s, repo := newBalanceNotifyServiceForTest()
	repo.data[SettingKeyBalanceLowNotifyEnabled] = "true"
	repo.data[SettingKeyBalanceLowNotifyThreshold] = "10"
	user := &User{ID: 42, BalanceNotifyEnabled: true} // no notification emails at all

	s.CheckBalanceAfterDeduction(context.Background(), user, 12, 3.5)

	require.Len(t, r.notices, 1)
	n := r.notices[0]
	require.Equal(t, MokSMSEventBalanceLow, n.Event)
	require.Equal(t, int64(42), n.UserID)
	require.Equal(t, []string{"8.50", "10.00"}, n.Params)
	require.Equal(t, "user-42-"+mokSMSDay(time.Now()), n.DedupKey)
}

func TestMokSMSBalanceLowFollowsTheEmailConditions(t *testing.T) {
	r := installMokSMSRecorder(t, MokSMSEventBalanceLow)
	s, repo := newBalanceNotifyServiceForTest()
	repo.data[SettingKeyBalanceLowNotifyEnabled] = "true"
	repo.data[SettingKeyBalanceLowNotifyThreshold] = "10"

	s.CheckBalanceAfterDeduction(context.Background(), &User{ID: 1, BalanceNotifyEnabled: false}, 12, 3)
	s.CheckBalanceAfterDeduction(context.Background(), &User{ID: 1, BalanceNotifyEnabled: true}, 30, 3)
	repo.data[SettingKeyBalanceLowNotifyEnabled] = "false"
	s.CheckBalanceAfterDeduction(context.Background(), &User{ID: 1, BalanceNotifyEnabled: true}, 12, 3)

	require.Empty(t, r.notices, "user switch off, no crossing, global switch off")
}

func TestMokSMSNothingWhenNotWantedOrNotInstalled(t *testing.T) {
	s, repo := newBalanceNotifyServiceForTest()
	repo.data[SettingKeyBalanceLowNotifyEnabled] = "true"
	repo.data[SettingKeyBalanceLowNotifyThreshold] = "10"
	user := &User{ID: 1, BalanceNotifyEnabled: true}

	MokSetSMSNotifier(nil)
	s.CheckBalanceAfterDeduction(context.Background(), user, 12, 3) // must not panic

	r := installMokSMSRecorder(t) // installed, nothing wanted
	s.CheckBalanceAfterDeduction(context.Background(), user, 12, 3)
	require.Equal(t, []string{MokSMSEventBalanceLow}, r.asked)
	require.Empty(t, r.notices)
}

func mokQuotaAccount() *Account {
	return &Account{
		ID:       7,
		Name:     "claude-pool-1",
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			"quota_notify_daily_enabled":   true,
			"quota_notify_daily_threshold": 100.0,
		},
	}
}

func TestMokSMSAccountQuotaCrossingWithoutAdminEmails(t *testing.T) {
	r := installMokSMSRecorder(t, MokSMSEventAccountQuota)
	s, repo := newBalanceNotifyServiceForTest()
	repo.data[SettingKeyAccountQuotaNotifyEnabled] = "true" // and no admin emails

	// Daily trigger point: 1000 - 100 = 900. 880 -> 920 crosses it.
	s.CheckAccountQuotaAfterIncrement(context.Background(), mokQuotaAccount(), 40,
		&AccountQuotaState{DailyUsed: 920, DailyLimit: 1000})

	require.Len(t, r.notices, 1)
	n := r.notices[0]
	require.Equal(t, MokSMSEventAccountQuota, n.Event)
	require.Zero(t, n.UserID, "admin alert phones")
	require.Equal(t, []string{"claude-pool-1", "日限额", "80.00", "1000.00"}, n.Params)
	require.Equal(t, "account-7-daily-"+mokSMSDay(time.Now()), n.DedupKey)
}

func TestMokSMSAccountQuotaNeedsTheCrossingAndTheGlobalSwitch(t *testing.T) {
	r := installMokSMSRecorder(t, MokSMSEventAccountQuota)
	s, repo := newBalanceNotifyServiceForTest()

	repo.data[SettingKeyAccountQuotaNotifyEnabled] = "false"
	s.CheckAccountQuotaAfterIncrement(context.Background(), mokQuotaAccount(), 40, &AccountQuotaState{DailyUsed: 920, DailyLimit: 1000})

	repo.data[SettingKeyAccountQuotaNotifyEnabled] = "true"
	s.CheckAccountQuotaAfterIncrement(context.Background(), mokQuotaAccount(), 10, &AccountQuotaState{DailyUsed: 950, DailyLimit: 1000})
	s.CheckAccountQuotaAfterIncrement(context.Background(), mokQuotaAccount(), 10, &AccountQuotaState{DailyUsed: 850, DailyLimit: 1000})
	s.CheckAccountQuotaAfterIncrement(context.Background(), mokQuotaAccount(), 0, &AccountQuotaState{DailyUsed: 920, DailyLimit: 1000})

	require.Empty(t, r.notices, "global switch off, already past, not yet reached, no cost")
}

func TestMokSMSOpsAlert(t *testing.T) {
	r := installMokSMSRecorder(t, MokSMSEventOpsAlert)
	s := &OpsAlertEvaluatorService{}
	rule := &OpsAlertRule{ID: 3, Name: "上游错误率", Severity: "P0", Threshold: 5}
	value, threshold := 12.345, 5.0
	event := &OpsAlertEvent{ID: 99, RuleID: 3, Severity: "P0", MetricValue: &value, ThresholdValue: &threshold}

	s.mokSendOpsAlertSMS(context.Background(), nil, rule, event)

	require.Len(t, r.notices, 1)
	n := r.notices[0]
	require.Equal(t, MokSMSEventOpsAlert, n.Event)
	require.Equal(t, "critical", n.Level)
	require.Equal(t, []string{"上游错误率", "P0", "12.35", "5"}, n.Params)
	require.Equal(t, "event-99", n.DedupKey)
}

func TestMokSMSOpsAlertHonoursSilencing(t *testing.T) {
	r := installMokSMSRecorder(t, MokSMSEventOpsAlert)
	s := &OpsAlertEvaluatorService{}
	rule := &OpsAlertRule{ID: 3, Name: "r", Severity: "P2"}
	event := &OpsAlertEvent{ID: 1, Severity: "P2"}
	silenced := &OpsAlertRuntimeSettings{Silencing: OpsAlertSilencingSettings{
		Enabled:            true,
		GlobalUntilRFC3339: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}}

	s.mokSendOpsAlertSMS(context.Background(), silenced, rule, event)
	s.mokSendOpsAlertSMS(context.Background(), nil, rule, nil)
	s.mokSendOpsAlertSMS(context.Background(), nil, rule, &OpsAlertEvent{})
	require.Empty(t, r.notices)

	s.mokSendOpsAlertSMS(context.Background(), nil, rule, event)
	require.Len(t, r.notices, 1)
	require.Equal(t, "info", r.notices[0].Level)
	require.Equal(t, []string{"r", "P2", "-", "0"}, r.notices[0].Params)
}
