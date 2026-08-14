package service

import (
	"context"
	"fmt"
	"strings"

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
	if err := s.validateKnownUserIDs(ctx, ids); err != nil {
		return "", nil, err
	}
	return normalized, ids, nil
}

func (s *adminServiceImpl) validateKnownUserIDs(ctx context.Context, ids []int64) error {
	ids = normalizeScheduleUserIDs(ids)
	if len(ids) == 0 || s == nil || s.accountRepo == nil {
		return nil
	}
	refs, err := s.accountRepo.ListScheduleUserRefs(ctx, ids)
	if err != nil {
		return err
	}
	found := make(map[int64]struct{}, len(refs))
	for _, ref := range refs {
		found[ref.ID] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := found[id]; !ok {
			return infraerrors.BadRequest("USER_SCHEDULE_UNKNOWN_USER", fmt.Sprintf("unknown user id: %d", id))
		}
	}
	return nil
}

func (s *adminServiceImpl) validateUserConcurrencyEntries(ctx context.Context, entries []UserConcurrencyEntry) (map[int64]int, error) {
	caps := make(map[int64]int, len(entries))
	ids := make([]int64, 0, len(entries))
	for _, entry := range entries {
		if entry.UserID <= 0 {
			return nil, infraerrors.BadRequest("USER_CONCURRENCY_UNKNOWN_USER", fmt.Sprintf("unknown user id: %d", entry.UserID))
		}
		if entry.MaxConcurrency < 1 {
			return nil, infraerrors.BadRequest("USER_CONCURRENCY_MIN", "explicit pair max concurrency must be >= 1")
		}
		caps[entry.UserID] = entry.MaxConcurrency
		ids = append(ids, entry.UserID)
	}
	if err := s.validateKnownUserIDs(ctx, ids); err != nil {
		return nil, err
	}
	return normalizeUserConcurrencyMap(caps), nil
}

func (s *adminServiceImpl) applyUserConcurrencyPatch(ctx context.Context, current map[int64]int, patch UserConcurrencyPatch) (map[int64]int, error) {
	if patch.UserID <= 0 {
		return nil, infraerrors.BadRequest("USER_CONCURRENCY_UNKNOWN_USER", fmt.Sprintf("unknown user id: %d", patch.UserID))
	}
	if err := s.validateKnownUserIDs(ctx, []int64{patch.UserID}); err != nil {
		return nil, err
	}
	out := copyUserConcurrencyMap(current)
	if out == nil {
		out = map[int64]int{}
	}
	if patch.MaxConcurrency == nil || *patch.MaxConcurrency == 0 {
		delete(out, patch.UserID)
		return normalizeUserConcurrencyMap(out), nil
	}
	if *patch.MaxConcurrency < 1 {
		return nil, infraerrors.BadRequest("USER_CONCURRENCY_MIN", "explicit pair max concurrency must be >= 1")
	}
	out[patch.UserID] = *patch.MaxConcurrency
	return out, nil
}

func (s *adminServiceImpl) validateUserQualityGateEntries(ctx context.Context, entries []UserQualityGateEntry) (map[int64]QualityHardCloseSettings, error) {
	gates := make(map[int64]QualityHardCloseSettings, len(entries))
	ids := make([]int64, 0, len(entries))
	for _, entry := range entries {
		if entry.UserID <= 0 {
			return nil, infraerrors.BadRequest("USER_QUALITY_GATE_UNKNOWN_USER", fmt.Sprintf("unknown user id: %d", entry.UserID))
		}
		if err := validateUserQualityGateFields(entry.MaxP50TTFTMs, entry.MinSuccessRate, entry.MinSuccessSamples, entry.MinTTFTSamples, entry.Condition); err != nil {
			return nil, err
		}
		gate, ok := userQualityGateFromFields(entry.MaxP50TTFTMs, entry.MinSuccessRate, entry.MinSuccessSamples, entry.MinTTFTSamples, entry.Condition)
		if !ok {
			continue
		}
		gates[entry.UserID] = gate
		ids = append(ids, entry.UserID)
	}
	if err := s.validateKnownUserIDs(ctx, ids); err != nil {
		return nil, err
	}
	return copyUserQualityGates(gates), nil
}

