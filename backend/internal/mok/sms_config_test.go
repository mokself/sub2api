//go:build unit

package mok

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestNormalizeSMSPhone(t *testing.T) {
	for raw, want := range map[string]string{
		"13800138000":       "13800138000",
		" 138 0013 8000 ":   "13800138000",
		"138-0013-8000":     "13800138000",
		"+8613800138000":    "13800138000",
		"+86 138 0013 8000": "13800138000",
		"008613800138000":   "13800138000",
		"8613800138000":     "13800138000",
		"19912345678":       "19912345678",
	} {
		got, ok := normalizeSMSPhone(raw)
		require.True(t, ok, raw)
		require.Equal(t, want, got, raw)
	}
	for _, raw := range []string{"", "1380013800", "138001380001", "12800138000", "+12025550123", "abc", "+85291234567"} {
		_, ok := normalizeSMSPhone(raw)
		require.False(t, ok, raw)
	}
	require.Equal(t, "138****8000", maskSMSPhone("13800138000"))
}

func readyConfig() SMSConfig {
	cfg := defaultSMSConfig()
	cfg.Enabled = true
	cfg.SecretID, cfg.SecretKey, cfg.SDKAppID, cfg.SignName = "id", "key", "1400000000", "测试站"
	cfg.Templates = SMSTemplates{VerifyCode: "1", BalanceLow: "2", SubscriptionExpiry: "3", OpsAlert: "4", AccountQuota: "5"}
	return cfg
}

func readySettings() smsSettings {
	return smsSettings{
		cfg:   readyConfig(),
		ops:   SMSOpsAlert{Enabled: true, Phones: []string{"13800138000"}},
		quota: SMSAccountQuota{Phones: []SMSQuotaPhone{{Phone: "13900139000"}, {Phone: "13900139001", Disabled: true}}},
	}
}

func TestSMSSettingsWants(t *testing.T) {
	s := readySettings()
	for _, event := range []string{service.MokSMSEventBalanceLow, service.MokSMSEventSubscriptionExpiry, service.MokSMSEventOpsAlert, service.MokSMSEventAccountQuota} {
		require.True(t, s.wants(event), event)
	}
	require.False(t, s.wants(smsEventVerifyCode), "not a notification")
	require.True(t, s.bindingAvailable())
	require.Equal(t, []string{"13800138000"}, s.recipients(service.MokSMSEventOpsAlert))
	require.Equal(t, []string{"13900139000"}, s.recipients(service.MokSMSEventAccountQuota), "switched-off phones get nothing")
	require.Nil(t, s.recipients(service.MokSMSEventBalanceLow))

	off := readySettings()
	off.cfg.Enabled = false
	require.False(t, off.wants(service.MokSMSEventBalanceLow))
	require.False(t, off.wants(service.MokSMSEventOpsAlert))
	require.False(t, off.bindingAvailable())

	noCredentials := readySettings()
	noCredentials.cfg.SecretKey = ""
	require.False(t, noCredentials.wants(service.MokSMSEventBalanceLow))
	noCredentials.cfg.DryRun = true
	require.True(t, noCredentials.wants(service.MokSMSEventBalanceLow), "dry-run mode needs no credentials")

	switchedOff := readySettings()
	switchedOff.cfg.Events.SubscriptionExpiry = false
	require.False(t, switchedOff.wants(service.MokSMSEventSubscriptionExpiry))

	noTemplate := readySettings()
	noTemplate.cfg.Templates.OpsAlert = ""
	require.False(t, noTemplate.wants(service.MokSMSEventOpsAlert))
	require.False(t, noTemplate.cfg.ready(service.MokSMSEventOpsAlert))

	opsOff := readySettings()
	opsOff.ops.Enabled = false
	require.False(t, opsOff.wants(service.MokSMSEventOpsAlert))
	require.True(t, opsOff.wants(service.MokSMSEventAccountQuota), "the two admin alerts are separate")

	noQuotaPhones := readySettings()
	noQuotaPhones.quota.Phones = []SMSQuotaPhone{{Phone: "13900139000", Disabled: true}}
	require.False(t, noQuotaPhones.wants(service.MokSMSEventAccountQuota))
	require.True(t, noQuotaPhones.wants(service.MokSMSEventOpsAlert))

	noVerify := readySettings()
	noVerify.cfg.Templates.VerifyCode = ""
	require.False(t, noVerify.bindingAvailable(), "users cannot bind without codes")

	adminOnly := readySettings()
	adminOnly.cfg.Events.BalanceLow, adminOnly.cfg.Events.SubscriptionExpiry = false, false
	require.False(t, adminOnly.bindingAvailable(), "nothing would reach a user's phone")
}

