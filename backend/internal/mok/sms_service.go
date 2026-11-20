package mok

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strconv"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	smsConfigCacheTTL    = 30 * time.Second
	smsSendTimeout       = 15 * time.Second
	smsCodeTTL           = 5 * time.Minute
	smsCodeCooldown      = 60 * time.Second
	smsCodeMaxAttempts   = 5
	smsCodeDailyPerUser  = 5
	smsCodeDailyPerPhone = 5
	smsLogRetention      = 90 * 24 * time.Hour
	smsJobInterval       = 10 * time.Minute
	smsJobFirstRun       = time.Minute
	smsJobTimeout        = 5 * time.Minute
	smsPruneInterval     = 24 * time.Hour
	// smsParamMaxRunes keeps template parameters within Tencent Cloud's variable limits.
	smsParamMaxRunes = 20
	// smsExpiryReminderMaxDays is the earliest reminder, see smsExpiryReminderDays.
	smsExpiryReminderMaxDays = 7
)

// smsExpiryReminderDays are the days before expiry a reminder goes out, as upstream's email.
var smsExpiryReminderDays = map[int]bool{7: true, 3: true, 1: true}

var (
	ErrSMSUnavailable    = infraerrors.BadRequest("MOK_SMS_UNAVAILABLE", "SMS reminders are not available")
	ErrSMSNotConfigured  = infraerrors.BadRequest("MOK_SMS_NOT_CONFIGURED", "fill in and save the Tencent Cloud settings and the template first")
	ErrSMSPhoneInvalid   = infraerrors.BadRequest("MOK_SMS_PHONE_INVALID", "enter a mainland China mobile number")
	ErrSMSCodeDailyLimit = infraerrors.TooManyRequests("MOK_SMS_CODE_DAILY_LIMIT", "too many verification codes today")
	ErrSMSCodeInvalid    = infraerrors.BadRequest("MOK_SMS_CODE_INVALID", "the verification code is wrong")
	ErrSMSCodeExpired    = infraerrors.BadRequest("MOK_SMS_CODE_EXPIRED", "the verification code has expired; get a new one")
	ErrSMSSendFailed     = infraerrors.ServiceUnavailable("MOK_SMS_SEND_FAILED", "the SMS could not be sent; try again later")
	ErrSMSPhoneNotBound  = infraerrors.NotFound("MOK_SMS_PHONE_NOT_BOUND", "no phone number is bound")
	ErrSMSEventInvalid   = infraerrors.BadRequest("MOK_SMS_EVENT_INVALID", "unknown message kind")
)

func errSMSCodeCooldown(wait time.Duration) error {
	seconds := int((wait + time.Second - 1) / time.Second)
	return infraerrors.TooManyRequests("MOK_SMS_CODE_COOLDOWN", "wait before requesting another code").
		WithMetadata(map[string]string{"retry_after": strconv.Itoa(seconds)})
}

// SMSService is the fork's SMS channel: it implements service.MokSMSNotifier for the
// [mok] seams in upstream's notifications, lets users bind a phone number, sends the
// subscription expiry reminders and serves the admin settings and send log.
type SMSService struct {
	settings service.SettingRepository
	store    *SMSStore
	sender   SMSSender
	subs     service.UserSubscriptionRepository
	now      func() time.Time
	location func() *time.Location

	cacheMu  sync.Mutex
	cached   *smsSettings
	cachedAt time.Time

	lastPrune time.Time
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	start     sync.Once
	stop      sync.Once
}

func NewSMSService(settings service.SettingRepository, store *SMSStore, sender SMSSender, subs service.UserSubscriptionRepository) *SMSService {
	ctx, cancel := context.WithCancel(context.Background())
	return &SMSService{
		settings: settings,
		store:    store,
		sender:   sender,
		subs:     subs,
		now:      time.Now,
		location: timezone.Location,
		ctx:      ctx,
		cancel:   cancel,
	}
}

var _ service.MokSMSNotifier = (*SMSService)(nil)

