package service

// [mok] Fork-only file (see FORK.md): the SMS channel of upstream's notifications.
//
// SMS is a channel of its own next to email: each one has its own recipients, so a
// notification can go out by email only, by SMS only, or both. Upstream decides when a
// notification is due (a balance crossing its threshold, an account quota crossing its
// alert line, an ops alert firing); [mok] seams at those points call the functions below,
// which hand the notice to the fork's SMS service (internal/mok/sms_service.go). That
// service picks the recipients (the user's bound phone, or the admin alert phones),
// enforces the daily cap and sends through Tencent Cloud.
//
// Recharge, subscription purchase and the content-moderation notices stay email only.
// Subscription expiry reminders are sent by the fork's own job, without a seam.

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

// SMS events. The template parameters of each event are listed next to it, in order.
const (
	// MokSMSEventBalanceLow: current balance, threshold.
	MokSMSEventBalanceLow = "balance_low"
	// MokSMSEventSubscriptionExpiry: subscription (group) name, expiry time, days remaining.
	MokSMSEventSubscriptionExpiry = "subscription_expiry"
	// MokSMSEventOpsAlert: rule name, severity, metric value, threshold.
	MokSMSEventOpsAlert = "ops_alert"
	// MokSMSEventAccountQuota: account name, quota dimension, remaining quota, quota limit.
	MokSMSEventAccountQuota = "account_quota"
)

// MokSMSNotice is one notification for the SMS channel.
type MokSMSNotice struct {
	Event string
	// UserID is the user to notify on their bound phone; 0 sends to the admin alert phones.
	UserID int64
	// Level is the alert level of an ops alert (critical, warning or info).
	Level string
	// Params are the template parameters, in template order.
	Params []string
	// DedupKey makes the notice go out at most once per phone; empty means no deduplication.
	DedupKey string
}

// MokSMSNotifier is implemented by the fork's SMS service.
type MokSMSNotifier interface {
	// MokSMSWanted reports, from cached settings, whether SMS is set up for the event.
	// Seams on hot paths call it before doing any work of their own.
	MokSMSWanted(ctx context.Context, event string) bool
	// MokNotifySMS sends the notice. Failures are logged, never returned.
	MokNotifySMS(ctx context.Context, notice MokSMSNotice)
}

type mokSMSHolder struct{ notifier MokSMSNotifier }

var mokSMSHook atomic.Pointer[mokSMSHolder]

// MokSetSMSNotifier installs the SMS service; the fork module calls it at startup.
// nil removes it.
func MokSetSMSNotifier(notifier MokSMSNotifier) {
	if notifier == nil {
		mokSMSHook.Store(nil)
		return
	}
	mokSMSHook.Store(&mokSMSHolder{notifier: notifier})
}

// mokSMSFor returns the installed notifier if SMS is set up for the event.
func mokSMSFor(ctx context.Context, event string) MokSMSNotifier {
	holder := mokSMSHook.Load()
	if holder == nil || holder.notifier == nil || !holder.notifier.MokSMSWanted(ctx, event) {
		return nil
	}
	return holder.notifier
}

// mokSMSDay is the server-time date that daily deduplication keys use.
func mokSMSDay(now time.Time) string {
	return now.In(timezone.Location()).Format("2006-01-02")
}

