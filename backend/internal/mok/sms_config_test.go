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
	cfg.AdminPhones = []string{"13800138000"}
	return cfg
}

func TestSMSConfigWants(t *testing.T) {
	cfg := readyConfig()
	for _, event := range []string{service.MokSMSEventBalanceLow, service.MokSMSEventSubscriptionExpiry, service.MokSMSEventOpsAlert, service.MokSMSEventAccountQuota} {
		require.True(t, cfg.wants(event), event)
	}
	require.False(t, cfg.wants(smsEventVerifyCode), "not a notification")
	require.True(t, cfg.bindingAvailable())

	off := cfg
	off.Enabled = false
	require.False(t, off.wants(service.MokSMSEventBalanceLow))
	require.False(t, off.bindingAvailable())

	noCredentials := cfg
	noCredentials.SecretKey = ""
	require.False(t, noCredentials.wants(service.MokSMSEventBalanceLow))
	noCredentials.DryRun = true
	require.True(t, noCredentials.wants(service.MokSMSEventBalanceLow), "dry-run mode needs no credentials")

	switchedOff := cfg
	switchedOff.Events.SubscriptionExpiry = false
	require.False(t, switchedOff.wants(service.MokSMSEventSubscriptionExpiry))

	noTemplate := cfg
	noTemplate.Templates.OpsAlert = ""
	require.False(t, noTemplate.wants(service.MokSMSEventOpsAlert))

	noAdmins := cfg
	noAdmins.AdminPhones = nil
	require.False(t, noAdmins.wants(service.MokSMSEventOpsAlert))
	require.False(t, noAdmins.wants(service.MokSMSEventAccountQuota))
	require.True(t, noAdmins.wants(service.MokSMSEventBalanceLow))

	noVerify := cfg
	noVerify.Templates.VerifyCode = ""
	require.False(t, noVerify.bindingAvailable(), "users cannot bind without codes")

	adminOnly := cfg
	adminOnly.Events.BalanceLow, adminOnly.Events.SubscriptionExpiry = false, false
	require.False(t, adminOnly.bindingAvailable(), "nothing would reach a user's phone")
}

func TestParseSMSConfigDefaults(t *testing.T) {
	cfg := parseSMSConfig("")
	require.False(t, cfg.Enabled)
	require.Equal(t, smsDefaultRegion, cfg.Region)
	require.Equal(t, 10, cfg.DailyLimit)
	require.Equal(t, SMSEvents{BalanceLow: true, SubscriptionExpiry: true, OpsAlert: true, AccountQuota: true}, cfg.Events)

	require.Equal(t, defaultSMSConfig(), parseSMSConfig("{broken"))

	stored := parseSMSConfig(`{"enabled":true,"events":{"balance_low":false},"region":"nowhere"}`)
	require.True(t, stored.Enabled)
	require.False(t, stored.Events.BalanceLow)
	require.True(t, stored.Events.OpsAlert, "a switch missing from the stored JSON keeps its default")
	require.Equal(t, smsDefaultRegion, stored.Region)
}

func TestNormalizeSMSConfig(t *testing.T) {
	stored := readyConfig()
	in := readyConfig()
	in.SecretKey = ""
	in.SecretID = "  id2 "
	in.AdminPhones = []string{"+86 138 0013 8000", "13800138000", "", "13900139000"}
	in.Templates.BalanceLow = " 22 "
	in.OpsAlertMinLevel = "Warning"
	in.Region = ""

	out, err := normalizeSMSConfig(in, stored)
	require.NoError(t, err)
	require.Equal(t, "key", out.SecretKey, "an empty key keeps the stored one")
	require.Equal(t, "id2", out.SecretID)
	require.Equal(t, []string{"13800138000", "13900139000"}, out.AdminPhones)
	require.Equal(t, "22", out.Templates.BalanceLow)
	require.Equal(t, "warning", out.OpsAlertMinLevel)
	require.Equal(t, smsDefaultRegion, out.Region)

	for name, change := range map[string]func(*SMSConfig){
		"admin phone": func(c *SMSConfig) { c.AdminPhones = []string{"12345"} },
		"template":    func(c *SMSConfig) { c.Templates.VerifyCode = "SMS_123" },
		"app id":      func(c *SMSConfig) { c.SDKAppID = "app" },
		"region":      func(c *SMSConfig) { c.Region = "us-east-1" },
		"daily limit": func(c *SMSConfig) { c.DailyLimit = -1 },
		"level":       func(c *SMSConfig) { c.OpsAlertMinLevel = "fatal" },
		"signature":   func(c *SMSConfig) { c.SignName = strings.Repeat("签", 33) },
		"too many": func(c *SMSConfig) {
			c.AdminPhones = nil
			for i := 0; i < 11; i++ {
				c.AdminPhones = append(c.AdminPhones, fmt.Sprintf("138001380%02d", i))
			}
		},
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
