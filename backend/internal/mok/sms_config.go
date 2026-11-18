package mok

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// smsConfigKey is the settings key the SMS configuration is stored under, as JSON.
const smsConfigKey = "mok_sms_config"

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

// SMSEvents switches the notifications on and off one by one.
type SMSEvents struct {
	BalanceLow         bool `json:"balance_low"`
	SubscriptionExpiry bool `json:"subscription_expiry"`
	OpsAlert           bool `json:"ops_alert"`
	AccountQuota       bool `json:"account_quota"`
}

// SMSConfig is the stored SMS configuration.
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
	// AdminPhones receive the ops alerts and account quota alerts.
	AdminPhones []string `json:"admin_phones"`
	// OpsAlertMinLevel is the lowest ops alert level sent by SMS: info, warning, critical,
	// or empty for every level.
	OpsAlertMinLevel string `json:"ops_alert_min_level"`
}

// defaultSMSConfig is what an unconfigured site starts from: off, with every notification
// switched on so that filling in the templates is all it takes.
func defaultSMSConfig() SMSConfig {
	return SMSConfig{
		Region:     smsDefaultRegion,
		DailyLimit: smsDefaultDailyLimit,
		Events:     SMSEvents{BalanceLow: true, SubscriptionExpiry: true, OpsAlert: true, AccountQuota: true},
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

// eventOn reports whether a notification is switched on.
func (c SMSConfig) eventOn(event string) bool {
	switch event {
	case service.MokSMSEventBalanceLow:
		return c.Events.BalanceLow
	case service.MokSMSEventSubscriptionExpiry:
		return c.Events.SubscriptionExpiry
	case service.MokSMSEventOpsAlert:
		return c.Events.OpsAlert
	case service.MokSMSEventAccountQuota:
		return c.Events.AccountQuota
	}
	return false
}

// wants reports whether a notification goes out by SMS. Admin alerts also need a phone.
func (c SMSConfig) wants(event string) bool {
	if !c.canSend() || !c.eventOn(event) || c.templateFor(event) == "" {
		return false
	}
	if event == service.MokSMSEventOpsAlert || event == service.MokSMSEventAccountQuota {
		return len(c.AdminPhones) > 0
	}
	return true
}

// userEvents are the notifications users get on their bound phone.
var userEvents = []string{service.MokSMSEventBalanceLow, service.MokSMSEventSubscriptionExpiry}

// bindingAvailable reports whether users can bind a phone: codes can be sent and at least
// one notification would reach that phone.
func (c SMSConfig) bindingAvailable() bool {
	if !c.canSend() || c.Templates.VerifyCode == "" {
		return false
	}
	for _, event := range userEvents {
		if c.wants(event) {
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

func smsConfigError(message string) error {
	return infraerrors.BadRequest("MOK_SMS_CONFIG_INVALID", message)
}

// normalizeSMSConfig validates an update from the admin page. An empty secret key keeps
// the stored one.
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
	out.OpsAlertMinLevel = strings.ToLower(strings.TrimSpace(in.OpsAlertMinLevel))

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
	if !smsMinLevels[out.OpsAlertMinLevel] {
		return SMSConfig{}, smsConfigError("unsupported ops alert level")
	}
	for _, id := range []*string{&out.Templates.VerifyCode, &out.Templates.BalanceLow, &out.Templates.SubscriptionExpiry, &out.Templates.OpsAlert, &out.Templates.AccountQuota} {
		*id = strings.TrimSpace(*id)
		if *id != "" && (!smsDigits.MatchString(*id) || len(*id) > 20) {
			return SMSConfig{}, smsConfigError("a template ID is a number such as 1234567")
		}
	}
	phones := make([]string, 0, len(in.AdminPhones))
	seen := map[string]bool{}
	for _, raw := range in.AdminPhones {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		phone, ok := normalizeSMSPhone(raw)
		if !ok {
			return SMSConfig{}, smsConfigError("invalid admin phone number: " + strings.TrimSpace(raw))
		}
		if !seen[phone] {
			seen[phone] = true
			phones = append(phones, phone)
		}
	}
	if len(phones) > smsMaxAdminPhones {
		return SMSConfig{}, smsConfigError("at most 10 admin phone numbers")
	}
	out.AdminPhones = phones
	if out.Enabled && !out.DryRun && !out.credentialsSet() {
		return SMSConfig{}, smsConfigError("fill in SecretId, SecretKey, SDK AppID and the signature before switching SMS on")
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
