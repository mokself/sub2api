package mok

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SMS log statuses.
const (
	smsStatusPending = "pending"
	smsStatusSent    = "sent"
	smsStatusFailed  = "failed"
	smsStatusDryRun  = "dry_run"
	smsStatusLimited = "limited"
)

// SMSStore persists phone numbers, verification codes and the send log
// (migrations/mok_006_sms.sql).
type SMSStore struct {
	db *sql.DB
}

func NewSMSStore(db *sql.DB) *SMSStore {
	return &SMSStore{db: db}
}

type smsPhone struct {
	Phone      string
	Enabled    bool
	VerifiedAt time.Time
}

func (s *SMSStore) GetPhone(ctx context.Context, userID int64) (smsPhone, bool, error) {
	var p smsPhone
	err := s.db.QueryRowContext(ctx, `
		SELECT phone, enabled, verified_at FROM mok_sms_phones WHERE user_id = $1`, userID).
		Scan(&p.Phone, &p.Enabled, &p.VerifiedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return smsPhone{}, false, nil
	}
	return p, err == nil, err
}

// SavePhone stores a verified number, replacing the user's previous one, switched on.
func (s *SMSStore) SavePhone(ctx context.Context, userID int64, phone string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mok_sms_phones (user_id, phone, enabled, verified_at, updated_at)
		VALUES ($1, $2, TRUE, $3, $3)
		ON CONFLICT (user_id) DO UPDATE SET
			phone = EXCLUDED.phone, enabled = TRUE,
			verified_at = EXCLUDED.verified_at, updated_at = EXCLUDED.updated_at`,
		userID, phone, at)
	return err
}

// SetPhoneEnabled switches the user's number on or off; found is false without a number.
func (s *SMSStore) SetPhoneEnabled(ctx context.Context, userID int64, enabled bool, at time.Time) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE mok_sms_phones SET enabled = $2, updated_at = $3 WHERE user_id = $1`, userID, enabled, at)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (s *SMSStore) DeletePhone(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM mok_sms_phones WHERE user_id = $1`, userID)
	return err
}

type smsCode struct {
	Phone     string
	CodeHash  string
	Attempts  int
	SentAt    time.Time
	ExpiresAt time.Time
}

func (s *SMSStore) GetCode(ctx context.Context, userID int64) (smsCode, bool, error) {
	var c smsCode
	err := s.db.QueryRowContext(ctx, `
		SELECT phone, code_hash, attempts, sent_at, expires_at FROM mok_sms_codes WHERE user_id = $1`, userID).
		Scan(&c.Phone, &c.CodeHash, &c.Attempts, &c.SentAt, &c.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return smsCode{}, false, nil
	}
	return c, err == nil, err
}

// SaveCode stores the user's new code, replacing an earlier one.
func (s *SMSStore) SaveCode(ctx context.Context, userID int64, c smsCode) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mok_sms_codes (user_id, phone, code_hash, attempts, sent_at, expires_at)
		VALUES ($1, $2, $3, 0, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET
			phone = EXCLUDED.phone, code_hash = EXCLUDED.code_hash, attempts = 0,
			sent_at = EXCLUDED.sent_at, expires_at = EXCLUDED.expires_at`,
		userID, c.Phone, c.CodeHash, c.SentAt, c.ExpiresAt)
	return err
}

