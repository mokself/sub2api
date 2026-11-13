package service

// [mok] Fork-only file (see FORK.md): the affiliate feature for selected users only.
//
// Upstream's switch (affiliate_enabled) turns the feature on or off for everyone. The scope
// adds a state in between: with scope "selected" the feature is on only for the users an
// admin switched it on for. For everyone else it is as if the feature were off: their
// invite code binds nobody, and the recharges and usage of the users they invited earlier
// earn them nothing. Rebate quota they already have stays theirs to transfer.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	MokAffiliateScopeAll      = "all"
	MokAffiliateScopeSelected = "selected"

	SettingKeyMokAffiliateScope = "mok_affiliate_scope"
)

// MokAffiliateSelectedUser is a user the affiliate feature is switched on for.
type MokAffiliateSelectedUser struct {
	UserID     int64     `json:"user_id"`
	Email      string    `json:"email"`
	Username   string    `json:"username"`
	SelectedAt time.Time `json:"selected_at"`
}

// MokAffiliateScopeStore is implemented by the affiliate repository in
// internal/repository/affiliate_scope_repo_mok.go.
type MokAffiliateScopeStore interface {
	MokIsAffiliateUserSelected(ctx context.Context, userID int64) (bool, error)
	// MokSetAffiliateUserSelected switches the user on or off. Switching on a user that does
	// not exist returns ErrUserNotFound.
	MokSetAffiliateUserSelected(ctx context.Context, userID int64, selected bool) error
	// MokListAffiliateSelectedUsers returns one page of the selected users that still exist,
	// most recently selected first, and their total.
	MokListAffiliateSelectedUsers(ctx context.Context, page, pageSize int) ([]MokAffiliateSelectedUser, int64, error)
	// MokAffiliateUserHasQuota reports whether the user has rebate quota, available or frozen.
	MokAffiliateUserHasQuota(ctx context.Context, userID int64) (bool, error)
}

// MokGetAffiliateScope returns the scope; anything but "selected" means everyone.
func (s *SettingService) MokGetAffiliateScope(ctx context.Context) (string, error) {
	if s == nil || s.settingRepo == nil {
		return MokAffiliateScopeAll, nil
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyMokAffiliateScope)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return MokAffiliateScopeAll, nil
		}
		return "", fmt.Errorf("read affiliate scope: %w", err)
	}
	if strings.TrimSpace(raw) == MokAffiliateScopeSelected {
		return MokAffiliateScopeSelected, nil
	}
	return MokAffiliateScopeAll, nil
}

// MokSetAffiliateScope stores the scope.
func (s *SettingService) MokSetAffiliateScope(ctx context.Context, scope string) error {
	if scope != MokAffiliateScopeAll && scope != MokAffiliateScopeSelected {
		return infraerrors.BadRequest("MOK_AFFILIATE_SCOPE_INVALID", "scope must be all or selected")
	}
	if s == nil || s.settingRepo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "setting service unavailable")
	}
	if err := s.settingRepo.Set(ctx, SettingKeyMokAffiliateScope, scope); err != nil {
		return fmt.Errorf("store affiliate scope: %w", err)
	}
	return nil
}

func (s *AffiliateService) mokScopeStore() (MokAffiliateScopeStore, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	store, ok := s.repo.(MokAffiliateScopeStore)
	if !ok {
		return nil, errors.New("affiliate repository cannot store selected users")
	}
	return store, nil
}

// mokInScope reports whether the scope lets the user take part as an inviter. It does not
// look at upstream's switch: its callers run behind upstream's own IsEnabled checks.
func (s *AffiliateService) mokInScope(ctx context.Context, userID int64) (bool, error) {
	if s == nil || s.settingService == nil {
		return true, nil
	}
	scope, err := s.settingService.MokGetAffiliateScope(ctx)
	if err != nil {
		return false, err
	}
	if scope != MokAffiliateScopeSelected {
		return true, nil
	}
	if userID <= 0 {
		return false, nil
	}
	store, err := s.mokScopeStore()
	if err != nil {
		return false, err
	}
	return store.MokIsAffiliateUserSelected(ctx, userID)
}

// MokAffiliateAllowed reports whether the affiliate feature is on for the user: upstream's
// switch is on and the scope includes the user.
func (s *AffiliateService) MokAffiliateAllowed(ctx context.Context, userID int64) (bool, error) {
	if s == nil || userID <= 0 || !s.IsEnabled(ctx) {
		return false, nil
	}
	return s.mokInScope(ctx, userID)
}

// MokAffiliateOwnAccess is what a signed-in user's menu needs to know.
type MokAffiliateOwnAccess struct {
	// Allowed is whether the affiliate feature is on for the user.
	Allowed bool `json:"allowed"`
	// HasQuota is set for a user the feature is off for who still has rebate quota. Such a
	// user keeps the menu entry, to be able to transfer the quota to the balance.
	HasQuota bool `json:"has_quota"`
}