// readSetting returns a stored value, or "" when the key is not set.
func (s *SMSService) readSetting(ctx context.Context, key string) (string, error) {
	raw, err := s.settings.GetValue(ctx, key)
	if errors.Is(err, service.ErrSettingNotFound) {
		return "", nil
	}
	return raw, err
}

// loadSettings reads all three parts. An admin alert part that has not been saved yet
// comes from the fields the SMS card held in its first release.
func (s *SMSService) loadSettings(ctx context.Context) (smsSettings, error) {
	values, err := s.settings.GetMultiple(ctx, []string{smsConfigKey, smsOpsAlertKey, smsAccountQuotaKey})
	if err != nil {
		return smsSettings{}, err
	}
	main := values[smsConfigKey]
	settings := smsSettings{cfg: parseSMSConfig(main)}
	legacy := parseSMSLegacyAdminAlerts(main)
	ops, ok := parseSMSOpsAlert(values[smsOpsAlertKey])
	if !ok {
		ops = legacy.opsAlert()
	}
	quota, ok := parseSMSAccountQuota(values[smsAccountQuotaKey])
	if !ok {
		quota = legacy.accountQuota()
	}
	settings.ops, settings.quota = ops, quota
	s.remember(settings)
	return settings, nil
}

func (s *SMSService) remember(settings smsSettings) {
	s.cacheMu.Lock()
	s.cached, s.cachedAt = &settings, s.now()
	s.cacheMu.Unlock()
}

// current returns the settings, cached for a short while: notifications check them on hot
// paths. When they cannot be read it falls back to the last value, or everything off.
func (s *SMSService) current(ctx context.Context) smsSettings {
	s.cacheMu.Lock()
	if s.cached != nil && s.now().Sub(s.cachedAt) < smsConfigCacheTTL {
		settings := *s.cached
		s.cacheMu.Unlock()
		return settings
	}
	s.cacheMu.Unlock()
	settings, err := s.loadSettings(ctx)
	if err != nil {
		slog.Warn("mok sms: read settings failed", "error", err)
		s.cacheMu.Lock()
		defer s.cacheMu.Unlock()
		if s.cached != nil {
			return *s.cached
		}
		return smsSettings{cfg: defaultSMSConfig()}
	}
	return settings
}

// storeJSON writes one settings key.
func (s *SMSService) storeJSON(ctx context.Context, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.settings.Set(ctx, key, string(raw))
}

// AdminConfig returns the SMS card's configuration.
func (s *SMSService) AdminConfig(ctx context.Context) (SMSConfigView, error) {
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return SMSConfigView{}, err
	}
	return viewSMSConfig(settings.cfg), nil
}

// SaveConfig validates and stores the SMS card's configuration. An empty secret key keeps
// the stored one.
func (s *SMSService) SaveConfig(ctx context.Context, in SMSConfig) (SMSConfigView, error) {
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return SMSConfigView{}, err
	}
	cfg, err := normalizeSMSConfig(in, settings.cfg)
	if err != nil {
		return SMSConfigView{}, err
	}
	// Saving drops the first release's admin alert fields from smsConfigKey; keep what
	// they meant in the admin alert keys first.
	if err := s.keepLegacyAdminAlerts(ctx, settings); err != nil {
		return SMSConfigView{}, err
	}
	if err := s.storeJSON(ctx, smsConfigKey, cfg); err != nil {
		return SMSConfigView{}, err
	}
	settings.cfg = cfg
	s.remember(settings)
	return viewSMSConfig(cfg), nil
}

