//go:build integration

package mok

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type sentSMS struct {
	phone, template string
	params          []string
}

// fakeSMSSender records the messages and fails when told to.
type fakeSMSSender struct {
	mu   sync.Mutex
	sent []sentSMS
	fail error
}

func (f *fakeSMSSender) Send(_ context.Context, _ SMSConfig, phone, templateID string, params []string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return "req-failed", f.fail
	}
	f.sent = append(f.sent, sentSMS{phone: phone, template: templateID, params: append([]string(nil), params...)})
	return "req-ok", nil
}

func (f *fakeSMSSender) messages() []sentSMS {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentSMS(nil), f.sent...)
}

// fakeSubscriptions serves List from a fixed slice, sorted by the caller.
type fakeSubscriptions struct {
	service.UserSubscriptionRepository
	subs []service.UserSubscription
}

func (f *fakeSubscriptions) List(_ context.Context, params pagination.PaginationParams, _, _ *int64, status, _, sortBy, sortOrder string) ([]service.UserSubscription, *pagination.PaginationResult, error) {
	if status != service.SubscriptionStatusActive || sortBy != "expires_at" || sortOrder != "asc" {
		return nil, nil, errors.New("unexpected query")
	}
	start := (params.Page - 1) * params.PageSize
	if start > len(f.subs) {
		start = len(f.subs)
	}
	end := start + params.PageSize
	if end > len(f.subs) {
		end = len(f.subs)
	}
	pages := (len(f.subs) + params.PageSize - 1) / params.PageSize
	return f.subs[start:end], &pagination.PaginationResult{Total: int64(len(f.subs)), Page: params.Page, PageSize: params.PageSize, Pages: pages}, nil
}

type smsFixture struct {
	db     *sql.DB
	svc    *SMSService
	sender *fakeSMSSender
	subs   *fakeSubscriptions
	clock  time.Time
}

func newSMSFixture(t *testing.T) *smsFixture {
	t.Helper()
	db := openMigratedDB(t)
	f := &smsFixture{db: db, sender: &fakeSMSSender{}, subs: &fakeSubscriptions{}}
	// Mid-morning in UTC, far from a day boundary.
	f.clock = time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	f.svc = NewSMSService(newMemorySettings(), NewSMSStore(db), f.sender, f.subs)
	f.svc.now = func() time.Time { return f.clock }
	f.svc.location = func() *time.Location { return time.UTC }
	return f
}

func (f *smsFixture) advance(d time.Duration) { f.clock = f.clock.Add(d) }

func (f *smsFixture) configure(t *testing.T, change func(*SMSConfig)) {
	t.Helper()
	cfg := readySMSConfig()
	if change != nil {
		change(&cfg)
	}
	_, err := f.svc.SaveConfig(context.Background(), cfg)
	require.NoError(t, err)
}

func readySMSConfig() SMSConfig {
	cfg := defaultSMSConfig()
	cfg.Enabled = true
	cfg.SecretID, cfg.SecretKey, cfg.SDKAppID, cfg.SignName = "id", "key", "1400000000", "测试站"
	cfg.Templates = SMSTemplates{VerifyCode: "100", BalanceLow: "200", SubscriptionExpiry: "300", OpsAlert: "400", AccountQuota: "500"}
	return cfg
}

type smsLogRow struct {
	event, phone, status, errorText, dedup string
	userID                                 sql.NullInt64
	params                                 string
}