// BumpCodeAttempts counts a wrong guess and returns the guesses made so far.
func (s *SMSStore) BumpCodeAttempts(ctx context.Context, userID int64) (int, error) {
	var attempts int
	err := s.db.QueryRowContext(ctx, `
		UPDATE mok_sms_codes SET attempts = attempts + 1 WHERE user_id = $1 RETURNING attempts`, userID).
		Scan(&attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return attempts, err
}

func (s *SMSStore) DeleteCode(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM mok_sms_codes WHERE user_id = $1`, userID)
	return err
}

// countedStatuses are the log statuses that took up a message: sent, being sent, or
// logged in dry-run mode. Failed and capped attempts do not count.
const countedStatuses = `('pending', 'sent', 'dry_run')`

// CountVerifyCodes counts the verification codes sent since a moment to a user, or to a
// phone number when userID is 0.
func (s *SMSStore) CountVerifyCodes(ctx context.Context, userID int64, phone string, since time.Time) (int, error) {
	query := `SELECT COUNT(*) FROM mok_sms_logs
		WHERE event = 'verify_code' AND status IN ` + countedStatuses + ` AND created_at >= $1 AND `
	arg := any(phone)
	if userID > 0 {
		query += `user_id = $2`
		arg = userID
	} else {
		query += `phone = $2`
	}
	var n int
	err := s.db.QueryRowContext(ctx, query, since, arg).Scan(&n)
	return n, err
}

// smsClaim describes a message about to be sent.
type smsClaim struct {
	Event      string
	UserID     int64 // 0 for admin alerts and test messages
	Phone      string
	DedupKey   string // empty: no deduplication
	TemplateID string
	Params     []string
	// Status is written when the message may go out: pending, or dry_run.
	Status string
	// DailyLimit caps the notifications of the user (admin alerts: of the phone) since
	// Since; 0 means no cap.
	DailyLimit int
	Since      time.Time
	// At is when the message is logged.
	At time.Time
}

// Claim writes the log row of a message before it is sent. It returns claimed=false when
// the deduplication key has been used for this phone already. Over the daily cap the row
// is written as limited (keeping the key, so the notice is not retried) and must not be
// sent. Claims for one user or admin phone are serialized, so the cap holds under
// concurrency and across instances.
func (s *SMSStore) Claim(ctx context.Context, c smsClaim) (id int64, status string, claimed bool, err error) {
	params, err := json.Marshal(nonNilParams(c.Params))
	if err != nil {
		return 0, "", false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, "", false, err
	}
	defer func() { _ = tx.Rollback() }()

	status = c.Status
	if c.DailyLimit > 0 {
		lockKey, countQuery, arg := "mok_sms:phone:"+c.Phone, `user_id IS NULL AND phone = $2`, any(c.Phone)
		if c.UserID > 0 {
			lockKey, countQuery, arg = fmt.Sprintf("mok_sms:user:%d", c.UserID), `user_id = $2`, any(c.UserID)
		}
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, lockKey); err != nil {
			return 0, "", false, err
		}
		var sent int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mok_sms_logs
			WHERE event NOT IN ('verify_code', 'test') AND status IN `+countedStatuses+`
			  AND created_at >= $1 AND `+countQuery, c.Since, arg).Scan(&sent); err != nil {
			return 0, "", false, err
		}
		if sent >= c.DailyLimit {
			status = smsStatusLimited
		}
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO mok_sms_logs (event, user_id, phone, dedup_key, template_id, params, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (event, dedup_key, phone) WHERE dedup_key IS NOT NULL DO NOTHING
		RETURNING id`,
		c.Event, nullUserID(c.UserID), c.Phone, nullString(c.DedupKey), c.TemplateID, string(params), status, c.At).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, err
	}
	if err := tx.Commit(); err != nil {
		return 0, "", false, err
	}
	return id, status, true, nil
}

// Finish records the outcome of a claimed message.
func (s *SMSStore) Finish(ctx context.Context, id int64, status, message, requestID string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE mok_sms_logs SET status = $2, error = $3, request_id = $4, finished_at = $5 WHERE id = $1`,
		id, status, truncateRunes(message, 500), truncateRunes(requestID, 64), at)
	return err
}

// SMSLogEntry is one row of the send log as the admin page shows it.
type SMSLogEntry struct {
	ID         int64      `json:"id"`
	CreatedAt  time.Time  `json:"created_at"`
	Event      string     `json:"event"`
	UserID     *int64     `json:"user_id"`
	UserEmail  string     `json:"user_email"`
	Phone      string     `json:"phone"`
	TemplateID string     `json:"template_id"`
	Params     []string   `json:"params"`
	Status     string     `json:"status"`
	Error      string     `json:"error"`
	RequestID  string     `json:"request_id"`
	FinishedAt *time.Time `json:"finished_at"`
}

// ListLogs returns the send log, newest first. search matches part of the phone number or
// of the user's email.
func (s *SMSStore) ListLogs(ctx context.Context, page, pageSize int, search string) ([]SMSLogEntry, int64, error) {
	where, args := "", []any{}
	if search = strings.TrimSpace(search); search != "" {
		args = append(args, "%"+escapeLike(search)+"%")
		where = `WHERE l.phone LIKE $1 ESCAPE '\' OR u.email ILIKE $1 ESCAPE '\'`
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mok_sms_logs l LEFT JOIN users u ON u.id = l.user_id `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT l.id, l.created_at, l.event, l.user_id, COALESCE(u.email, ''), l.phone, l.template_id,
		       l.params, l.status, l.error, l.request_id, l.finished_at
		FROM mok_sms_logs l LEFT JOIN users u ON u.id = l.user_id
		%s ORDER BY l.id DESC LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	entries := []SMSLogEntry{}
	for rows.Next() {
		var e SMSLogEntry
		var userID sql.NullInt64
		var finishedAt sql.NullTime
		var params []byte
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.Event, &userID, &e.UserEmail, &e.Phone, &e.TemplateID,
			&params, &e.Status, &e.Error, &e.RequestID, &finishedAt); err != nil {
			return nil, 0, err
		}
		if userID.Valid {
			id := userID.Int64
			e.UserID = &id
		}
		if finishedAt.Valid {
			t := finishedAt.Time
			e.FinishedAt = &t
		}
		if err := json.Unmarshal(params, &e.Params); err != nil || e.Params == nil {
			e.Params = []string{}
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}

// Prune deletes log rows older than a moment and verification codes that have expired.
func (s *SMSStore) Prune(ctx context.Context, logsBefore, now time.Time) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM mok_sms_logs WHERE created_at < $1`, logsBefore); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM mok_sms_codes WHERE expires_at < $1`, now)
	return err
}

func nonNilParams(params []string) []string {
	if params == nil {
		return []string{}
	}
	return params
}

func nullUserID(userID int64) sql.NullInt64 {
	return sql.NullInt64{Int64: userID, Valid: userID > 0}
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