// keepLegacyAdminAlerts writes the admin alert parts that have not been saved yet.
func (s *SMSService) keepLegacyAdminAlerts(ctx context.Context, settings smsSettings) error {
	for key, value := range map[string]any{smsOpsAlertKey: settings.ops, smsAccountQuotaKey: settings.quota} {
		raw, err := s.readSetting(ctx, key)
		if err != nil {
			return err
		}
		if raw != "" {
			continue
		}
		if err := s.storeJSON(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

// AdminOpsAlert returns the SMS part of the ops alert settings.
func (s *SMSService) AdminOpsAlert(ctx context.Context) (SMSOpsAlertView, error) {
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return SMSOpsAlertView{}, err
	}
	return SMSOpsAlertView{SMSOpsAlert: settings.ops, Ready: settings.cfg.ready(service.MokSMSEventOpsAlert)}, nil
}

// SaveOpsAlert validates and stores the SMS part of the ops alert settings.
func (s *SMSService) SaveOpsAlert(ctx context.Context, in SMSOpsAlert) (SMSOpsAlertView, error) {
	ops, err := normalizeSMSOpsAlert(in)
	if err != nil {
		return SMSOpsAlertView{}, err
	}
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return SMSOpsAlertView{}, err
	}
	if err := s.storeJSON(ctx, smsOpsAlertKey, ops); err != nil {
		return SMSOpsAlertView{}, err
	}
	settings.ops = ops
	s.remember(settings)
	return SMSOpsAlertView{SMSOpsAlert: ops, Ready: settings.cfg.ready(service.MokSMSEventOpsAlert)}, nil
}

// AdminAccountQuota returns the SMS phones of the account quota alerts.
func (s *SMSService) AdminAccountQuota(ctx context.Context) (SMSAccountQuotaView, error) {
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return SMSAccountQuotaView{}, err
	}
	return SMSAccountQuotaView{SMSAccountQuota: settings.quota, Ready: settings.cfg.ready(service.MokSMSEventAccountQuota)}, nil
}

// SaveAccountQuota validates and stores the SMS phones of the account quota alerts.
func (s *SMSService) SaveAccountQuota(ctx context.Context, in SMSAccountQuota) (SMSAccountQuotaView, error) {
	quota, err := normalizeSMSAccountQuota(in)
	if err != nil {
		return SMSAccountQuotaView{}, err
	}
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return SMSAccountQuotaView{}, err
	}
	if err := s.storeJSON(ctx, smsAccountQuotaKey, quota); err != nil {
		return SMSAccountQuotaView{}, err
	}
	settings.quota = quota
	s.remember(settings)
	return SMSAccountQuotaView{SMSAccountQuota: quota, Ready: settings.cfg.ready(service.MokSMSEventAccountQuota)}, nil
}

// MokSMSWanted implements service.MokSMSNotifier.
func (s *SMSService) MokSMSWanted(ctx context.Context, event string) bool {
	return s.current(ctx).wants(event)
}

// smsLevelRanks orders the ops alert levels, as upstream's email does.
var smsLevelRanks = map[string]int{"info": 1, "warning": 2, "critical": 3}

// MokNotifySMS implements service.MokSMSNotifier: it sends a notice to the user's bound
// phone, or to the admin alert phones when the notice has no user.
func (s *SMSService) MokNotifySMS(ctx context.Context, notice service.MokSMSNotice) {
	settings := s.current(ctx)
	if !settings.wants(notice.Event) {
		return
	}
	if notice.Event == service.MokSMSEventOpsAlert && settings.ops.MinLevel != "" &&
		smsLevelRanks[notice.Level] < smsLevelRanks[settings.ops.MinLevel] {
		return
	}
	cfg := settings.cfg
	phones := settings.recipients(notice.Event)
	if notice.UserID > 0 {
		phone, ok, err := s.store.GetPhone(ctx, notice.UserID)
		if err != nil {
			slog.Warn("mok sms: read phone failed", "user_id", notice.UserID, "error", err)
			return
		}
		if !ok || !phone.Enabled {
			return
		}
		phones = []string{phone.Phone}
	}
	params := make([]string, len(notice.Params))
	for i, p := range notice.Params {
		params[i] = truncateRunes(p, smsParamMaxRunes)
	}
	for _, phone := range phones {
		if ctx.Err() != nil {
			return
		}
		_, _, _ = s.deliver(ctx, cfg, smsMessage{
			event:      notice.Event,
			userID:     notice.UserID,
			phone:      phone,
			dedupKey:   notice.DedupKey,
			templateID: cfg.templateFor(notice.Event),
			params:     params,
			dailyLimit: cfg.DailyLimit,
		})
	}
}

