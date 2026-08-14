package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

const (
	UserScheduleModeUnrestricted = "unrestricted"
	UserScheduleModeAllow        = "allow"
	UserScheduleModeDeny         = "deny"
)

// ScheduleUserRef is the admin-facing hydration of a schedule-list user.
type ScheduleUserRef struct {
	ID             int64  `json:"id"`
	Email          string `json:"email"`
	Deleted        bool   `json:"deleted"`
	Allow          bool   `json:"allow"`
	Deny           bool   `json:"deny"`
	MaxConcurrency *int   `json:"max_concurrency,omitempty"`
}

// UserConcurrencyEntry is one replace-all pair-cap row on account update.
type UserConcurrencyEntry struct {
	UserID         int64 `json:"user_id"`
	MaxConcurrency int   `json:"max_concurrency"`
}

// UserConcurrencyPatch merges one user's pair cap without rewriting lists.
// MaxConcurrency nil or 0 deletes that user's cap.
type UserConcurrencyPatch struct {
	UserID         int64 `json:"user_id"`
	MaxConcurrency *int  `json:"max_concurrency"`
}

// AccountUserScheduleWrite is the full join-table replacement for one account.
type AccountUserScheduleWrite struct {
	AllowUserIDs    []int64
	DenyUserIDs     []int64
	UserConcurrency map[int64]int
}

func NormalizeUserScheduleMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case UserScheduleModeAllow:
		return UserScheduleModeAllow
	case UserScheduleModeDeny:
		return UserScheduleModeDeny
	case UserScheduleModeUnrestricted, "":
		return UserScheduleModeUnrestricted
	default:
		return strings.ToLower(strings.TrimSpace(mode))
	}
}

func IsValidUserScheduleMode(mode string) bool {
	switch NormalizeUserScheduleMode(mode) {
	case UserScheduleModeUnrestricted, UserScheduleModeAllow, UserScheduleModeDeny:
		return true
	default:
		return false
	}
}

func (a *Account) normalizedUserScheduleMode() string {
	if a == nil {
		return UserScheduleModeUnrestricted
	}
	return NormalizeUserScheduleMode(a.UserScheduleMode)
}

func (a *Account) hasUserScheduleRules() bool {
	if a == nil {
		return false
	}
	return len(a.AllowUserIDs) > 0 || len(a.DenyUserIDs) > 0 || len(a.UserConcurrency) > 0
}

// DeriveLegacyUserSchedule fills leftover exclusive-mode fields from the
// independent lists so old readers keep a best-effort view. Both lists
// nonempty cannot be expressed as a single mode; those stay unrestricted
// and leftover readers must not use mode as admission truth.
func (a *Account) DeriveLegacyUserSchedule() {
	if a == nil {
		return
	}
	allow := normalizeScheduleUserIDs(a.AllowUserIDs)
	deny := normalizeScheduleUserIDs(a.DenyUserIDs)
	a.AllowUserIDs = allow
	a.DenyUserIDs = deny
	a.UserConcurrency = normalizeUserConcurrencyMap(a.UserConcurrency)
	switch {
	case len(allow) > 0 && len(deny) == 0:
		a.UserScheduleMode = UserScheduleModeAllow
		a.ScheduleUserIDs = append([]int64(nil), allow...)
	case len(deny) > 0 && len(allow) == 0:
		a.UserScheduleMode = UserScheduleModeDeny
		a.ScheduleUserIDs = append([]int64(nil), deny...)
	default:
		a.UserScheduleMode = UserScheduleModeUnrestricted
		a.ScheduleUserIDs = unionScheduleUserIDs(allow, deny, concurrencyUserIDs(a.UserConcurrency))
	}
}

// AllowsScheduleUser reports whether this account may be scheduled for userID.
//
// Priority:
//  1. userID<=0 and any allow/deny/pair-cap rule exists → false (fail closed)
//  2. deny list hit → false (cap ignored)
//  3. allow list nonempty and miss → false (cap ignored)
//  4. else true
func (a *Account) AllowsScheduleUser(userID int64) bool {
	if a == nil {
		return false
	}
	if userID <= 0 {
		return !a.hasUserScheduleRules()
	}
	if containsScheduleUserID(a.DenyUserIDs, userID) {
		return false
	}
	if len(a.AllowUserIDs) > 0 && !containsScheduleUserID(a.AllowUserIDs, userID) {
		return false
	}
	return true
}

// PairMaxConcurrency returns the explicit pair cap N>=1, or 0 if unset.
// Callers must still check AllowsScheduleUser first; a denied user may still
// have a stored number that must not take effect.
func (a *Account) PairMaxConcurrency(userID int64) int {
	if a == nil || userID <= 0 || len(a.UserConcurrency) == 0 {
		return 0
	}
	n := a.UserConcurrency[userID]
	if n < 1 {
		return 0
	}
	return n
}

func containsScheduleUserID(ids []int64, userID int64) bool {
	for _, id := range ids {
		if id == userID {
			return true
		}
	}
	return false
}

func normalizeUserConcurrencyMap(in map[int64]int) map[int64]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int64]int, len(in))
	for userID, n := range in {
		if userID <= 0 || n < 1 {
			continue
		}
		out[userID] = n
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func concurrencyUserIDs(in map[int64]int) []int64 {
	if len(in) == 0 {
		return nil
	}
	out := make([]int64, 0, len(in))
	for userID, n := range in {
		if userID <= 0 || n < 1 {
			continue
		}
		out = append(out, userID)
	}
	return out
}

func unionScheduleUserIDs(groups ...[]int64) []int64 {
	var merged []int64
	for _, group := range groups {
		merged = append(merged, group...)
	}
	return normalizeScheduleUserIDs(merged)
}

func copyUserConcurrencyMap(in map[int64]int) map[int64]int {
	normalized := normalizeUserConcurrencyMap(in)
	if len(normalized) == 0 {
		return nil
	}
	out := make(map[int64]int, len(normalized))
	for k, v := range normalized {
		out[k] = v
	}
	return out
}

func buildAccountUserScheduleWrite(allow, deny []int64, caps map[int64]int) AccountUserScheduleWrite {
	return AccountUserScheduleWrite{
		AllowUserIDs:    normalizeScheduleUserIDs(allow),
		DenyUserIDs:     normalizeScheduleUserIDs(deny),
		UserConcurrency: copyUserConcurrencyMap(caps),
	}
}

// scheduleUserIDFromContext prefers an explicit positive user ID, then ctxkey.UserID.
func scheduleUserIDFromContext(ctx context.Context, explicit int64) int64 {
	if explicit > 0 {
		return explicit
	}
	if ctx == nil {
		return 0
	}
	if v, ok := ctx.Value(ctxkey.UserID).(int64); ok && v > 0 {
		return v
	}
	return 0
}

func withScheduleUserID(ctx context.Context, explicit int64) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	uid := scheduleUserIDFromContext(ctx, explicit)
	if uid <= 0 {
		return ctx
	}
	if existing, ok := ctx.Value(ctxkey.UserID).(int64); ok && existing == uid {
		return ctx
	}
	return context.WithValue(ctx, ctxkey.UserID, uid)
}