// mokSMSAmount formats an amount for an SMS parameter.
func mokSMSAmount(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

// mokSMSNumber formats a metric value without needless decimals: 95, 0.5, 12.35.
func mokSMSNumber(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}

// mokSendBalanceLowSMS is called through a [mok] seam in dispatchBalanceLowEmail, when the
// user's balance has just crossed their threshold. At most one SMS a day per user.
func (s *BalanceNotifyService) mokSendBalanceLowSMS(ctx context.Context, user *User, balance, threshold float64) {
	if user == nil || user.ID <= 0 {
		return
	}
	notifier := mokSMSFor(ctx, MokSMSEventBalanceLow)
	if notifier == nil {
		return
	}
	notifier.MokNotifySMS(ctx, MokSMSNotice{
		Event:    MokSMSEventBalanceLow,
		UserID:   user.ID,
		Params:   []string{mokSMSAmount(balance), mokSMSAmount(threshold)},
		DedupKey: fmt.Sprintf("user-%d-%s", user.ID, mokSMSDay(time.Now())),
	})
}

// mokSMSQuotaDimLabels are the quota dimensions as an SMS names them.
var mokSMSQuotaDimLabels = map[string]string{
	quotaDimDaily:  "日限额",
	quotaDimWeekly: "周限额",
	quotaDimTotal:  "总限额",
}

// mokCheckAccountQuotaSMS is called through a [mok] seam at the start of
// CheckAccountQuotaAfterIncrement. It applies the same crossing rule as the email alert,
// with the same per-account thresholds and the same global "account quota notification"
// switch, but does not need any admin email: the SMS goes to the admin alert phones.
// At most one SMS a day per account and quota dimension.
func (s *BalanceNotifyService) mokCheckAccountQuotaSMS(ctx context.Context, account *Account, cost float64, quotaState *AccountQuotaState) {
	if account == nil || s.settingRepo == nil || cost <= 0 {
		return
	}
	notifier := mokSMSFor(ctx, MokSMSEventAccountQuota)
	if notifier == nil || !s.isAccountQuotaNotifyEnabled(ctx) {
		return
	}
	var dims []quotaDim
	if quotaState != nil {
		dims = buildQuotaDimsFromState(account, quotaState)
	} else {
		account = s.fetchFreshAccount(ctx, account)
		dims = buildQuotaDims(account)
	}
	for _, dim := range dims {
		if !dim.enabled || dim.threshold <= 0 {
			continue
		}
		effectiveThreshold := dim.resolvedThreshold()
		if effectiveThreshold <= 0 {
			continue
		}
		if dim.currentUsed-cost >= effectiveThreshold || dim.currentUsed < effectiveThreshold {
			continue
		}
		label := mokSMSQuotaDimLabels[dim.name]
		if label == "" {
			label = dim.name
		}
		remaining := math.Max(dim.limit-dim.currentUsed, 0)
		notifier.MokNotifySMS(ctx, MokSMSNotice{
			Event:    MokSMSEventAccountQuota,
			Params:   []string{firstNonEmpty(account.Name, fmt.Sprintf("#%d", account.ID)), label, mokSMSAmount(remaining), mokSMSAmount(dim.limit)},
			DedupKey: fmt.Sprintf("account-%d-%s-%s", account.ID, dim.name, mokSMSDay(time.Now())),
		})
	}
}

// mokSendOpsAlertSMS is called through a [mok] seam in evaluateOnce, right after an alert
// event is created. It honours the alert silencing settings like the email does; the
// minimum level for SMS is a setting of its own.
func (s *OpsAlertEvaluatorService) mokSendOpsAlertSMS(ctx context.Context, runtimeCfg *OpsAlertRuntimeSettings, rule *OpsAlertRule, event *OpsAlertEvent) {
	if rule == nil || event == nil || event.ID <= 0 {
		return
	}
	notifier := mokSMSFor(ctx, MokSMSEventOpsAlert)
	if notifier == nil {
		return
	}
	if runtimeCfg != nil && runtimeCfg.Silencing.Enabled && isOpsAlertSilenced(time.Now().UTC(), rule, event, runtimeCfg.Silencing) {
		return
	}
	value, threshold := "-", mokSMSNumber(rule.Threshold)
	if event.MetricValue != nil {
		value = mokSMSNumber(*event.MetricValue)
	}
	if event.ThresholdValue != nil {
		threshold = mokSMSNumber(*event.ThresholdValue)
	}
	notifier.MokNotifySMS(ctx, MokSMSNotice{
		Event:    MokSMSEventOpsAlert,
		Level:    opsEmailSeverityForOps(rule.Severity),
		Params:   []string{firstNonEmpty(rule.Name, fmt.Sprintf("#%d", rule.ID)), firstNonEmpty(rule.Severity, "-"), value, threshold},
		DedupKey: fmt.Sprintf("event-%d", event.ID),
	})
}