// smsMessage is one message to one phone.
type smsMessage struct {
	event      string
	userID     int64
	phone      string
	dedupKey   string
	templateID string
	params     []string
	// logged replaces params in the send log (verification codes are not stored).
	logged     []string
	dailyLimit int
}

// deliver logs the message, then sends it unless it is a duplicate, over the daily cap
// or in dry-run mode. It returns the final status ("" for a duplicate) and the provider's
// error message.
func (s *SMSService) deliver(ctx context.Context, cfg SMSConfig, m smsMessage) (status, message string, err error) {
	logged := m.logged
	if logged == nil {
		logged = m.params
	}
	initial := smsStatusPending
	if cfg.DryRun {
		initial = smsStatusDryRun
	}
	now := s.now()
	id, status, claimed, err := s.store.Claim(ctx, smsClaim{
		Event:      m.event,
		UserID:     m.userID,
		Phone:      m.phone,
		DedupKey:   m.dedupKey,
		TemplateID: m.templateID,
		Params:     logged,
		Status:     initial,
		DailyLimit: m.dailyLimit,
		Since:      s.startOfDay(now),
		At:         now,
	})
	if err != nil {
		slog.Warn("mok sms: write log failed", "event", m.event, "phone", maskSMSPhone(m.phone), "error", err)
		return "", "", err
	}
	if !claimed {
		return "", "", nil
	}
	finish := func(status, message, requestID string) {
		writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := s.store.Finish(writeCtx, id, status, message, requestID, s.now()); err != nil {
			slog.Warn("mok sms: update log failed", "id", id, "error", err)
		}
	}
	switch status {
	case smsStatusLimited:
		slog.Info("mok sms: daily cap reached", "event", m.event, "user_id", m.userID, "phone", maskSMSPhone(m.phone))
		finish(smsStatusLimited, "", "")
		return smsStatusLimited, "", nil
	case smsStatusDryRun:
		finish(smsStatusDryRun, "", "")
		return smsStatusDryRun, "", nil
	}
	sendCtx, cancel := context.WithTimeout(ctx, smsSendTimeout)
	requestID, sendErr := s.sender.Send(sendCtx, cfg, m.phone, m.templateID, m.params)
	cancel()
	if sendErr != nil {
		slog.Warn("mok sms: send failed", "event", m.event, "phone", maskSMSPhone(m.phone), "error", sendErr)
		finish(smsStatusFailed, sendErr.Error(), requestID)
		return smsStatusFailed, sendErr.Error(), nil
	}
	finish(smsStatusSent, "", requestID)
	return smsStatusSent, "", nil
}

