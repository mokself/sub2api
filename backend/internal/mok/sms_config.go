package mok

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// The SMS settings live in three settings keys, as JSON, each edited where its email
// counterpart is:
//   - smsConfigKey: the SMS card in the system settings (provider, templates, the user
//     notifications, the daily cap);
//   - smsOpsAlertKey: ops monitoring → settings, next to the alert emails;
//   - smsAccountQuotaKey: the account quota notification card, next to its emails.
const (
	smsConfigKey       = "mok_sms_config"
	smsOpsAlertKey     = "mok_sms_ops_alert"
	smsAccountQuotaKey = "mok_sms_account_quota"
)

// Events that are not notifications: binding a phone number, and the admin's test message.
const (
	smsEventVerifyCode = "verify_code"
	smsEventTest       = "test"
)

const (
	smsDefaultRegion     = "ap-guangzhou"
	smsDefaultDailyLimit = 10
	smsMaxDailyLimit     = 1000
	smsMaxAdminPhones    = 10
	smsMaxSignNameLength = 32
	smsMaxSecretLength   = 128
)

// smsRegions are the regions Tencent Cloud serves the SMS API from.
var smsRegions = map[string]bool{"ap-guangzhou": true, "ap-beijing": true, "ap-nanjing": true}

// smsMinLevels are the accepted minimum ops alert levels; empty sends every level.
var smsMinLevels = map[string]bool{"": true, "info": true, "warning": true, "critical": true}

var smsDigits = regexp.MustCompile(`^\d+$`)

// SMSTemplates holds a Tencent Cloud template ID per message kind.
type SMSTemplates struct {
	VerifyCode         string `json:"verify_code"`
	BalanceLow         string `json:"balance_low"`
	SubscriptionExpiry string `json:"subscription_expiry"`
	OpsAlert           string `json:"ops_alert"`
	AccountQuota       string `json:"account_quota"`
}

// SMSEvents switches the user notifications on and off. The admin alerts are switched
// where their recipients are set (SMSOpsAlert, SMSAccountQuota).
type SMSEvents struct {
	BalanceLow         bool `json:"balance_low"`
	SubscriptionExpiry bool `json:"subscription_expiry"`
}

// SMSConfig is the configuration of the SMS card.
type SMSConfig struct {
	Enabled bool `json:"enabled"`
	// DryRun logs every message instead of sending it, for trying the setup out.
	DryRun    bool   `json:"dry_run"`
	SecretID  string `json:"secret_id"`
	SecretKey string `json:"secret_key"`
	SDKAppID  string `json:"sdk_app_id"`
	SignName  string `json:"sign_name"`
	Region    string `json:"region"`
	// DailyLimit caps the notifications a user (for admin alerts: a phone) gets a day;
	// verification codes and test messages do not count. 0 means no cap.
	DailyLimit int          `json:"daily_limit"`
	Templates  SMSTemplates `json:"templates"`
	Events     SMSEvents    `json:"events"`
}

// SMSOpsAlert is the SMS part of the ops alert settings.
type SMSOpsAlert struct {
	Enabled bool     `json:"enabled"`
	Phones  []string `json:"phones"`
	// MinLevel is the lowest alert level sent: info, warning, critical, or empty for all.
	MinLevel string `json:"min_level"`
}

// SMSQuotaPhone is one admin phone of the account quota alerts.
type SMSQuotaPhone struct {
	Phone    string `json:"phone"`
	Disabled bool   `json:"disabled"`
}

// SMSAccountQuota is the SMS part of the account quota notification settings.
type SMSAccountQuota struct {
	Phones []SMSQuotaPhone `json:"phones"`
}

// defaultSMSConfig is what an unconfigured site starts from: off, with both user
// notifications switched on so that filling in the templates is all it takes.
func defaultSMSConfig() SMSConfig {
	return SMSConfig{
		Region:     smsDefaultRegion,
		DailyLimit: smsDefaultDailyLimit,
		Events:     SMSEvents{BalanceLow: true, SubscriptionExpiry: true},
	}
}

// parseSMSConfig reads the stored JSON over the defaults.
func parseSMSConfig(raw string) SMSConfig {
	cfg := defaultSMSConfig()
	if strings.TrimSpace(raw) == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return defaultSMSConfig()
	}
	if !smsRegions[cfg.Region] {
		cfg.Region = smsDefaultRegion
	}
	return cfg
}

