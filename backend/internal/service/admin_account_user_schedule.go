package service

import (
	"context"
	"fmt"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func normalizeScheduleUserIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *adminServiceImpl) validateUserScheduleWrite(ctx context.Context, mode string, ids []int64) (string, []int64, error) {
	normalized := NormalizeUserScheduleMode(mode)
	if !IsValidUserScheduleMode(mode) {
		return "", nil, infraerrors.BadRequest("INVALID_USER_SCHEDULE_MODE", "user_schedule_mode must be unrestricted, allow, or deny")
	}
	ids = normalizeScheduleUserIDs(ids)
	if normalized == UserScheduleModeUnrestricted {
		return UserScheduleModeUnrestricted, nil, nil
	}
	if len(ids) == 0 {
		return "", nil, infraerrors.BadRequest("USER_SCHEDULE_USERS_REQUIRED", "allow/deny requires at least one user id")
	}
	if s.accountRepo == nil {
		return normalized, ids, nil
	}
	refs, err := s.accountRepo.ListScheduleUserRefs(ctx, ids)
	if err != nil {
		return "", nil, err
	}
	found := make(map[int64]struct{}, len(refs))
	for _, ref := range refs {
		found[ref.ID] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := found[id]; !ok {
			return "", nil, infraerrors.BadRequest("USER_SCHEDULE_UNKNOWN_USER", fmt.Sprintf("unknown user id: %d", id))
		}
	}
	return normalized, ids, nil
}

func (s *adminServiceImpl) hydrateAccountScheduleUsers(ctx context.Context, accounts []Account) {
	if s == nil || s.accountRepo == nil || len(accounts) == 0 {
		return
	}
	var allIDs []int64
	for i := range accounts {
		allIDs = append(allIDs, accounts[i].ScheduleUserIDs...)
	}
	if len(allIDs) == 0 {
		return
	}
	refs, err := s.accountRepo.ListScheduleUserRefs(ctx, allIDs)
	if err != nil {
		return
	}
	byID := make(map[int64]ScheduleUserRef, len(refs))
	for _, ref := range refs {
		byID[ref.ID] = ref
	}
	for i := range accounts {
		ids := accounts[i].ScheduleUserIDs
		if len(ids) == 0 {
			continue
		}
		users := make([]ScheduleUserRef, 0, len(ids))
		for _, id := range ids {
			if ref, ok := byID[id]; ok {
				users = append(users, ref)
				continue
			}
			users = append(users, ScheduleUserRef{ID: id})
		}
		accounts[i].ScheduleUsers = users
	}
}

func (s *adminServiceImpl) hydrateAccountScheduleUser(ctx context.Context, account *Account) {
	if account == nil {
		return
	}
	tmp := []Account{*account}
	s.hydrateAccountScheduleUsers(ctx, tmp)
	account.ScheduleUsers = tmp[0].ScheduleUsers
}