func (s *SMSService) startOfDay(t time.Time) time.Time {
	loc := s.location()
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// SMSPhoneStatus is what the profile page shows about the user's SMS reminders.
type SMSPhoneStatus struct {
	// Available is false when the admin has not set SMS reminders up; the page then hides them.
	Available bool `json:"available"`
	// Phone is the bound number, empty without one.
	Phone   string `json:"phone"`
	Enabled bool   `json:"enabled"`
	// Events are the reminders the bound phone receives.
	Events []string `json:"events"`
}

func (s *SMSService) PhoneStatus(ctx context.Context, userID int64) (SMSPhoneStatus, error) {
	settings := s.current(ctx)
	status := SMSPhoneStatus{Available: settings.bindingAvailable(), Events: []string{}}
	for _, event := range userEvents {
		if settings.wants(event) {
			status.Events = append(status.Events, event)
		}
	}
	phone, ok, err := s.store.GetPhone(ctx, userID)
	if err != nil {
		return SMSPhoneStatus{}, err
	}
	if ok {
		status.Phone, status.Enabled = phone.Phone, phone.Enabled
	}
	return status, nil
}

// SMSCodeSent tells the profile page when it may ask for another code.
type SMSCodeSent struct {
	CooldownSeconds  int `json:"cooldown_seconds"`
	ExpiresInSeconds int `json:"expires_in_seconds"`
}

// SendBindCode sends a verification code to the number the user wants to bind.
func (s *SMSService) SendBindCode(ctx context.Context, userID int64, rawPhone string) (SMSCodeSent, error) {
	settings := s.current(ctx)
	if !settings.bindingAvailable() {
		return SMSCodeSent{}, ErrSMSUnavailable
	}
	cfg := settings.cfg
	phone, ok := normalizeSMSPhone(rawPhone)
	if !ok {
		return SMSCodeSent{}, ErrSMSPhoneInvalid
	}
	now := s.now()
	if pending, ok, err := s.store.GetCode(ctx, userID); err != nil {
		return SMSCodeSent{}, err
	} else if ok && now.Sub(pending.SentAt) < smsCodeCooldown {
		return SMSCodeSent{}, errSMSCodeCooldown(smsCodeCooldown - now.Sub(pending.SentAt))
	}
	since := s.startOfDay(now)
	if n, err := s.store.CountVerifyCodes(ctx, userID, "", since); err != nil {
		return SMSCodeSent{}, err
	} else if n >= smsCodeDailyPerUser {
		return SMSCodeSent{}, ErrSMSCodeDailyLimit
	}
	if n, err := s.store.CountVerifyCodes(ctx, 0, phone, since); err != nil {
		return SMSCodeSent{}, err
	} else if n >= smsCodeDailyPerPhone {
		return SMSCodeSent{}, ErrSMSCodeDailyLimit
	}
	code, err := randomSMSCode()
	if err != nil {
		return SMSCodeSent{}, err
	}
	minutes := strconv.Itoa(int(smsCodeTTL / time.Minute))
	logged := []string{"******", minutes}
	if cfg.DryRun {
		// Nothing goes out in dry-run mode; the log is the only place to read the code.
		logged = []string{code, minutes}
	}
	status, _, err := s.deliver(ctx, cfg, smsMessage{
		event:      smsEventVerifyCode,
		userID:     userID,
		phone:      phone,
		templateID: cfg.Templates.VerifyCode,
		params:     []string{code, minutes},
		logged:     logged,
	})
	if err != nil {
		return SMSCodeSent{}, err
	}
	if status != smsStatusSent && status != smsStatusDryRun {
		return SMSCodeSent{}, ErrSMSSendFailed
	}
	if err := s.store.SaveCode(ctx, userID, smsCode{
		Phone:     phone,
		CodeHash:  hashSMSCode(phone, code),
		SentAt:    now,
		ExpiresAt: now.Add(smsCodeTTL),
	}); err != nil {
		return SMSCodeSent{}, err
	}
	return SMSCodeSent{CooldownSeconds: int(smsCodeCooldown / time.Second), ExpiresInSeconds: int(smsCodeTTL / time.Second)}, nil
}

// BindPhone checks the code and binds the number, replacing the user's previous one.
func (s *SMSService) BindPhone(ctx context.Context, userID int64, rawPhone, code string) (SMSPhoneStatus, error) {
	if !s.current(ctx).bindingAvailable() {
		return SMSPhoneStatus{}, ErrSMSUnavailable
	}
	phone, ok := normalizeSMSPhone(rawPhone)
	if !ok {
		return SMSPhoneStatus{}, ErrSMSPhoneInvalid
	}
	pending, ok, err := s.store.GetCode(ctx, userID)
	if err != nil {
		return SMSPhoneStatus{}, err
	}
	if !ok || pending.Phone != phone {
		return SMSPhoneStatus{}, ErrSMSCodeInvalid
	}
	if !s.now().Before(pending.ExpiresAt) {
		_ = s.store.DeleteCode(ctx, userID)
		return SMSPhoneStatus{}, ErrSMSCodeExpired
	}
	if subtle.ConstantTimeCompare([]byte(hashSMSCode(phone, code)), []byte(pending.CodeHash)) != 1 {
		attempts, err := s.store.BumpCodeAttempts(ctx, userID)
		if err != nil {
			return SMSPhoneStatus{}, err
		}
		if attempts >= smsCodeMaxAttempts {
			if err := s.store.DeleteCode(ctx, userID); err != nil {
				return SMSPhoneStatus{}, err
			}
			return SMSPhoneStatus{}, ErrSMSCodeExpired
		}
		return SMSPhoneStatus{}, ErrSMSCodeInvalid
	}
	if err := s.store.SavePhone(ctx, userID, phone, s.now()); err != nil {
		return SMSPhoneStatus{}, err
	}
	if err := s.store.DeleteCode(ctx, userID); err != nil {
		return SMSPhoneStatus{}, err
	}
	return s.PhoneStatus(ctx, userID)
}

// SetPhoneEnabled pauses or resumes the reminders to the bound number.
func (s *SMSService) SetPhoneEnabled(ctx context.Context, userID int64, enabled bool) (SMSPhoneStatus, error) {
	found, err := s.store.SetPhoneEnabled(ctx, userID, enabled, s.now())
	if err != nil {
		return SMSPhoneStatus{}, err
	}
	if !found {
		return SMSPhoneStatus{}, ErrSMSPhoneNotBound
	}
	return s.PhoneStatus(ctx, userID)
}

// RemovePhone unbinds the user's number.
func (s *SMSService) RemovePhone(ctx context.Context, userID int64) (SMSPhoneStatus, error) {
	if err := s.store.DeletePhone(ctx, userID); err != nil {
		return SMSPhoneStatus{}, err
	}
	return s.PhoneStatus(ctx, userID)
}

// SMSTestResult is the outcome of the admin's test message.
type SMSTestResult struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// testParams fills each template with sample values, in the parameter order of the event.
func (s *SMSService) testParams(event string) ([]string, error) {
	switch event {
	case smsEventVerifyCode:
		code, err := randomSMSCode()
		return []string{code, strconv.Itoa(int(smsCodeTTL / time.Minute))}, err
	case service.MokSMSEventBalanceLow:
		return []string{"8.50", "10.00"}, nil
	case service.MokSMSEventSubscriptionExpiry:
		expiry := s.now().Add(smsExpiryReminderMaxDays * 24 * time.Hour).In(s.location())
		return []string{"测试订阅", expiry.Format("2006-01-02 15:04"), strconv.Itoa(smsExpiryReminderMaxDays)}, nil
	case service.MokSMSEventOpsAlert:
		return []string{"测试告警", "P1", "95", "90"}, nil
	case service.MokSMSEventAccountQuota:
		return []string{"测试账号", "日限额", "20.00", "100.00"}, nil
	}
	return nil, ErrSMSEventInvalid
}

// TestSend sends one template with sample values to a number, using the saved settings.
// It works before SMS is switched on, ignores the switches and the daily cap, and is
// logged as a test message.
func (s *SMSService) TestSend(ctx context.Context, rawPhone, event string) (SMSTestResult, error) {
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return SMSTestResult{}, err
	}
	cfg := settings.cfg
	phone, ok := normalizeSMSPhone(rawPhone)
	if !ok {
		return SMSTestResult{}, ErrSMSPhoneInvalid
	}
	params, err := s.testParams(event)
	if err != nil {
		return SMSTestResult{}, err
	}
	templateID := cfg.templateFor(event)
	if templateID == "" || (!cfg.DryRun && !cfg.credentialsSet()) {
		return SMSTestResult{}, ErrSMSNotConfigured
	}
	status, message, err := s.deliver(ctx, cfg, smsMessage{
		event:      smsEventTest,
		phone:      phone,
		templateID: templateID,
		params:     params,
	})
	if err != nil {
		return SMSTestResult{}, err
	}
	return SMSTestResult{Status: status, Error: message}, nil
}