// smsLegacyAdminAlerts are the admin alert fields the SMS card held in its first release
// (mok-v0.2.23.2): one phone list for both alerts. They are read from smsConfigKey while
// the new keys have not been saved yet.
type smsLegacyAdminAlerts struct {
	AdminPhones      []string `json:"admin_phones"`
	OpsAlertMinLevel string   `json:"ops_alert_min_level"`
	Events           struct {
		OpsAlert     *bool `json:"ops_alert"`
		AccountQuota *bool `json:"account_quota"`
	} `json:"events"`
}

func parseSMSLegacyAdminAlerts(raw string) smsLegacyAdminAlerts {
	var legacy smsLegacyAdminAlerts
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &legacy)
	}
	return legacy
}

// switchedOn reads a legacy switch, which defaulted to on.
func switchedOn(v *bool) bool { return v == nil || *v }

// phones returns the valid legacy phones; anything else is dropped rather than losing the list.
func (l smsLegacyAdminAlerts) phones() []string {
	phones := []string{}
	seen := map[string]bool{}
	for _, entry := range l.AdminPhones {
		phone, ok := normalizeSMSPhone(entry)
		if ok && !seen[phone] && len(phones) < smsMaxAdminPhones {
			seen[phone] = true
			phones = append(phones, phone)
		}
	}
	return phones
}

func (l smsLegacyAdminAlerts) opsAlert() SMSOpsAlert {
	phones := l.phones()
	level := strings.ToLower(strings.TrimSpace(l.OpsAlertMinLevel))
	if !smsMinLevels[level] {
		level = ""
	}
	return SMSOpsAlert{Enabled: switchedOn(l.Events.OpsAlert) && len(phones) > 0, Phones: phones, MinLevel: level}
}

func (l smsLegacyAdminAlerts) accountQuota() SMSAccountQuota {
	quota := SMSAccountQuota{Phones: []SMSQuotaPhone{}}
	if !switchedOn(l.Events.AccountQuota) {
		return quota
	}
	for _, phone := range l.phones() {
		quota.Phones = append(quota.Phones, SMSQuotaPhone{Phone: phone})
	}
	return quota
}

func parseSMSOpsAlert(raw string) (SMSOpsAlert, bool) {
	var ops SMSOpsAlert
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &ops) != nil {
		return SMSOpsAlert{}, false
	}
	normalized, err := normalizeSMSOpsAlert(ops)
	if err != nil {
		return SMSOpsAlert{}, false
	}
	return normalized, true
}

func parseSMSAccountQuota(raw string) (SMSAccountQuota, bool) {
	var quota SMSAccountQuota
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &quota) != nil {
		return SMSAccountQuota{}, false
	}
	normalized, err := normalizeSMSAccountQuota(quota)
	if err != nil {
		return SMSAccountQuota{}, false
	}
	return normalized, true
}

// smsSettings is everything a notification decision needs.
type smsSettings struct {
	cfg   SMSConfig
	ops   SMSOpsAlert
	quota SMSAccountQuota
}

// credentialsSet reports whether the provider can be called.
func (c SMSConfig) credentialsSet() bool {
	return c.SecretID != "" && c.SecretKey != "" && c.SDKAppID != "" && c.SignName != ""
}

// canSend reports whether messages can go out at all: on, and either able to reach the
// provider or only logging.
func (c SMSConfig) canSend() bool {
	return c.Enabled && (c.DryRun || c.credentialsSet())
}

// templateFor returns the template ID of a message kind.
func (c SMSConfig) templateFor(event string) string {
	switch event {
	case smsEventVerifyCode:
		return c.Templates.VerifyCode
	case service.MokSMSEventBalanceLow:
		return c.Templates.BalanceLow
	case service.MokSMSEventSubscriptionExpiry:
		return c.Templates.SubscriptionExpiry
	case service.MokSMSEventOpsAlert:
		return c.Templates.OpsAlert
	case service.MokSMSEventAccountQuota:
		return c.Templates.AccountQuota
	}
	return ""
}