func (f *smsFixture) logs(t *testing.T) []smsLogRow {
	t.Helper()
	rows, err := f.db.Query(`SELECT event, phone, status, error, COALESCE(dedup_key, ''), user_id, params::text FROM mok_sms_logs ORDER BY id`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out []smsLogRow
	for rows.Next() {
		var r smsLogRow
		require.NoError(t, rows.Scan(&r.event, &r.phone, &r.status, &r.errorText, &r.dedup, &r.userID, &r.params))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func requireReason(t *testing.T, err error, reason string) {
	t.Helper()
	require.Error(t, err)
	require.Contains(t, err.Error(), reason)
}

// bind sends a code to the phone and binds it with the code the fake sender saw.
func (f *smsFixture) bind(t *testing.T, userID int64, phone string) {
	t.Helper()
	ctx := context.Background()
	_, err := f.svc.SendBindCode(ctx, userID, phone)
	require.NoError(t, err)
	msgs := f.sender.messages()
	code := msgs[len(msgs)-1].params[0]
	_, err = f.svc.BindPhone(ctx, userID, phone, code)
	require.NoError(t, err)
}

func TestSMSBindPhone(t *testing.T) {
	f := newSMSFixture(t)
	ctx := context.Background()

	status, err := f.svc.PhoneStatus(ctx, 1)
	require.NoError(t, err)
	require.False(t, status.Available, "not configured")
	_, err = f.svc.SendBindCode(ctx, 1, "13800138000")
	requireReason(t, err, "MOK_SMS_UNAVAILABLE")

	f.configure(t, nil)
	status, err = f.svc.PhoneStatus(ctx, 1)
	require.NoError(t, err)
	require.True(t, status.Available)
	require.Equal(t, []string{service.MokSMSEventBalanceLow, service.MokSMSEventSubscriptionExpiry}, status.Events)
	require.Empty(t, status.Phone)

	_, err = f.svc.SendBindCode(ctx, 1, "12345")
	requireReason(t, err, "MOK_SMS_PHONE_INVALID")

	sent, err := f.svc.SendBindCode(ctx, 1, "+86 138 0013 8000")
	require.NoError(t, err)
	require.Equal(t, SMSCodeSent{CooldownSeconds: 60, ExpiresInSeconds: 300}, sent)
	msgs := f.sender.messages()
	require.Len(t, msgs, 1)
	require.Equal(t, "13800138000", msgs[0].phone)
	require.Equal(t, "100", msgs[0].template)
	require.Len(t, msgs[0].params[0], 6)
	require.Equal(t, "5", msgs[0].params[1])
	code := msgs[0].params[0]

	f.advance(30 * time.Second)
	_, err = f.svc.SendBindCode(ctx, 1, "13800138000")
	requireReason(t, err, "MOK_SMS_CODE_COOLDOWN")

	_, err = f.svc.BindPhone(ctx, 1, "13900139000", code)
	requireReason(t, err, "MOK_SMS_CODE_INVALID")
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	_, err = f.svc.BindPhone(ctx, 1, "13800138000", wrong)
	requireReason(t, err, "MOK_SMS_CODE_INVALID")

	status, err = f.svc.BindPhone(ctx, 1, "13800138000", code)
	require.NoError(t, err)
	require.Equal(t, "13800138000", status.Phone)
	require.True(t, status.Enabled)

	_, err = f.svc.BindPhone(ctx, 1, "13800138000", code)
	requireReason(t, err, "MOK_SMS_CODE_INVALID")

	logs := f.logs(t)
	require.Len(t, logs, 1)
	require.Equal(t, smsEventVerifyCode, logs[0].event)
	require.Equal(t, smsStatusSent, logs[0].status)
	require.Equal(t, `["******", "5"]`, logs[0].params, "the code is not stored")

	status, err = f.svc.SetPhoneEnabled(ctx, 1, false)
	require.NoError(t, err)
	require.False(t, status.Enabled)
	status, err = f.svc.RemovePhone(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, status.Phone)
	_, err = f.svc.SetPhoneEnabled(ctx, 1, true)
	requireReason(t, err, "MOK_SMS_PHONE_NOT_BOUND")
}

func TestSMSBindCodeExpiresAndWearsOut(t *testing.T) {
	f := newSMSFixture(t)
	f.configure(t, nil)
	ctx := context.Background()

	_, err := f.svc.SendBindCode(ctx, 1, "13800138000")
	require.NoError(t, err)
	code := f.sender.messages()[0].params[0]
	f.advance(5 * time.Minute)
	_, err = f.svc.BindPhone(ctx, 1, "13800138000", code)
	requireReason(t, err, "MOK_SMS_CODE_EXPIRED")

	_, err = f.svc.SendBindCode(ctx, 1, "13800138000")
	require.NoError(t, err)
	code = f.sender.messages()[1].params[0]
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	for i := 0; i < smsCodeMaxAttempts-1; i++ {
		_, err = f.svc.BindPhone(ctx, 1, "13800138000", wrong)
		requireReason(t, err, "MOK_SMS_CODE_INVALID")
	}
	_, err = f.svc.BindPhone(ctx, 1, "13800138000", wrong)
	requireReason(t, err, "MOK_SMS_CODE_EXPIRED")
	_, err = f.svc.BindPhone(ctx, 1, "13800138000", code)
	requireReason(t, err, "MOK_SMS_CODE_INVALID") // the code is gone after too many guesses
}

func TestSMSBindCodeDailyLimits(t *testing.T) {
	f := newSMSFixture(t)
	f.configure(t, nil)
	ctx := context.Background()

	for i := 0; i < smsCodeDailyPerUser; i++ {
		_, err := f.svc.SendBindCode(ctx, 1, "13800138000")
		require.NoError(t, err)
		f.advance(time.Minute)
	}
	_, err := f.svc.SendBindCode(ctx, 1, "13800138001")
	requireReason(t, err, "MOK_SMS_CODE_DAILY_LIMIT")

	// Other users cannot flood the same number either.
	_, err = f.svc.SendBindCode(ctx, 2, "13800138000")
	requireReason(t, err, "MOK_SMS_CODE_DAILY_LIMIT")
	_, err = f.svc.SendBindCode(ctx, 2, "13800138002")
	require.NoError(t, err)

	// A failed send does not count.
	f.sender.fail = errors.New("FailedOperation: boom")
	_, err = f.svc.SendBindCode(ctx, 3, "13800138003")
	requireReason(t, err, "MOK_SMS_SEND_FAILED")
	f.sender.fail = nil
	f.advance(time.Minute)

	f.advance(24 * time.Hour)
	_, err = f.svc.SendBindCode(ctx, 1, "13800138000")
	require.NoError(t, err, "a new day")
}

func TestSMSDryRunLogsInsteadOfSending(t *testing.T) {
	f := newSMSFixture(t)
	f.configure(t, func(c *SMSConfig) {
		c.DryRun = true
		c.SecretID, c.SecretKey, c.SDKAppID, c.SignName = "", "", "", ""
	})
	ctx := context.Background()

	_, err := f.svc.SendBindCode(ctx, 1, "13800138000")
	require.NoError(t, err)
	require.Empty(t, f.sender.messages())
	logs := f.logs(t)
	require.Len(t, logs, 1)
	require.Equal(t, smsStatusDryRun, logs[0].status)
	require.NotContains(t, logs[0].params, "******", "in dry-run mode the log shows the code")

	var code string
	require.NoError(t, f.db.QueryRow(`SELECT params->>0 FROM mok_sms_logs`).Scan(&code))
	require.Len(t, code, 6)
	status, err := f.svc.BindPhone(ctx, 1, "13800138000", code)
	require.NoError(t, err)
	require.Equal(t, "13800138000", status.Phone)
}

func TestSMSNotifyUser(t *testing.T) {
	f := newSMSFixture(t)
	f.configure(t, nil)
	ctx := context.Background()
	notice := service.MokSMSNotice{Event: service.MokSMSEventBalanceLow, UserID: 1, Params: []string{"8.50", "10.00"}, DedupKey: "user-1-2026-11-18"}

	f.svc.MokNotifySMS(ctx, notice)
	require.Empty(t, f.sender.messages(), "no phone bound")

	f.bind(t, 1, "13800138000")
	f.svc.MokNotifySMS(ctx, notice)
	f.svc.MokNotifySMS(ctx, notice)
	msgs := f.sender.messages()
	require.Len(t, msgs, 2, "the code, then one reminder: the second is a duplicate")
	require.Equal(t, sentSMS{phone: "13800138000", template: "200", params: []string{"8.50", "10.00"}}, msgs[1])

	_, err := f.svc.SetPhoneEnabled(ctx, 1, false)
	require.NoError(t, err)
	notice.DedupKey = "user-1-2026-11-19"
	f.svc.MokNotifySMS(ctx, notice)
	require.Len(t, f.sender.messages(), 2, "the phone is switched off")

	_, err = f.svc.SetPhoneEnabled(ctx, 1, true)
	require.NoError(t, err)
	f.configure(t, func(c *SMSConfig) { c.Events.BalanceLow = false })
	f.svc.MokNotifySMS(ctx, notice)
	require.Len(t, f.sender.messages(), 2, "the reminder is switched off")
}

func TestSMSDailyCapPerUser(t *testing.T) {
	f := newSMSFixture(t)
	f.configure(t, func(c *SMSConfig) { c.DailyLimit = 2 })
	ctx := context.Background()
	f.bind(t, 1, "13800138000")
	notify := func(key string) {
		f.svc.MokNotifySMS(ctx, service.MokSMSNotice{Event: service.MokSMSEventSubscriptionExpiry, UserID: 1, Params: []string{"Pro", "2026-11-25 10:00", "7"}, DedupKey: key})
	}

	f.sender.fail = errors.New("InternalError: try later")
	notify("a")
	f.sender.fail = nil
	notify("b")
	notify("c")
	notify("d")
	notify("d")

	logs := f.logs(t)
	var statuses []string
	for _, l := range logs[1:] {
		statuses = append(statuses, l.dedup+":"+l.status)
	}
	require.Equal(t, []string{"a:failed", "b:sent", "c:sent", "d:limited"}, statuses,
		"the verification code and the failed attempt do not count; a capped notice is not retried")
	require.Equal(t, "InternalError: try later", logs[1].errorText)
	require.Len(t, f.sender.messages(), 3)

	f.advance(24 * time.Hour)
	notify("e")
	require.Len(t, f.sender.messages(), 4, "a new day")
}

func TestSMSNotifyAdmins(t *testing.T) {
	f := newSMSFixture(t)
	f.configure(t, func(c *SMSConfig) { c.DailyLimit = 1 })
	ctx := context.Background()
	_, err := f.svc.SaveOpsAlert(ctx, SMSOpsAlert{Enabled: true, Phones: []string{"13900000001", "13900000002"}, MinLevel: "warning"})
	require.NoError(t, err)
	_, err = f.svc.SaveAccountQuota(ctx, SMSAccountQuota{Phones: []SMSQuotaPhone{{Phone: "13900000001"}, {Phone: "13900000003", Disabled: true}}})
	require.NoError(t, err)

	f.svc.MokNotifySMS(ctx, service.MokSMSNotice{Event: service.MokSMSEventOpsAlert, Level: "info", Params: []string{"r", "P2", "1", "2"}, DedupKey: "event-1"})
	require.Empty(t, f.sender.messages(), "below the minimum level")

	f.svc.MokNotifySMS(ctx, service.MokSMSNotice{Event: service.MokSMSEventOpsAlert, Level: "critical", Params: []string{"一个非常非常长的告警规则名称超过二十个字符的部分会被截断", "P0", "1", "2"}, DedupKey: "event-2"})
	msgs := f.sender.messages()
	require.Len(t, msgs, 2)
	require.Equal(t, "13900000001", msgs[0].phone)
	require.Equal(t, "13900000002", msgs[1].phone)
	require.Equal(t, "400", msgs[0].template)
	require.Equal(t, 20, len([]rune(msgs[0].params[0])), "long parameters are cut to the provider's variable limit")

	f.svc.MokNotifySMS(ctx, service.MokSMSNotice{Event: service.MokSMSEventAccountQuota, Params: []string{"a", "日限额", "1", "2"}, DedupKey: "account-1"})
	require.Len(t, f.sender.messages(), 2, "the daily cap counts per admin phone; the switched-off phone gets nothing")
	logs := f.logs(t)
	require.Len(t, logs, 3)
	require.Equal(t, "13900000001", logs[2].phone)
	require.Equal(t, smsStatusLimited, logs[2].status)
	for _, l := range logs {
		require.False(t, l.userID.Valid)
	}

	_, err = f.svc.SaveOpsAlert(ctx, SMSOpsAlert{Enabled: false, Phones: []string{"13900000001"}})
	require.NoError(t, err)
	f.svc.MokNotifySMS(ctx, service.MokSMSNotice{Event: service.MokSMSEventOpsAlert, Level: "critical", Params: []string{"r", "P0", "1", "2"}, DedupKey: "event-3"})
	require.Len(t, f.logs(t), 3, "the ops alert SMS is switched off")
}

func TestSMSAdminAlertSettings(t *testing.T) {
	f := newSMSFixture(t)
	ctx := context.Background()

	ops, err := f.svc.AdminOpsAlert(ctx)
	require.NoError(t, err)
	require.Equal(t, SMSOpsAlertView{SMSOpsAlert: SMSOpsAlert{Phones: []string{}}}, ops, "nothing set, SMS not set up")
	quota, err := f.svc.AdminAccountQuota(ctx)
	require.NoError(t, err)
	require.Equal(t, SMSAccountQuotaView{SMSAccountQuota: SMSAccountQuota{Phones: []SMSQuotaPhone{}}}, quota)

	_, err = f.svc.SaveOpsAlert(ctx, SMSOpsAlert{Enabled: true, Phones: []string{"12345"}})
	requireReason(t, err, "MOK_SMS_CONFIG_INVALID")

	ops, err = f.svc.SaveOpsAlert(ctx, SMSOpsAlert{Enabled: true, Phones: []string{"+86 139 0000 0001"}, MinLevel: "critical"})
	require.NoError(t, err)
	require.False(t, ops.Ready)
	f.configure(t, func(c *SMSConfig) { c.Templates.AccountQuota = "" })

	// A fresh service sees what was saved.
	fresh := NewSMSService(f.svc.settings, f.svc.store, f.sender, nil)
	ops, err = fresh.AdminOpsAlert(ctx)
	require.NoError(t, err)
	require.Equal(t, SMSOpsAlertView{SMSOpsAlert: SMSOpsAlert{Enabled: true, Phones: []string{"13900000001"}, MinLevel: "critical"}, Ready: true}, ops)
	quota, err = fresh.AdminAccountQuota(ctx)
	require.NoError(t, err)
	require.False(t, quota.Ready, "no account quota template")
}

func TestSMSFirstReleaseAdminPhonesAreKept(t *testing.T) {
	f := newSMSFixture(t)
	ctx := context.Background()
	// The SMS card of mok-v0.2.23.2 kept one phone list for both admin alerts.
	require.NoError(t, f.svc.settings.Set(ctx, smsConfigKey, `{"enabled":true,"dry_run":true,"region":"ap-guangzhou","daily_limit":10,`+
		`"templates":{"verify_code":"100","ops_alert":"400","account_quota":"500"},`+
		`"events":{"balance_low":true,"subscription_expiry":true,"ops_alert":true,"account_quota":true},`+
		`"admin_phones":["13900000001","13900000002"],"ops_alert_min_level":"warning"}`))

	ops, err := f.svc.AdminOpsAlert(ctx)
	require.NoError(t, err)
	require.Equal(t, SMSOpsAlertView{SMSOpsAlert: SMSOpsAlert{Enabled: true, Phones: []string{"13900000001", "13900000002"}, MinLevel: "warning"}, Ready: true}, ops)
	require.True(t, f.svc.MokSMSWanted(ctx, service.MokSMSEventAccountQuota))

	// Saving the SMS card rewrites its key without those fields; the phones stay.
	cfg := readySMSConfig()
	cfg.DryRun = true
	_, err = f.svc.SaveConfig(ctx, cfg)
	require.NoError(t, err)
	raw, err := f.svc.settings.GetValue(ctx, smsConfigKey)
	require.NoError(t, err)
	require.NotContains(t, raw, "admin_phones")

	fresh := NewSMSService(f.svc.settings, f.svc.store, f.sender, nil)
	ops, err = fresh.AdminOpsAlert(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"13900000001", "13900000002"}, ops.Phones)
	require.Equal(t, "warning", ops.MinLevel)
	quota, err := fresh.AdminAccountQuota(ctx)
	require.NoError(t, err)
	require.Equal(t, []SMSQuotaPhone{{Phone: "13900000001"}, {Phone: "13900000002"}}, quota.Phones)
}

func TestSMSSubscriptionExpiryReminders(t *testing.T) {
	f := newSMSFixture(t)
	f.configure(t, nil)
	f.bind(t, 1, "13800138000")
	f.bind(t, 2, "13800138002")
	group := &service.Group{Name: "Claude Pro"}
	f.subs.subs = []service.UserSubscription{
		{ID: 11, UserID: 1, GroupID: 5, Group: group, ExpiresAt: f.clock.Add(23 * time.Hour)},     // 1 day
		{ID: 12, UserID: 3, GroupID: 5, Group: group, ExpiresAt: f.clock.Add(2*24*time.Hour + 1)}, // 3 days, no phone
		{ID: 13, UserID: 2, GroupID: 6, ExpiresAt: f.clock.Add(3*24*time.Hour - time.Minute)},     // 3 days
		{ID: 14, UserID: 1, GroupID: 5, Group: group, ExpiresAt: f.clock.Add(5 * 24 * time.Hour)}, // 5 days
		{ID: 15, UserID: 2, GroupID: 5, Group: group, ExpiresAt: f.clock.Add(9 * 24 * time.Hour)}, // past 7 days
	}

	f.svc.runOnce()
	f.svc.runOnce()

	msgs := f.sender.messages()[2:] // after the two codes
	require.Equal(t, []sentSMS{
		{phone: "13800138000", template: "300", params: []string{"Claude Pro", "2026-11-19 09:00", "1"}},
		{phone: "13800138002", template: "300", params: []string{"#6", "2026-11-21 09:59", "3"}},
	}, msgs)

	f.configure(t, func(c *SMSConfig) { c.Events.SubscriptionExpiry = false })
	f.advance(24 * time.Hour) // subscription 13 enters... nothing, the reminder is off
	f.svc.runOnce()
	require.Len(t, f.sender.messages(), 4)
}

func TestSMSTestSendAndLogs(t *testing.T) {
	f := newSMSFixture(t)
	ctx := context.Background()

	_, err := f.svc.TestSend(ctx, "13800138000", service.MokSMSEventBalanceLow)
	requireReason(t, err, "MOK_SMS_NOT_CONFIGURED")

	f.configure(t, func(c *SMSConfig) { c.Enabled = false })
	_, err = f.svc.TestSend(ctx, "13800138000", "nope")
	requireReason(t, err, "MOK_SMS_EVENT_INVALID")
	result, err := f.svc.TestSend(ctx, "13800138000", service.MokSMSEventSubscriptionExpiry)
	require.NoError(t, err, "testing works before SMS is switched on")
	require.Equal(t, SMSTestResult{Status: smsStatusSent}, result)
	require.Equal(t, sentSMS{phone: "13800138000", template: "300", params: []string{"测试订阅", "2026-11-25 10:00", "7"}}, f.sender.messages()[0])

	f.sender.fail = errors.New("FailedOperation.TemplateParamSetNotMatchApprovedTemplate: params")
	result, err = f.svc.TestSend(ctx, "13900139000", service.MokSMSEventOpsAlert)
	require.NoError(t, err)
	require.Equal(t, smsStatusFailed, result.Status)
	require.Contains(t, result.Error, "TemplateParamSetNotMatchApprovedTemplate")

	entries, total, err := f.svc.ListLogs(ctx, 1, 10, "")
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Equal(t, "13900139000", entries[0].Phone, "newest first")
	require.Equal(t, smsEventTest, entries[0].Event)
	require.Equal(t, "400", entries[0].TemplateID)
	require.Equal(t, []string{"测试告警", "P1", "95", "90"}, entries[0].Params)
	require.NotNil(t, entries[0].FinishedAt)
	require.Nil(t, entries[0].UserID)

	entries, total, err = f.svc.ListLogs(ctx, 1, 10, "8000")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "13800138000", entries[0].Phone)

	entries, total, err = f.svc.ListLogs(ctx, 1, 10, "100%_")
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, entries)
}

func TestSMSPruneDropsOldLogsAndExpiredCodes(t *testing.T) {
	f := newSMSFixture(t)
	f.configure(t, nil)
	ctx := context.Background()
	_, err := f.svc.SendBindCode(ctx, 1, "13800138000")
	require.NoError(t, err)

	f.advance(91 * 24 * time.Hour)
	f.svc.runOnce()

	var logs, codes int
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM mok_sms_logs`).Scan(&logs))
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM mok_sms_codes`).Scan(&codes))
	require.Zero(t, logs)
	require.Zero(t, codes)
}