// ListLogs returns the send log, newest first.
func (s *SMSService) ListLogs(ctx context.Context, page, pageSize int, search string) ([]SMSLogEntry, int64, error) {
	return s.store.ListLogs(ctx, page, pageSize, search)
}

// Start runs the subscription expiry reminders and the log cleanup until Stop.
func (s *SMSService) Start() {
	if s == nil {
		return
	}
	s.start.Do(func() {
		s.wg.Add(1)
		go s.loop()
	})
}

// Stop ends the background job and waits for a running pass to finish or give up.
func (s *SMSService) Stop() {
	if s == nil {
		return
	}
	s.stop.Do(func() {
		s.cancel()
		s.wg.Wait()
	})
}

func (s *SMSService) loop() {
	defer s.wg.Done()
	timer := time.NewTimer(smsJobFirstRun)
	defer timer.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-timer.C:
			s.runOnce()
			timer.Reset(smsJobInterval)
		}
	}
}

func (s *SMSService) runOnce() {
	ctx, cancel := context.WithTimeout(s.ctx, smsJobTimeout)
	defer cancel()
	s.sendExpiryReminders(ctx)
	now := s.now()
	if now.Sub(s.lastPrune) >= smsPruneInterval {
		if err := s.store.Prune(ctx, now.Add(-smsLogRetention), now); err != nil {
			slog.Warn("mok sms: prune failed", "error", err)
			return
		}
		s.lastPrune = now
	}
}