// ready reports whether messages of a kind can be sent: SMS on and its template set.
func (c SMSConfig) ready(event string) bool {
	return c.canSend() && c.templateFor(event) != ""
}

// enabledPhones are the account quota phones that are switched on.
func (q SMSAccountQuota) enabledPhones() []string {
	phones := []string{}
	for _, p := range q.Phones {
		if !p.Disabled {
			phones = append(phones, p.Phone)
		}
	}
	return phones
}

// recipients returns the admin phones of an admin alert; nil for user notifications.
func (s smsSettings) recipients(event string) []string {
	switch event {
	case service.MokSMSEventOpsAlert:
		if !s.ops.Enabled {
			return nil
		}
		return s.ops.Phones
	case service.MokSMSEventAccountQuota:
		return s.quota.enabledPhones()
	}
	return nil
}

// wants reports whether a notification goes out by SMS. Admin alerts also need a phone.
func (s smsSettings) wants(event string) bool {
	if !s.cfg.ready(event) {
		return false
	}
	switch event {
	case service.MokSMSEventBalanceLow:
		return s.cfg.Events.BalanceLow
	case service.MokSMSEventSubscriptionExpiry:
		return s.cfg.Events.SubscriptionExpiry
	case service.MokSMSEventOpsAlert, service.MokSMSEventAccountQuota:
		return len(s.recipients(event)) > 0
	}
	return false
}

// userEvents are the notifications users get on their bound phone.
var userEvents = []string{service.MokSMSEventBalanceLow, service.MokSMSEventSubscriptionExpiry}

// bindingAvailable reports whether users can bind a phone: codes can be sent and at least
// one notification would reach that phone.
func (s smsSettings) bindingAvailable() bool {
	if !s.cfg.ready(smsEventVerifyCode) {
		return false
	}
	for _, event := range userEvents {
		if s.wants(event) {
			return true
		}
	}
	return false
}

// SMSConfigView is the configuration as the admin page sees it: the secret key is never
// sent back, only whether one is stored.
type SMSConfigView struct {
	SMSConfig
	SecretKey    string `json:"secret_key"`
	SecretKeySet bool   `json:"secret_key_set"`
}

func viewSMSConfig(c SMSConfig) SMSConfigView {
	return SMSConfigView{SMSConfig: c, SecretKey: "", SecretKeySet: c.SecretKey != ""}
}

// SMSOpsAlertView adds whether the SMS card is set up for ops alerts.
type SMSOpsAlertView struct {
	SMSOpsAlert
	// Ready is false until SMS is switched on and the ops alert template is set.
	Ready bool `json:"ready"`
}

// SMSAccountQuotaView adds whether the SMS card is set up for account quota alerts.
type SMSAccountQuotaView struct {
	SMSAccountQuota
	// Ready is false until SMS is switched on and the account quota template is set.
	Ready bool `json:"ready"`
}

func smsConfigError(message string) error {
	return infraerrors.BadRequest("MOK_SMS_CONFIG_INVALID", message)
}

// normalizeSMSConfig validates an update from the SMS card. An empty secret key keeps the
// stored one.
func normalizeSMSConfig(in SMSConfig, stored SMSConfig) (SMSConfig, error) {
	out := in
	out.SecretID = strings.TrimSpace(in.SecretID)
	out.SecretKey = strings.TrimSpace(in.SecretKey)
	if out.SecretKey == "" {
		out.SecretKey = stored.SecretKey
	}
	out.SDKAppID = strings.TrimSpace(in.SDKAppID)
	out.SignName = strings.TrimSpace(in.SignName)
	out.Region = strings.TrimSpace(in.Region)
	if out.Region == "" {
		out.Region = smsDefaultRegion
	}

	if len(out.SecretID) > smsMaxSecretLength || len(out.SecretKey) > smsMaxSecretLength {
		return SMSConfig{}, smsConfigError("SecretId or SecretKey is too long")
	}
	if out.SDKAppID != "" && !smsDigits.MatchString(out.SDKAppID) {
		return SMSConfig{}, smsConfigError("the SDK AppID is a number such as 1400000000")
	}
	if utf8.RuneCountInString(out.SignName) > smsMaxSignNameLength {
		return SMSConfig{}, smsConfigError("the signature is too long")
	}
	if !smsRegions[out.Region] {
		return SMSConfig{}, smsConfigError("unsupported region")
	}
	if out.DailyLimit < 0 || out.DailyLimit > smsMaxDailyLimit {
		return SMSConfig{}, smsConfigError("the daily cap must be between 0 and 1000")
	}
	for _, id := range []*string{&out.Templates.VerifyCode, &out.Templates.BalanceLow, &out.Templates.SubscriptionExpiry, &out.Templates.OpsAlert, &out.Templates.AccountQuota} {
		*id = strings.TrimSpace(*id)
		if *id != "" && (!smsDigits.MatchString(*id) || len(*id) > 20) {
			return SMSConfig{}, smsConfigError("a template ID is a number such as 1234567")
		}
	}
	if out.Enabled && !out.DryRun && !out.credentialsSet() {
		return SMSConfig{}, smsConfigError("fill in SecretId, SecretKey, SDK AppID and the signature before switching SMS on")
	}
	return out, nil
}

