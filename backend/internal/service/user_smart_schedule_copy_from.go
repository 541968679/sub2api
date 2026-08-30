package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (s *UserSmartScheduleService) PreviewCopyFromUser(ctx context.Context, targetID int64, platform string, sourceID int64) (*SmartScheduleCopyFromPreview, error) {
	snapshot, err := s.loadCopyFromSnapshot(ctx, targetID, platform, sourceID)
	if err != nil {
		return nil, err
	}
	add, remove, overlap := diffSmartScheduleAccountIDs(snapshot.sourceMembers, snapshot.targetMembers)
	return &SmartScheduleCopyFromPreview{
		SourceRevision:         snapshot.revision,
		SkippedUnavailable:     snapshot.skipped,
		Add:                    add,
		Remove:                 remove,
		Overlap:                overlap,
		SourcePausedAccountIDs: pausedAccountIDs(snapshot.sourceMembers),
		EnabledDelta:           copyFromEnabledDelta(snapshot.source.Enabled, snapshot.target.Enabled),
		SourceEmpty:            len(snapshot.sourceMembers) == 0,
		SourceMembers:          snapshot.sourceMembers,
		TargetMembers:          snapshot.targetMembers,
	}, nil
}

func (s *UserSmartScheduleService) CopyFromUser(ctx context.Context, targetID int64, platform string, sourceID int64, revision string, slices SmartScheduleCopySlices) (*UserSmartScheduleView, error) {
	platform = normalizeSmartSchedulePlatform(platform)
	if !IsAllowedSmartSchedulePlatform(platform) {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_PLATFORM", "invalid platform")
	}
	if targetID <= 0 || sourceID <= 0 || sourceID == targetID {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_COPY_INVALID", "source_user_id is required and must be a different user")
	}
	if err := validateSmartScheduleCopySlices(slices); err != nil {
		return nil, err
	}
	snapshot, err := s.loadCopyFromSnapshot(ctx, targetID, platform, sourceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(revision) == "" || revision != snapshot.revision {
		return nil, infraerrors.Conflict("SMART_SCHEDULE_COPY_STALE", "source smart schedule changed; refresh preview")
	}
	if slices.Pool && len(snapshot.sourceMembers) == 0 {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_COPY_EMPTY_SOURCE", "source platform has no usable pool members")
	}

	write := platformViewToWrite(snapshot.target)
	if slices.Thresholds {
		copySmartScheduleThresholds(snapshot.source, &write)
	}
	if slices.Pool {
		write.Accounts = copyPoolMembersFromSource(snapshot.sourceMembers, slices.Concurrency, slices.SortOrder)
	}
	if slices.Enabled {
		write.Enabled = snapshot.source.Enabled
	}
	if write.Enabled && len(write.Accounts) == 0 {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_EMPTY_POOL", "cannot enable smart schedule with an empty account pool")
	}
	if len(write.Accounts) == 0 {
		write.Enabled = false
	}

	pausedByID := map[int64]bool{}
	for _, member := range write.Accounts {
		pausedByID[member.AccountID] = member.Paused
	}
	normalized, err := normalizeSmartScheduleWrite(write)
	if err != nil {
		return nil, err
	}
	for i := range normalized.Accounts {
		normalized.Accounts[i].Paused = pausedByID[normalized.Accounts[i].AccountID]
	}

	if s == nil || s.repo == nil {
		return nil, infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable")
	}
	if err := s.repo.ReplacePlatformWithMemberPaused(ctx, targetID, platform, normalized); err != nil {
		return nil, err
	}
	if s.cache != nil {
		_ = s.cache.Invalidate(ctx, targetID)
	}
	return s.Get(ctx, targetID)
}

type copyFromSnapshot struct {
	source        SmartSchedulePlatformView
	target        SmartSchedulePlatformView
	sourceMembers []SmartScheduleAccountMember
	targetMembers []SmartScheduleAccountMember
	skipped       int
	revision      string
}

func (s *UserSmartScheduleService) loadCopyFromSnapshot(ctx context.Context, targetID int64, platform string, sourceID int64) (*copyFromSnapshot, error) {
	platform = normalizeSmartSchedulePlatform(platform)
	if !IsAllowedSmartSchedulePlatform(platform) {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_PLATFORM", "invalid platform")
	}
	if targetID <= 0 || sourceID <= 0 || sourceID == targetID {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_COPY_INVALID", "source_user_id is required and must be a different user")
	}
	sourceView, err := s.Get(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	targetView, err := s.Get(ctx, targetID)
	if err != nil {
		return nil, err
	}
	source := sourceView.Platforms[platform]
	target := targetView.Platforms[platform]
	sourceMembers, skipped, err := s.usableCopyFromMembers(ctx, sourceID, platform, source.Accounts)
	if err != nil {
		return nil, err
	}
	targetMembers, _, err := s.usableCopyFromMembers(ctx, targetID, platform, target.Accounts)
	if err != nil {
		return nil, err
	}
	source.Accounts = sourceMembers
	return &copyFromSnapshot{
		source:        source,
		target:        target,
		sourceMembers: sourceMembers,
		targetMembers: targetMembers,
		skipped:       skipped,
		revision:      smartScheduleCopyRevision(source, sourceMembers),
	}, nil
}

func (s *UserSmartScheduleService) usableCopyFromMembers(ctx context.Context, userID int64, platform string, members []SmartScheduleAccountMember) ([]SmartScheduleAccountMember, int, error) {
	rawIDs := smartScheduleMemberIDs(members)
	if s != nil && s.repo != nil {
		if bundle, err := s.repo.ListByUser(ctx, userID); err == nil && bundle != nil {
			if policy := bundle.Policy(platform); policy != nil {
				rawIDs = smartSchedulePolicyAccountIDs(policy)
			}
		}
	}
	if len(rawIDs) == 0 {
		return []SmartScheduleAccountMember{}, 0, nil
	}
	if s == nil || s.accountRepo == nil {
		return nil, 0, infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable")
	}
	accounts, err := s.accountRepo.GetByIDs(ctx, rawIDs)
	if err != nil {
		return nil, 0, err
	}
	byID := make(map[int64]*Account, len(accounts))
	for _, acc := range accounts {
		if acc != nil {
			byID[acc.ID] = acc
		}
	}
	skipped := 0
	for _, id := range rawIDs {
		if byID[id] == nil {
			skipped++
		}
	}
	memberByID := make(map[int64]SmartScheduleAccountMember, len(members))
	for _, member := range members {
		memberByID[member.AccountID] = member
	}
	kept := make([]SmartScheduleAccountMember, 0, len(rawIDs))
	for _, id := range rawIDs {
		acc := byID[id]
		if acc == nil {
			continue
		}
		if !smartScheduleAccountMatchesTab(acc, platform) {
			return nil, 0, infraerrors.BadRequest("SMART_SCHEDULE_PLATFORM_MISMATCH", "account platform does not match the selected tab")
		}
		member, ok := memberByID[id]
		if !ok {
			member = SmartScheduleAccountMember{AccountID: id, Platform: platform}
		}
		member.AccountID = id
		member.Platform = platform
		member.CurrentConcurrency = 0
		member.CooldownUntil = nil
		member.CooldownReason = nil
		member.SoftCooldownProgress = nil
		member.ResumeUntil = nil
		member.ResumeChipUntil = nil
		member.Probing = false
		member.Pinned = false
		member.ProbeCap = nil
		member.PairQuality = nil
		member.WillCool = false
		member.QualityReason = nil
		member.Priority = 0
		kept = append(kept, member)
	}
	return kept, skipped, nil
}

func validateSmartScheduleCopySlices(slices SmartScheduleCopySlices) error {
	if !slices.Pool && (slices.Concurrency || slices.SortOrder) {
		return infraerrors.BadRequest("SMART_SCHEDULE_COPY_SLICES", "concurrency and sort_order require pool")
	}
	return nil
}

func platformViewToWrite(view SmartSchedulePlatformView) SmartSchedulePlatformWrite {
	accounts := make([]SmartScheduleAccountMember, 0, len(view.Accounts))
	for _, member := range view.Accounts {
		copied := member
		copied.CurrentConcurrency = 0
		copied.CooldownUntil = nil
		copied.CooldownReason = nil
		copied.SoftCooldownProgress = nil
		copied.ResumeUntil = nil
		copied.ResumeChipUntil = nil
		copied.Probing = false
		copied.Pinned = false
		copied.ProbeCap = nil
		copied.PairQuality = nil
		copied.WillCool = false
		copied.QualityReason = nil
		copied.Priority = 0
		accounts = append(accounts, copied)
	}
	return SmartSchedulePlatformWrite{
		Enabled:                        view.Enabled,
		QualityMaxP50TTFTMs:            view.QualityMaxP50TTFTMs,
		QualityMinSuccessRate:          view.QualityMinSuccessRate,
		QualityWindowSamples:           view.QualityWindowSamples,
		QualityWindowN:                 view.QualityWindowN,
		QualityMinSuccessSamples:       view.QualityMinSuccessSamples,
		QualityMinTTFTSamples:          view.QualityMinTTFTSamples,
		QualityCondition:               view.QualityCondition,
		CooldownMinutes:                view.CooldownMinutes,
		SoftCooldown:                   view.SoftCooldown,
		ProbeConcurrencyMode:           view.ProbeConcurrencyMode,
		ProbeConcurrency:               view.ProbeConcurrency,
		QualityMaxSlowInWindow:         view.QualityMaxSlowInWindow,
		QualityMaxConsecutiveSlow:      view.QualityMaxConsecutiveSlow,
		QualityMaxP50DurationMs:        view.QualityMaxP50DurationMs,
		QualitySchedWindowN:            view.QualitySchedWindowN,
		QualitySchedMaxSlowInWindow:    view.QualitySchedMaxSlowInWindow,
		QualitySchedMaxConsecutiveSlow: view.QualitySchedMaxConsecutiveSlow,
		ProbeLatencyV2:                 view.ProbeLatencyV2,
		Accounts:                       accounts,
	}
}

func copySmartScheduleThresholds(from SmartSchedulePlatformView, write *SmartSchedulePlatformWrite) {
	write.QualityMaxP50TTFTMs = from.QualityMaxP50TTFTMs
	write.QualityMinSuccessRate = from.QualityMinSuccessRate
	write.QualityWindowSamples = from.QualityWindowSamples
	write.QualityWindowN = from.QualityWindowN
	write.QualityMinSuccessSamples = from.QualityMinSuccessSamples
	write.QualityMinTTFTSamples = from.QualityMinTTFTSamples
	write.QualityCondition = from.QualityCondition
	write.CooldownMinutes = from.CooldownMinutes
	write.SoftCooldown = from.SoftCooldown
	write.ProbeConcurrencyMode = from.ProbeConcurrencyMode
	write.ProbeConcurrency = from.ProbeConcurrency
	write.QualityMaxSlowInWindow = from.QualityMaxSlowInWindow
	write.QualityMaxConsecutiveSlow = from.QualityMaxConsecutiveSlow
	write.QualityMaxP50DurationMs = from.QualityMaxP50DurationMs
	write.QualitySchedWindowN = from.QualitySchedWindowN
	write.QualitySchedMaxSlowInWindow = from.QualitySchedMaxSlowInWindow
	write.QualitySchedMaxConsecutiveSlow = from.QualitySchedMaxConsecutiveSlow
	write.ProbeLatencyV2 = from.ProbeLatencyV2
}

func copyPoolMembersFromSource(source []SmartScheduleAccountMember, concurrency, sortOrder bool) []SmartScheduleAccountMember {
	out := make([]SmartScheduleAccountMember, 0, len(source))
	for _, member := range source {
		copied := member
		if !concurrency {
			copied.MaxConcurrency = nil
		}
		if !sortOrder {
			copied.SortOrder = nil
		}
		out = append(out, copied)
	}
	return out
}

func smartScheduleCopyRevision(source SmartSchedulePlatformView, members []SmartScheduleAccountMember) string {
	type memberPayload struct {
		AccountID      int64 `json:"account_id"`
		Paused         bool  `json:"paused"`
		MaxConcurrency *int  `json:"max_concurrency"`
		SortOrder      *int  `json:"sort_order"`
	}
	payload := struct {
		Enabled                        bool            `json:"enabled"`
		QualityMaxP50TTFTMs            *int            `json:"quality_max_p50_ttft_ms"`
		QualityMinSuccessRate          *float64        `json:"quality_min_success_rate"`
		QualityWindowSamples           *int            `json:"quality_window_samples"`
		QualityWindowN                 *int            `json:"quality_window_n"`
		QualityMinSuccessSamples       *int            `json:"quality_min_success_samples"`
		QualityMinTTFTSamples          *int            `json:"quality_min_ttft_samples"`
		QualityCondition               *string         `json:"quality_condition"`
		CooldownMinutes                int             `json:"cooldown_minutes"`
		SoftCooldown                   bool            `json:"soft_cooldown"`
		ProbeConcurrencyMode           string          `json:"probe_concurrency_mode"`
		ProbeConcurrency               *int            `json:"probe_concurrency"`
		QualityMaxSlowInWindow         *int            `json:"quality_max_slow_in_window"`
		QualityMaxConsecutiveSlow      *int            `json:"quality_max_consecutive_slow"`
		QualityMaxP50DurationMs        *int            `json:"quality_max_p50_duration_ms"`
		QualitySchedWindowN            *int            `json:"quality_sched_window_n"`
		QualitySchedMaxSlowInWindow    *int            `json:"quality_sched_max_slow_in_window"`
		QualitySchedMaxConsecutiveSlow *int            `json:"quality_sched_max_consecutive_slow"`
		ProbeLatencyV2                 bool            `json:"probe_latency_v2"`
		Members                        []memberPayload `json:"members"`
	}{
		Enabled:                        source.Enabled,
		QualityMaxP50TTFTMs:            source.QualityMaxP50TTFTMs,
		QualityMinSuccessRate:          source.QualityMinSuccessRate,
		QualityWindowSamples:           source.QualityWindowSamples,
		QualityWindowN:                 source.QualityWindowN,
		QualityMinSuccessSamples:       source.QualityMinSuccessSamples,
		QualityMinTTFTSamples:          source.QualityMinTTFTSamples,
		QualityCondition:               source.QualityCondition,
		CooldownMinutes:                source.CooldownMinutes,
		SoftCooldown:                   source.SoftCooldown,
		ProbeConcurrencyMode:           source.ProbeConcurrencyMode,
		ProbeConcurrency:               source.ProbeConcurrency,
		QualityMaxSlowInWindow:         source.QualityMaxSlowInWindow,
		QualityMaxConsecutiveSlow:      source.QualityMaxConsecutiveSlow,
		QualityMaxP50DurationMs:        source.QualityMaxP50DurationMs,
		QualitySchedWindowN:            source.QualitySchedWindowN,
		QualitySchedMaxSlowInWindow:    source.QualitySchedMaxSlowInWindow,
		QualitySchedMaxConsecutiveSlow: source.QualitySchedMaxConsecutiveSlow,
		ProbeLatencyV2:                 source.ProbeLatencyV2,
		Members:                        make([]memberPayload, 0, len(members)),
	}
	sorted := append([]SmartScheduleAccountMember(nil), members...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].AccountID < sorted[j].AccountID })
	for _, member := range sorted {
		payload.Members = append(payload.Members, memberPayload{
			AccountID:      member.AccountID,
			Paused:         member.Paused,
			MaxConcurrency: member.MaxConcurrency,
			SortOrder:      member.SortOrder,
		})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func diffSmartScheduleAccountIDs(source, target []SmartScheduleAccountMember) (add, remove, overlap []int64) {
	sourceSet := map[int64]struct{}{}
	targetSet := map[int64]struct{}{}
	for _, member := range source {
		if member.AccountID > 0 {
			sourceSet[member.AccountID] = struct{}{}
		}
	}
	for _, member := range target {
		if member.AccountID > 0 {
			targetSet[member.AccountID] = struct{}{}
		}
	}
	for id := range sourceSet {
		if _, ok := targetSet[id]; ok {
			overlap = append(overlap, id)
		} else {
			add = append(add, id)
		}
	}
	for id := range targetSet {
		if _, ok := sourceSet[id]; !ok {
			remove = append(remove, id)
		}
	}
	sort.Slice(add, func(i, j int) bool { return add[i] < add[j] })
	sort.Slice(remove, func(i, j int) bool { return remove[i] < remove[j] })
	sort.Slice(overlap, func(i, j int) bool { return overlap[i] < overlap[j] })
	if add == nil {
		add = []int64{}
	}
	if remove == nil {
		remove = []int64{}
	}
	if overlap == nil {
		overlap = []int64{}
	}
	return add, remove, overlap
}

func pausedAccountIDs(members []SmartScheduleAccountMember) []int64 {
	out := make([]int64, 0)
	for _, member := range members {
		if member.AccountID > 0 && member.Paused {
			out = append(out, member.AccountID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func copyFromEnabledDelta(sourceEnabled, targetEnabled bool) string {
	if sourceEnabled == targetEnabled {
		return SmartScheduleCopyEnabledUnchanged
	}
	if sourceEnabled {
		return SmartScheduleCopyEnabledEnable
	}
	return SmartScheduleCopyEnabledDisable
}

func smartScheduleMemberIDs(members []SmartScheduleAccountMember) []int64 {
	ids := make([]int64, 0, len(members))
	seen := map[int64]struct{}{}
	for _, member := range members {
		if member.AccountID <= 0 {
			continue
		}
		if _, ok := seen[member.AccountID]; ok {
			continue
		}
		seen[member.AccountID] = struct{}{}
		ids = append(ids, member.AccountID)
	}
	return ids
}

func smartSchedulePolicyAccountIDs(policy *SmartSchedulePlatformPolicy) []int64 {
	if policy == nil || len(policy.AccountIDs) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(policy.AccountIDs))
	for id := range policy.AccountIDs {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