// sendExpiryReminders sends the SMS counterpart of upstream's subscription expiry email:
// 7, 3 and 1 days before an active subscription expires, once each, to the user's bound
// phone. Every instance runs it; the deduplication key stops repeats.
func (s *SMSService) sendExpiryReminders(ctx context.Context) {
	if s.subs == nil || !s.current(ctx).wants(service.MokSMSEventSubscriptionExpiry) {
		return
	}
	now := s.now()
	for page := 1; ; page++ {
		subs, pag, err := s.subs.List(ctx, pagination.PaginationParams{Page: page, PageSize: 200}, nil, nil,
			service.SubscriptionStatusActive, "", "expires_at", "asc")
		if err != nil {
			slog.Warn("mok sms: list subscriptions failed", "error", err)
			return
		}
		for i := range subs {
			sub := &subs[i]
			days := smsDaysRemaining(sub.ExpiresAt, now)
			if days > smsExpiryReminderMaxDays {
				return // sorted by expiry: every later one is further away
			}
			if !smsExpiryReminderDays[days] {
				continue
			}
			name := fmt.Sprintf("#%d", sub.GroupID)
			if sub.Group != nil && sub.Group.Name != "" {
				name = sub.Group.Name
			}
			s.MokNotifySMS(ctx, service.MokSMSNotice{
				Event:    service.MokSMSEventSubscriptionExpiry,
				UserID:   sub.UserID,
				Params:   []string{name, sub.ExpiresAt.In(s.location()).Format("2006-01-02 15:04"), strconv.Itoa(days)},
				DedupKey: fmt.Sprintf("subscription-%d-%dd", sub.ID, days),
			})
		}
		if pag == nil || page >= pag.Pages || len(subs) == 0 {
			return
		}
	}
}

// smsDaysRemaining counts whole days left, rounding up, as UserSubscription.DaysRemaining.
func smsDaysRemaining(expiresAt, now time.Time) int {
	remaining := expiresAt.Sub(now)
	if remaining <= 0 {
		return 0
	}
	day := 24 * time.Hour
	days := int(remaining / day)
	if remaining%day != 0 {
		days++
	}
	return days
}

func randomSMSCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func hashSMSCode(phone, code string) string {
	sum := sha256.Sum256([]byte(phone + ":" + code))
	return hex.EncodeToString(sum[:])
}