func (s *adminServiceImpl) applyUserQualityGatePatch(ctx context.Context, current map[int64]QualityHardCloseSettings, patch UserQualityGatePatch) (map[int64]QualityHardCloseSettings, error) {
	if patch.UserID <= 0 {
		return nil, infraerrors.BadRequest("USER_QUALITY_GATE_UNKNOWN_USER", fmt.Sprintf("unknown user id: %d", patch.UserID))
	}
	if err := s.validateKnownUserIDs(ctx, []int64{patch.UserID}); err != nil {
		return nil, err
	}
	if err := validateUserQualityGateFields(patch.MaxP50TTFTMs, patch.MinSuccessRate, patch.MinSuccessSamples, patch.MinTTFTSamples, patch.Condition); err != nil {
		return nil, err
	}
	out := copyUserQualityGates(current)
	if out == nil {
		out = map[int64]QualityHardCloseSettings{}
	}
	if qualityGateClears(patch.MaxP50TTFTMs, patch.MinSuccessRate, patch.MinSuccessSamples, patch.MinTTFTSamples, patch.Condition) {
		delete(out, patch.UserID)
		return copyUserQualityGates(out), nil
	}
	gate, ok := userQualityGateFromFields(patch.MaxP50TTFTMs, patch.MinSuccessRate, patch.MinSuccessSamples, patch.MinTTFTSamples, patch.Condition)
	if !ok {
		delete(out, patch.UserID)
		return copyUserQualityGates(out), nil
	}
	out[patch.UserID] = gate
	return out, nil
}

func validateUserQualityGateFields(p50 *int, rate *float64, minSuccess, minTTFT *int, condition *string) error {
	if p50 != nil && *p50 < 1 {
		return infraerrors.BadRequest("USER_QUALITY_GATE_INVALID", "quality_max_p50_ttft_ms must be >= 1")
	}
	if rate != nil && (*rate <= 0 || *rate > 1) {
		return infraerrors.BadRequest("USER_QUALITY_GATE_INVALID", "quality_min_success_rate must be in (0,1]")
	}
	if minSuccess != nil && *minSuccess < 1 {
		return infraerrors.BadRequest("USER_QUALITY_GATE_INVALID", "quality_min_success_samples must be >= 1")
	}
	if minTTFT != nil && *minTTFT < 1 {
		return infraerrors.BadRequest("USER_QUALITY_GATE_INVALID", "quality_min_ttft_samples must be >= 1")
	}
	if condition != nil && strings.TrimSpace(*condition) != "" {
		cond := strings.ToLower(strings.TrimSpace(*condition))
		if cond != QualityHardCloseConditionOr && cond != QualityHardCloseConditionAnd {
			return infraerrors.BadRequest("USER_QUALITY_GATE_INVALID", "quality_condition must be \"or\" or \"and\"")
		}
	}
	return nil
}

func applyLegacyUserScheduleLists(mode string, ids []int64) ([]int64, []int64) {
	switch NormalizeUserScheduleMode(mode) {
	case UserScheduleModeAllow:
		return normalizeScheduleUserIDs(ids), nil
	case UserScheduleModeDeny:
		return nil, normalizeScheduleUserIDs(ids)
	default:
		return nil, nil
	}
}

func scheduleUserUnionIDs(account Account) []int64 {
	return unionScheduleUserIDs(account.AllowUserIDs, account.DenyUserIDs, concurrencyUserIDs(account.UserConcurrency), qualityGateUserIDs(account.UserQualityGates), account.ScheduleUserIDs)
}

func stampScheduleUserFlags(account Account, ref ScheduleUserRef) ScheduleUserRef {
	ref.Allow = containsScheduleUserID(account.AllowUserIDs, ref.ID)
	ref.Deny = containsScheduleUserID(account.DenyUserIDs, ref.ID)
	if n := account.PairMaxConcurrency(ref.ID); n >= 1 {
		ref.MaxConcurrency = &n
	} else {
		ref.MaxConcurrency = nil
	}
	gate, ok := account.UserQualityGates[ref.ID]
	stampScheduleUserQuality(&ref, gate, ok)
	return ref
}