// MokAffiliateOwnAccess reports whether the feature is on for the user and, when it is not
// because of the scope, whether the user still has rebate quota.
func (s *AffiliateService) MokAffiliateOwnAccess(ctx context.Context, userID int64) (*MokAffiliateOwnAccess, error) {
	access := &MokAffiliateOwnAccess{}
	if s == nil || userID <= 0 || !s.IsEnabled(ctx) {
		return access, nil
	}
	allowed, err := s.mokInScope(ctx, userID)
	if err != nil {
		return nil, err
	}
	access.Allowed = allowed
	if allowed {
		return access, nil
	}
	store, err := s.mokScopeStore()
	if err != nil {
		return nil, err
	}
	if access.HasQuota, err = store.MokAffiliateUserHasQuota(ctx, userID); err != nil {
		return nil, err
	}
	return access, nil
}

// mokCheckInviterInScope is called by BindInviterByCode through a [mok] seam once the code's
// owner is known. The code of an inviter outside the scope counts as invalid, so it binds
// nobody; registration itself goes on, as with any invalid code.
func (s *AffiliateService) mokCheckInviterInScope(ctx context.Context, inviterID int64) error {
	inScope, err := s.mokInScope(ctx, inviterID)
	if err != nil {
		return err
	}
	if !inScope {
		return ErrAffiliateCodeInvalid
	}
	return nil
}

// MokAffiliateScopeStatus is what the admin settings card shows.
type MokAffiliateScopeStatus struct {
	// AffiliateEnabled is upstream's switch.
	AffiliateEnabled bool   `json:"affiliate_enabled"`
	Scope            string `json:"scope"`
	SelectedCount    int64  `json:"selected_count"`
}

// MokAffiliateScopeStatus reports the scope and how many users are selected.
func (s *AffiliateService) MokAffiliateScopeStatus(ctx context.Context) (*MokAffiliateScopeStatus, error) {
	store, err := s.mokScopeStore()
	if err != nil {
		return nil, err
	}
	scope, err := s.settingService.MokGetAffiliateScope(ctx)
	if err != nil {
		return nil, err
	}
	_, count, err := store.MokListAffiliateSelectedUsers(ctx, 1, 1)
	if err != nil {
		return nil, err
	}
	return &MokAffiliateScopeStatus{AffiliateEnabled: s.IsEnabled(ctx), Scope: scope, SelectedCount: count}, nil
}

// MokSetAffiliateScope stores the scope and returns the new status.
func (s *AffiliateService) MokSetAffiliateScope(ctx context.Context, scope string) (*MokAffiliateScopeStatus, error) {
	if s == nil || s.settingService == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := s.settingService.MokSetAffiliateScope(ctx, scope); err != nil {
		return nil, err
	}
	return s.MokAffiliateScopeStatus(ctx)
}

// MokAffiliateUserAccess is one user's switch as the admin user editor shows it.
type MokAffiliateUserAccess struct {
	UserID int64 `json:"user_id"`
	// Selected is the user's own switch; it only matters while the scope is "selected".
	Selected         bool   `json:"selected"`
	Scope            string `json:"scope"`
	AffiliateEnabled bool   `json:"affiliate_enabled"`
	// Allowed is whether the affiliate feature is on for the user right now.
	Allowed bool `json:"allowed"`
}

// MokAffiliateUserAccess reports the user's switch and what it currently amounts to.
func (s *AffiliateService) MokAffiliateUserAccess(ctx context.Context, userID int64) (*MokAffiliateUserAccess, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	store, err := s.mokScopeStore()
	if err != nil {
		return nil, err
	}
	scope, err := s.settingService.MokGetAffiliateScope(ctx)
	if err != nil {
		return nil, err
	}
	selected, err := store.MokIsAffiliateUserSelected(ctx, userID)
	if err != nil {
		return nil, err
	}
	enabled := s.IsEnabled(ctx)
	return &MokAffiliateUserAccess{
		UserID:           userID,
		Selected:         selected,
		Scope:            scope,
		AffiliateEnabled: enabled,
		Allowed:          enabled && (scope != MokAffiliateScopeSelected || selected),
	}, nil
}

// MokSetAffiliateUserSelected switches the feature on or off for the user and returns the
// new state.
func (s *AffiliateService) MokSetAffiliateUserSelected(ctx context.Context, userID int64, selected bool) (*MokAffiliateUserAccess, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	store, err := s.mokScopeStore()
	if err != nil {
		return nil, err
	}
	if err := store.MokSetAffiliateUserSelected(ctx, userID, selected); err != nil {
		return nil, err
	}
	return s.MokAffiliateUserAccess(ctx, userID)
}

// MokListAffiliateSelectedUsers returns one page of the selected users and their total.
func (s *AffiliateService) MokListAffiliateSelectedUsers(ctx context.Context, page, pageSize int) ([]MokAffiliateSelectedUser, int64, error) {
	store, err := s.mokScopeStore()
	if err != nil {
		return nil, 0, err
	}
	users, total, err := store.MokListAffiliateSelectedUsers(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	if users == nil {
		users = []MokAffiliateSelectedUser{}
	}
	return users, total, nil
}