func TestParseSMSConfigDefaults(t *testing.T) {
	cfg := parseSMSConfig("")
	require.False(t, cfg.Enabled)
	require.Equal(t, smsDefaultRegion, cfg.Region)
	require.Equal(t, 10, cfg.DailyLimit)
	require.Equal(t, SMSEvents{BalanceLow: true, SubscriptionExpiry: true}, cfg.Events)

	require.Equal(t, defaultSMSConfig(), parseSMSConfig("{broken"))

	stored := parseSMSConfig(`{"enabled":true,"events":{"balance_low":false},"region":"nowhere"}`)
	require.True(t, stored.Enabled)
	require.False(t, stored.Events.BalanceLow)
	require.True(t, stored.Events.SubscriptionExpiry, "a switch missing from the stored JSON keeps its default")
	require.Equal(t, smsDefaultRegion, stored.Region)
}

func TestSMSLegacyAdminAlerts(t *testing.T) {
	// What the first release stored: one phone list for both admin alerts.
	legacy := parseSMSLegacyAdminAlerts(`{"enabled":true,"admin_phones":["13800138000","bad","13900139000"],"ops_alert_min_level":"Warning","events":{"balance_low":true,"ops_alert":true}}`)
	require.Equal(t, SMSOpsAlert{Enabled: true, Phones: []string{"13800138000", "13900139000"}, MinLevel: "warning"}, legacy.opsAlert(),
		"an invalid number is dropped rather than losing the list")
	require.Equal(t, SMSAccountQuota{Phones: []SMSQuotaPhone{{Phone: "13800138000"}, {Phone: "13900139000"}}}, legacy.accountQuota(),
		"a switch missing from the stored JSON was on")

	switchedOff := parseSMSLegacyAdminAlerts(`{"admin_phones":["13800138000"],"events":{"ops_alert":false,"account_quota":false}}`)
	require.False(t, switchedOff.opsAlert().Enabled)
	require.Equal(t, []string{"13800138000"}, switchedOff.opsAlert().Phones)
	require.Empty(t, switchedOff.accountQuota().Phones)

	none := parseSMSLegacyAdminAlerts("")
	require.Equal(t, SMSOpsAlert{Phones: []string{}}, none.opsAlert())
	require.Empty(t, none.accountQuota().Phones)
}

func TestNormalizeSMSConfig(t *testing.T) {
	stored := readyConfig()
	in := readyConfig()
	in.SecretKey = ""
	in.SecretID = "  id2 "
	in.Templates.BalanceLow = " 22 "
	in.Region = ""

	out, err := normalizeSMSConfig(in, stored)
	require.NoError(t, err)
	require.Equal(t, "key", out.SecretKey, "an empty key keeps the stored one")
	require.Equal(t, "id2", out.SecretID)
	require.Equal(t, "22", out.Templates.BalanceLow)
	require.Equal(t, smsDefaultRegion, out.Region)

	for name, change := range map[string]func(*SMSConfig){
		"template":      func(c *SMSConfig) { c.Templates.VerifyCode = "SMS_123" },
		"app id":        func(c *SMSConfig) { c.SDKAppID = "app" },
		"region":        func(c *SMSConfig) { c.Region = "us-east-1" },
		"daily limit":   func(c *SMSConfig) { c.DailyLimit = -1 },
		"signature":     func(c *SMSConfig) { c.SignName = strings.Repeat("签", 33) },
		"no credential": func(c *SMSConfig) { c.SDKAppID = "" },
	} {
		cfg := readyConfig()
		change(&cfg)
		_, err := normalizeSMSConfig(cfg, stored)
		require.Error(t, err, name)
	}

	dryRun := readyConfig()
	dryRun.SecretID, dryRun.SecretKey, dryRun.SDKAppID, dryRun.SignName = "", "", "", ""
	dryRun.DryRun = true
	_, err = normalizeSMSConfig(dryRun, SMSConfig{})
	require.NoError(t, err, "dry-run mode can be switched on without credentials")
}