func (s *adminServiceImpl) hydrateAccountScheduleUsers(ctx context.Context, accounts []Account) {
	if s == nil || s.accountRepo == nil || len(accounts) == 0 {
		return
	}
	var allIDs []int64
	for i := range accounts {
		allIDs = append(allIDs, scheduleUserUnionIDs(accounts[i])...)
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
		ids := scheduleUserUnionIDs(accounts[i])
		if len(ids) == 0 {
			continue
		}
		users := make([]ScheduleUserRef, 0, len(ids))
		for _, id := range ids {
			ref, ok := byID[id]
			if !ok {
				ref = ScheduleUserRef{ID: id}
			}
			users = append(users, stampScheduleUserFlags(accounts[i], ref))
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

func (s *adminServiceImpl) resolveAccountUserScheduleWrite(ctx context.Context, current *Account, input *UpdateAccountInput) (AccountUserScheduleWrite, bool, error) {
	if current == nil || input == nil {
		return AccountUserScheduleWrite{}, false, nil
	}
	allow := append([]int64(nil), current.AllowUserIDs...)
	deny := append([]int64(nil), current.DenyUserIDs...)
	caps := copyUserConcurrencyMap(current.UserConcurrency)
	gates := copyUserQualityGates(current.UserQualityGates)
	changed := false

	if input.AllowUserIDs != nil {
		allow = normalizeScheduleUserIDs(*input.AllowUserIDs)
		if err := s.validateKnownUserIDs(ctx, allow); err != nil {
			return AccountUserScheduleWrite{}, false, err
		}
		changed = true
	}
	if input.DenyUserIDs != nil {
		deny = normalizeScheduleUserIDs(*input.DenyUserIDs)
		if err := s.validateKnownUserIDs(ctx, deny); err != nil {
			return AccountUserScheduleWrite{}, false, err
		}
		changed = true
	}
	if input.UserConcurrencies != nil && input.UserConcurrencyPatch != nil {
		return AccountUserScheduleWrite{}, false, infraerrors.BadRequest("USER_CONCURRENCY_CONFLICT", "user_concurrencies and user_concurrency_patch cannot be set together")
	}
	if input.UserConcurrencies != nil {
		next, err := s.validateUserConcurrencyEntries(ctx, *input.UserConcurrencies)
		if err != nil {
			return AccountUserScheduleWrite{}, false, err
		}
		caps = next
		changed = true
	}
	if input.UserConcurrencyPatch != nil {
		next, err := s.applyUserConcurrencyPatch(ctx, caps, *input.UserConcurrencyPatch)
		if err != nil {
			return AccountUserScheduleWrite{}, false, err
		}
		caps = next
		changed = true
	}
	if input.UserQualityGates != nil && input.UserQualityGatePatch != nil {
		return AccountUserScheduleWrite{}, false, infraerrors.BadRequest("USER_QUALITY_GATE_CONFLICT", "user_quality_gates and user_quality_gate_patch cannot be set together")
	}
	if input.UserQualityGates != nil {
		next, err := s.validateUserQualityGateEntries(ctx, *input.UserQualityGates)
		if err != nil {
			return AccountUserScheduleWrite{}, false, err
		}
		gates = next
		changed = true
	}
	if input.UserQualityGatePatch != nil {
		next, err := s.applyUserQualityGatePatch(ctx, gates, *input.UserQualityGatePatch)
		if err != nil {
			return AccountUserScheduleWrite{}, false, err
		}
		gates = next
		changed = true
	}

	useLegacy := (input.UserScheduleMode != nil || input.ScheduleUserIDs != nil) &&
		input.AllowUserIDs == nil && input.DenyUserIDs == nil
	if useLegacy {
		mode := current.UserScheduleMode
		ids := current.ScheduleUserIDs
		if input.UserScheduleMode != nil {
			mode = *input.UserScheduleMode
		}
		if input.ScheduleUserIDs != nil {
			ids = *input.ScheduleUserIDs
		}
		normalized, normalizedIDs, err := s.validateUserScheduleWrite(ctx, mode, ids)
		if err != nil {
			return AccountUserScheduleWrite{}, false, err
		}
		allow, deny = applyLegacyUserScheduleLists(normalized, normalizedIDs)
		changed = true
	}

	if !changed {
		return AccountUserScheduleWrite{}, false, nil
	}
	return buildAccountUserScheduleWrite(allow, deny, caps, gates), true, nil
}

func applyAccountUserScheduleWrite(account *Account, write AccountUserScheduleWrite) {
	if account == nil {
		return
	}
	account.AllowUserIDs = append([]int64(nil), write.AllowUserIDs...)
	account.DenyUserIDs = append([]int64(nil), write.DenyUserIDs...)
	account.UserConcurrency = copyUserConcurrencyMap(write.UserConcurrency)
	account.UserQualityGates = copyUserQualityGates(write.UserQualityGates)
	account.DeriveLegacyUserSchedule()
}
