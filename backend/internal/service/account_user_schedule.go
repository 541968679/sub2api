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
	ID      int64  `json:"id"`
	Email   string `json:"email"`
	Deleted bool   `json:"deleted"`
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

// AllowsScheduleUser reports whether this account may be scheduled for userID.
//
// Rules:
//   - unrestricted (or empty mode): always true
//   - userID <= 0 and mode is allow/deny: false (fail closed)
//   - allow + empty list: false (fail closed)
//   - deny + empty list: true (equivalent to unrestricted)
func (a *Account) AllowsScheduleUser(userID int64) bool {
	if a == nil {
		return false
	}
	mode := a.normalizedUserScheduleMode()
	if mode == UserScheduleModeUnrestricted {
		return true
	}
	if userID <= 0 {
		return false
	}
	switch mode {
	case UserScheduleModeAllow:
		if len(a.ScheduleUserIDs) == 0 {
			return false
		}
		return containsScheduleUserID(a.ScheduleUserIDs, userID)
	case UserScheduleModeDeny:
		if len(a.ScheduleUserIDs) == 0 {
			return true
		}
		return !containsScheduleUserID(a.ScheduleUserIDs, userID)
	default:
		return true
	}
}

func containsScheduleUserID(ids []int64, userID int64) bool {
	for _, id := range ids {
		if id == userID {
			return true
		}
	}
	return false
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