func TestNormalizeSMSAdminAlerts(t *testing.T) {
	ops, err := normalizeSMSOpsAlert(SMSOpsAlert{Enabled: true, Phones: []string{"+86 138 0013 8000", "13800138000", "", "13900139000"}, MinLevel: " Critical "})
	require.NoError(t, err)
	require.Equal(t, SMSOpsAlert{Enabled: true, Phones: []string{"13800138000", "13900139000"}, MinLevel: "critical"}, ops)

	ops, err = normalizeSMSOpsAlert(SMSOpsAlert{Enabled: true})
	require.NoError(t, err)
	require.False(t, ops.Enabled, "no phone switches the SMS alert off, as upstream does for the email")

	tooMany := []string{}
	for i := 0; i < 11; i++ {
		tooMany = append(tooMany, fmt.Sprintf("138001380%02d", i))
	}
	for name, in := range map[string]SMSOpsAlert{
		"phone":    {Phones: []string{"12345"}},
		"level":    {MinLevel: "fatal"},
		"too many": {Phones: tooMany},
	} {
		_, err := normalizeSMSOpsAlert(in)
		require.Error(t, err, name)
	}

	quota, err := normalizeSMSAccountQuota(SMSAccountQuota{Phones: []SMSQuotaPhone{
		{Phone: " 139 0013 9000 ", Disabled: true}, {Phone: ""}, {Phone: "13900139000"}, {Phone: "+8613800138000"},
	}})
	require.NoError(t, err)
	require.Equal(t, []SMSQuotaPhone{{Phone: "13900139000", Disabled: true}, {Phone: "13800138000"}}, quota.Phones,
		"empty rows are dropped and a repeated number keeps its first entry")

	_, err = normalizeSMSAccountQuota(SMSAccountQuota{Phones: []SMSQuotaPhone{{Phone: "12345"}}})
	require.Error(t, err)
	quota, err = normalizeSMSAccountQuota(SMSAccountQuota{})
	require.NoError(t, err)
	require.Equal(t, []SMSQuotaPhone{}, quota.Phones)
}

func TestSMSConfigViewHidesTheSecretKey(t *testing.T) {
	view := viewSMSConfig(readyConfig())
	raw, err := json.Marshal(view)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"key"`)
	require.Contains(t, string(raw), `"secret_key":""`)
	require.Contains(t, string(raw), `"secret_key_set":true`)
}

func newTestTencentServer(t *testing.T, reply string) (*tencentSMSSender, *map[string]any) {
	t.Helper()
	got := map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "SendSms", r.Header.Get("X-TC-Action"))
		require.Equal(t, "2021-01-11", r.Header.Get("X-TC-Version"))
		require.Equal(t, "ap-guangzhou", r.Header.Get("X-TC-Region"))
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &got))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	return &tencentSMSSender{endpoint: strings.TrimPrefix(srv.URL, "http://"), scheme: "HTTP"}, &got
}

func TestTencentSMSSenderSends(t *testing.T) {
	sender, got := newTestTencentServer(t, `{"Response":{"SendStatusSet":[{"PhoneNumber":"+8613800138000","Code":"Ok","Message":"send success"}],"RequestId":"req-1"}}`)

	requestID, err := sender.Send(context.Background(), readyConfig(), "13800138000", "2", []string{"8.50", "10.00"})

	require.NoError(t, err)
	require.Equal(t, "req-1", requestID)
	require.Equal(t, []any{"+8613800138000"}, (*got)["PhoneNumberSet"])
	require.Equal(t, "1400000000", (*got)["SmsSdkAppId"])
	require.Equal(t, "测试站", (*got)["SignName"])
	require.Equal(t, "2", (*got)["TemplateId"])
	require.Equal(t, []any{"8.50", "10.00"}, (*got)["TemplateParamSet"])
}

func TestTencentSMSSenderReportsAFailedStatus(t *testing.T) {
	sender, _ := newTestTencentServer(t, `{"Response":{"SendStatusSet":[{"Code":"LimitExceeded.PhoneNumberDailyLimit","Message":"daily limit"}],"RequestId":"req-2"}}`)

	requestID, err := sender.Send(context.Background(), readyConfig(), "13800138000", "2", nil)

	require.EqualError(t, err, "LimitExceeded.PhoneNumberDailyLimit: daily limit")
	require.Equal(t, "req-2", requestID)
}

func TestTencentSMSSenderReportsAnAPIError(t *testing.T) {
	sender, _ := newTestTencentServer(t, `{"Response":{"Error":{"Code":"AuthFailure.SecretIdNotFound","Message":"bad id"},"RequestId":"req-3"}}`)

	requestID, err := sender.Send(context.Background(), readyConfig(), "13800138000", "2", nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "AuthFailure.SecretIdNotFound")
	require.Equal(t, "req-3", requestID)
}