// normalizeSMSPhoneList normalizes, deduplicates and checks a list of admin phones. Empty
// entries are skipped; the first invalid entry is returned with the error.
func normalizeSMSPhoneList(raw []string) ([]string, error) {
	phones := []string{}
	seen := map[string]bool{}
	for _, entry := range raw {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		phone, ok := normalizeSMSPhone(entry)
		if !ok {
			return nil, smsConfigError("invalid phone number: " + strings.TrimSpace(entry))
		}
		if !seen[phone] {
			seen[phone] = true
			phones = append(phones, phone)
		}
	}
	if len(phones) > smsMaxAdminPhones {
		return nil, smsConfigError("at most 10 phone numbers")
	}
	return phones, nil
}

// normalizeSMSOpsAlert validates the ops alert settings. Without a phone the alert is off,
// as upstream switches the alert email off without a recipient.
func normalizeSMSOpsAlert(in SMSOpsAlert) (SMSOpsAlert, error) {
	phones, err := normalizeSMSPhoneList(in.Phones)
	if err != nil {
		return SMSOpsAlert{}, err
	}
	level := strings.ToLower(strings.TrimSpace(in.MinLevel))
	if !smsMinLevels[level] {
		return SMSOpsAlert{}, smsConfigError("unsupported ops alert level")
	}
	return SMSOpsAlert{Enabled: in.Enabled && len(phones) > 0, Phones: phones, MinLevel: level}, nil
}

// normalizeSMSAccountQuota validates the account quota phones; a number listed twice keeps
// its first entry.
func normalizeSMSAccountQuota(in SMSAccountQuota) (SMSAccountQuota, error) {
	out := SMSAccountQuota{Phones: []SMSQuotaPhone{}}
	seen := map[string]bool{}
	for _, entry := range in.Phones {
		if strings.TrimSpace(entry.Phone) == "" {
			continue
		}
		phone, ok := normalizeSMSPhone(entry.Phone)
		if !ok {
			return SMSAccountQuota{}, smsConfigError("invalid phone number: " + strings.TrimSpace(entry.Phone))
		}
		if !seen[phone] {
			seen[phone] = true
			out.Phones = append(out.Phones, SMSQuotaPhone{Phone: phone, Disabled: entry.Disabled})
		}
	}
	if len(out.Phones) > smsMaxAdminPhones {
		return SMSAccountQuota{}, smsConfigError("at most 10 phone numbers")
	}
	return out, nil
}

var smsPhonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)

// normalizeSMSPhone accepts a mainland China mobile number, with or without +86 and with
// spaces or dashes, and returns its 11 digits.
func normalizeSMSPhone(raw string) (string, bool) {
	phone := strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(raw))
	switch {
	case strings.HasPrefix(phone, "+86"):
		phone = phone[3:]
	case strings.HasPrefix(phone, "0086"):
		phone = phone[4:]
	case len(phone) == 13 && strings.HasPrefix(phone, "86"):
		phone = phone[2:]
	}
	if !smsPhonePattern.MatchString(phone) {
		return "", false
	}
	return phone, true
}

// maskSMSPhone hides the middle of a number for logs: 138****5678.
func maskSMSPhone(phone string) string {
	if len(phone) != 11 {
		return phone
	}
	return phone[:3] + "****" + phone[7:]
}
