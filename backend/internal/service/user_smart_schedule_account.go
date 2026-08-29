package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (s *UserSmartScheduleService) ListAccountMemberships(ctx context.Context, accountID int64, platform string) ([]SmartScheduleAccountMembership, error) {
	if s == nil || s.repo == nil || accountID <= 0 {
		return []SmartScheduleAccountMembership{}, nil
	}
	platform = normalizeSmartSchedulePlatform(platform)
	if platform != "" && !IsAllowedSmartSchedulePlatform(platform) {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_PLATFORM", "platform is not allowed")
	}
	rows, err := s.repo.ListMembershipsByAccount(ctx, accountID, platform)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []SmartScheduleAccountMembership{}
	}
	s.hydrateAccountMemberships(ctx, accountID, rows)
	return rows, nil
}

func (s *UserSmartScheduleService) AddAccountMember(ctx context.Context, accountID, userID int64, platform string) error {
	if s == nil || s.repo == nil {
		return infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable")
	}
	if accountID <= 0 || userID <= 0 {
		return infraerrors.BadRequest("SMART_SCHEDULE_INVALID_ACCOUNT", "account_id and user_id are required")
	}
	platform = normalizeSmartSchedulePlatform(platform)
	if !IsAllowedSmartSchedulePlatform(platform) {
		return infraerrors.BadRequest("SMART_SCHEDULE_INVALID_PLATFORM", "platform is required")
	}
	if err := s.ensureAccountMatchesTab(ctx, accountID, platform); err != nil {
		return err
	}
	if err := s.repo.AddMember(ctx, userID, accountID, platform); err != nil {
		return err
	}
	if s.cache != nil {
		_ = s.cache.Invalidate(ctx, userID)
	}
	return nil
}

func (s *UserSmartScheduleService) RemoveAccountMember(ctx context.Context, accountID, userID int64, platform string) error {
	if s == nil || s.repo == nil {
		return infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable")
	}
	if accountID <= 0 || userID <= 0 {
		return infraerrors.BadRequest("SMART_SCHEDULE_INVALID_ACCOUNT", "account_id and user_id are required")
	}
	platform = normalizeSmartSchedulePlatform(platform)
	if !IsAllowedSmartSchedulePlatform(platform) {
		return infraerrors.BadRequest("SMART_SCHEDULE_INVALID_PLATFORM", "platform is required")
	}
	if err := s.repo.RemoveMember(ctx, userID, accountID, platform); err != nil {
		return err
	}
	s.clearAccountMemberLiveState(ctx, accountID, userID, platform)
	if s.cache != nil {
		_ = s.cache.Invalidate(ctx, userID)
	}
	return nil
}

func (s *UserSmartScheduleService) SetAccountPairAdmissionBatch(ctx context.Context, accountID int64, platform string, userIDs []int64, state string) ([]PairAdmissionResult, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable")
	}
	if accountID <= 0 {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_RESUME_INVALID", "account_id is required")
	}
	platform = normalizeSmartSchedulePlatform(platform)
	if !IsAllowedSmartSchedulePlatform(platform) {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_PLATFORM", "platform is required")
	}
	if _, err := ParsePairAdmissionState(state); err != nil {
		return nil, err
	}
	members, err := s.repo.ListMembershipsByAccount(ctx, accountID, platform)
	if err != nil {
		return nil, err
	}
	memberSet := make(map[int64]struct{}, len(members))
	allIDs := make([]int64, 0, len(members))
	for _, member := range members {
		if member.UserID <= 0 {
			continue
		}
		if _, seen := memberSet[member.UserID]; seen {
			continue
		}
		memberSet[member.UserID] = struct{}{}
		allIDs = append(allIDs, member.UserID)
	}
	targets := allIDs
	if len(userIDs) > 0 {
		targets = make([]int64, 0, len(userIDs))
		seen := map[int64]struct{}{}
		for _, userID := range userIDs {
			if userID <= 0 {
				continue
			}
			if _, ok := memberSet[userID]; !ok {
				return nil, infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "user is not in this platform pool")
			}
			if _, dup := seen[userID]; dup {
				continue
			}
			seen[userID] = struct{}{}
			targets = append(targets, userID)
		}
	}
	results := make([]PairAdmissionResult, 0, len(targets))
	for _, userID := range targets {
		result, err := s.SetPairAdmission(ctx, accountID, userID, state, platform)
		if err != nil {
			return results, err
		}
		if result != nil {
			results = append(results, *result)
		}
	}
	return results, nil
}

func (s *UserSmartScheduleService) ensureAccountMatchesTab(ctx context.Context, accountID int64, platform string) error {
	if s == nil || s.accountRepo == nil {
		return infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable")
	}
	accounts, err := s.accountRepo.GetByIDs(ctx, []int64{accountID})
	if err != nil {
		return err
	}
	if len(accounts) == 0 || accounts[0] == nil {
		return infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account not found")
	}
	if !smartScheduleAccountMatchesTab(accounts[0], platform) {
		return infraerrors.BadRequest("SMART_SCHEDULE_PLATFORM_MISMATCH", "account platform does not match the selected tab")
	}
	return nil
}

func (s *UserSmartScheduleService) hydrateAccountMemberships(ctx context.Context, accountID int64, rows []SmartScheduleAccountMembership) {
	if s == nil || accountID <= 0 || len(rows) == 0 {
		return
	}
	now := time.Now().UTC()
	accountIDs := []int64{accountID}
	for i := range rows {
		platform := normalizeSmartSchedulePlatform(rows[i].Platform)
		userID := rows[i].UserID
		if userID <= 0 {
			continue
		}
		if s.cache != nil {
			until := s.cache.GetCooldownUntilBatch(ctx, accountIDs, userID, platform, now)[accountID]
			if !until.IsZero() {
				copied := until
				rows[i].CooldownUntil = &copied
			}
			resume := s.cache.GetPairResumeUntilBatch(ctx, accountIDs, userID, platform, now)[accountID]
			if !resume.ChipUntil.IsZero() {
				copied := resume.ChipUntil
				rows[i].ResumeChipUntil = &copied
			}
			if !resume.WatchUntil.IsZero() {
				copied := resume.WatchUntil
				rows[i].ResumeWatchUntil = &copied
			}
			rows[i].Probing = s.cache.IsProbingBatch(ctx, accountIDs, userID, platform)[accountID]
			rows[i].Pinned = s.cache.IsPinnedBatch(ctx, accountIDs, userID, platform)[accountID]
		}
		if s.pairConcurrency != nil {
			occupancy, err := s.pairConcurrency.GetAccountUserConcurrencyBatch(
				WithScheduleLookupPlatform(ctx, platform),
				accountIDs,
				userID,
			)
			if err == nil {
				rows[i].CurrentConcurrency = occupancy[accountID]
			}
		}
	}
}

func (s *UserSmartScheduleService) clearAccountMemberLiveState(ctx context.Context, accountID, userID int64, platform string) {
	if s == nil || s.cache == nil {
		return
	}
	_ = s.cache.ClearCooldown(ctx, accountID, userID, platform)
	s.cache.ClearProbing(ctx, accountID, userID, platform)
	s.cache.ClearPinned(ctx, accountID, userID, platform)
	s.cache.ClearPairResume(ctx, accountID, userID, platform)
}
