package service

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// smartScheduleAccountReader is the account lookup used when validating pool members.
type smartScheduleAccountReader interface {
	GetByIDs(ctx context.Context, ids []int64) ([]*Account, error)
}

// pairConcurrencyReader hydrates live user×account occupancy for the admin pool table.
type pairConcurrencyReader interface {
	GetAccountUserConcurrencyBatch(ctx context.Context, accountIDs []int64, userID int64) (map[int64]int, error)
}

// UserSmartScheduleService is the admin write/read surface for user smart schedule.
type UserSmartScheduleService struct {
	repo             UserSmartScheduleRepository
	cache            UserSmartScheduleCache
	accountRepo      smartScheduleAccountReader
	qualityLiveCache AccountQualityLiveCache
	pairConcurrency  pairConcurrencyReader
}

func NewUserSmartScheduleService(
	repo UserSmartScheduleRepository,
	cache UserSmartScheduleCache,
	accountRepo smartScheduleAccountReader,
	qualityLiveCache AccountQualityLiveCache,
	pairConcurrency pairConcurrencyReader,
) *UserSmartScheduleService {
	return &UserSmartScheduleService{
		repo:             repo,
		cache:            cache,
		accountRepo:      accountRepo,
		qualityLiveCache: qualityLiveCache,
		pairConcurrency:  pairConcurrency,
	}
}

func (s *UserSmartScheduleService) Lookup(ctx context.Context, userID int64) *UserSmartScheduleBundle {
	if s == nil || s.cache == nil {
		return nil
	}
	return s.cache.Lookup(ctx, userID)
}

func (s *UserSmartScheduleService) CooldownActive(ctx context.Context, accountID, userID int64, now time.Time) bool {
	if s == nil || s.cache == nil {
		return false
	}
	return s.cache.CooldownActive(ctx, accountID, userID, now)
}

func (s *UserSmartScheduleService) StartCooldown(ctx context.Context, accountID, userID int64, minutes int, now time.Time) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.StartCooldown(ctx, accountID, userID, minutes, now)
}

func (s *UserSmartScheduleService) GetPairQuality(ctx context.Context, accountID, userID int64) *PairQualityLive {
	if s == nil || s.cache == nil {
		return nil
	}
	return s.cache.GetPairQuality(ctx, accountID, userID)
}

func (s *UserSmartScheduleService) IsProbing(ctx context.Context, accountID, userID int64) bool {
	if s == nil || s.cache == nil {
		return false
	}
	return s.cache.IsProbing(ctx, accountID, userID)
}

func (s *UserSmartScheduleService) MarkProbing(ctx context.Context, accountID, userID int64) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.MarkProbing(ctx, accountID, userID)
}

func (s *UserSmartScheduleService) ClearProbing(ctx context.Context, accountID, userID int64) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.ClearProbing(ctx, accountID, userID)
}

func (s *UserSmartScheduleService) GraduateProbing(ctx context.Context, accountID, userID int64) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.GraduateProbing(ctx, accountID, userID)
}

func (s *UserSmartScheduleService) ObservePairCompletion(ctx context.Context, obs PairQualityObservation) {
	if s == nil || s.cache == nil || obs.AccountID <= 0 || obs.UserID <= 0 {
		return
	}
	bundle := s.Lookup(ctx, obs.UserID)
	if bundle == nil {
		return
	}
	var policy *SmartSchedulePlatformPolicy
	for _, candidate := range bundle.Policies {
		if candidate != nil && candidate.HasAccount(obs.AccountID) {
			policy = candidate
			break
		}
	}
	if policy == nil || !policy.Enabled || policy.MemberCount() == 0 {
		return
	}
	if policy.IsPaused(obs.AccountID) {
		return
	}
	now := time.Now().UTC()
	if s.cache.CooldownActive(ctx, obs.AccountID, obs.UserID, now) {
		return
	}
	live := s.cache.IngestPairQuality(ctx, obs.AccountID, obs.UserID, policy.WindowN(), obs.Success, obs.FirstTokenMs)
	stats := loadLiveQualityForAdmission(ctx, s.qualityLiveCache, &Account{ID: obs.AccountID}, true)
	if UserQualityResumeActive(stats, obs.UserID, now) {
		return
	}
	evaluateSmartSchedulePairQuality(ctx, s.cache, obs.AccountID, obs.UserID, policy, live, now)
}

func (s *UserSmartScheduleService) Get(ctx context.Context, userID int64) (*UserSmartScheduleView, error) {
	if s == nil || s.repo == nil {
		return emptySmartScheduleView(userID), nil
	}
	bundle, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	view := bundleToView(userID, bundle)
	s.hydrateAccountPriority(ctx, view)
	s.hydratePairCurrent(ctx, userID, view)
	s.hydratePairCooldown(ctx, userID, view)
	s.hydratePairProbing(ctx, userID, view)
	s.hydratePairQuality(ctx, userID, view)
	view.DefaultPlatform = pickDefaultSmartSchedulePlatform(view)
	return view, nil
}

const smartScheduleSummaryMaxBatch = 200

func (s *UserSmartScheduleService) ListSummaries(ctx context.Context, userIDs []int64) (map[string]UserSmartScheduleSummary, error) {
	if len(userIDs) > smartScheduleSummaryMaxBatch {
		userIDs = userIDs[:smartScheduleSummaryMaxBatch]
	}
	out := make(map[string]UserSmartScheduleSummary, len(userIDs))
	for _, userID := range userIDs {
		out[strconv.FormatInt(userID, 10)] = emptySmartScheduleSummary()
	}
	if s == nil || s.repo == nil || len(userIDs) == 0 {
		return out, nil
	}
	bundles, err := s.repo.ListByUsers(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	for userID, bundle := range bundles {
		out[strconv.FormatInt(userID, 10)] = summarizeSmartScheduleBundle(bundle)
	}
	return out, nil
}

func (s *UserSmartScheduleService) PutPlatform(ctx context.Context, userID int64, platform string, write SmartSchedulePlatformWrite) (*UserSmartScheduleView, error) {
	platform = normalizeSmartSchedulePlatform(platform)
	if !IsAllowedSmartSchedulePlatform(platform) {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_PLATFORM", "invalid platform")
	}
	normalized, err := normalizeSmartScheduleWrite(write)
	if err != nil {
		return nil, err
	}
	normalized.Accounts, err = s.sanitizePoolMembers(ctx, userID, platform, normalized.Accounts)
	if err != nil {
		return nil, err
	}
	if normalized.Enabled && len(normalized.Accounts) == 0 {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_EMPTY_POOL", "cannot enable smart schedule with an empty account pool")
	}
	if len(normalized.Accounts) == 0 {
		normalized.Enabled = false
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable")
	}
	if err := s.repo.ReplacePlatform(ctx, userID, platform, normalized); err != nil {
		return nil, err
	}
	if s.cache != nil {
		_ = s.cache.Invalidate(ctx, userID)
	}
	return s.Get(ctx, userID)
}

func (s *UserSmartScheduleService) PatchSortOrders(ctx context.Context, userID int64, platform string, orders []SmartScheduleSortAssignment) (*UserSmartScheduleView, error) {
	platform = normalizeSmartSchedulePlatform(platform)
	if !IsAllowedSmartSchedulePlatform(platform) {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_PLATFORM", "invalid platform")
	}
	normalized, err := normalizeSmartScheduleSortAssignments(orders)
	if err != nil {
		return nil, err
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable")
	}
	bundle, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	policy := bundle.Policy(platform)
	if policy == nil || policy.MemberCount() == 0 {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account is not in this platform pool")
	}
	for _, order := range normalized {
		if !policy.HasAccount(order.AccountID) {
			return nil, infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account is not in this platform pool")
		}
	}
	if err := s.repo.UpdateSortOrders(ctx, userID, platform, normalized); err != nil {
		return nil, err
	}
	if s.cache != nil {
		_ = s.cache.Invalidate(ctx, userID)
	}
	return s.Get(ctx, userID)
}

func (s *UserSmartScheduleService) CopyPlatform(ctx context.Context, userID int64, toPlatform, fromPlatform string) (*UserSmartScheduleView, error) {
	toPlatform = normalizeSmartSchedulePlatform(toPlatform)
	fromPlatform = normalizeSmartSchedulePlatform(fromPlatform)
	if !IsAllowedSmartSchedulePlatform(toPlatform) || !IsAllowedSmartSchedulePlatform(fromPlatform) {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_PLATFORM", "invalid platform")
	}
	if toPlatform == fromPlatform {
		return s.Get(ctx, userID)
	}
	view, err := s.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	from := view.Platforms[fromPlatform]
	to := view.Platforms[toPlatform]
	write := SmartSchedulePlatformWrite{
		Enabled:                  from.Enabled,
		QualityMaxP50TTFTMs:      from.QualityMaxP50TTFTMs,
		QualityMinSuccessRate:    from.QualityMinSuccessRate,
		QualityWindowSamples:     from.QualityWindowSamples,
		QualityWindowN:           from.QualityWindowN,
		QualityMinSuccessSamples: from.QualityMinSuccessSamples,
		QualityMinTTFTSamples:    from.QualityMinTTFTSamples,
		QualityCondition:         from.QualityCondition,
		CooldownMinutes:          from.CooldownMinutes,
		Accounts:                 to.Accounts,
	}
	if write.Enabled && len(write.Accounts) == 0 {
		write.Enabled = false
	}
	return s.PutPlatform(ctx, userID, toPlatform, write)
}

func ParsePairAdmissionState(raw string) (string, error) {
	state := strings.ToLower(strings.TrimSpace(raw))
	if state == "" {
		return PairAdmissionResumed, nil
	}
	switch state {
	case PairAdmissionPaused, PairAdmissionCooling, PairAdmissionProbing, PairAdmissionResumed, PairAdmissionSelectable:
		return state, nil
	default:
		return "", infraerrors.BadRequest("SMART_SCHEDULE_ADMISSION_INVALID", "state must be paused, cooling, probing, resumed, or selectable")
	}
}

func (s *UserSmartScheduleService) ResumePair(ctx context.Context, accountID, userID int64) error {
	_, err := s.SetPairAdmission(ctx, accountID, userID, PairAdmissionResumed)
	return err
}

func (s *UserSmartScheduleService) SetPairAdmission(ctx context.Context, accountID, userID int64, state string) (*PairAdmissionResult, error) {
	if accountID <= 0 || userID <= 0 {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_RESUME_INVALID", "account_id and user_id are required")
	}
	parsed, err := ParsePairAdmissionState(state)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	result := &PairAdmissionResult{AccountID: accountID, UserID: userID, State: parsed}
	if parsed != PairAdmissionProbing {
		s.clearProbeMark(ctx, accountID, userID)
	}
	switch parsed {
	case PairAdmissionPaused:
		if s != nil && s.cache != nil {
			if err := s.cache.ClearCooldown(ctx, accountID, userID); err != nil {
				return nil, err
			}
		}
		if s != nil && s.qualityLiveCache != nil {
			if err := s.qualityLiveCache.ClearUserResume(ctx, accountID, userID); err != nil {
				return nil, err
			}
		}
	case PairAdmissionCooling:
		until, err := s.forcePairCooldown(ctx, accountID, userID, now)
		if err != nil {
			return nil, err
		}
		result.CooldownUntil = &until
		if s != nil && s.qualityLiveCache != nil {
			if err := s.qualityLiveCache.ClearUserResume(ctx, accountID, userID); err != nil {
				return nil, err
			}
		}
	case PairAdmissionProbing:
		if s != nil && s.cache != nil {
			if err := s.cache.ClearCooldown(ctx, accountID, userID); err != nil {
				return nil, err
			}
			s.cache.ZeroPairQuality(ctx, accountID, userID, "")
			s.cache.MarkProbing(ctx, accountID, userID)
		}
		if s != nil && s.qualityLiveCache != nil {
			if err := s.qualityLiveCache.ClearUserResume(ctx, accountID, userID); err != nil {
				return nil, err
			}
		}
		result.Probing = true
		cap := s.probeCapForPair(ctx, accountID, userID)
		result.ProbeCap = &cap
	case PairAdmissionSelectable:
		if s != nil && s.cache != nil {
			if err := s.cache.ClearCooldown(ctx, accountID, userID); err != nil {
				return nil, err
			}
			s.cache.ZeroPairQuality(ctx, accountID, userID, PairQualityEventSelectable)
		}
		if s != nil && s.qualityLiveCache != nil {
			if err := s.qualityLiveCache.ClearUserResume(ctx, accountID, userID); err != nil {
				return nil, err
			}
		}
	default:
		result.State = PairAdmissionResumed
		if s != nil && s.cache != nil {
			if err := s.cache.ClearCooldown(ctx, accountID, userID); err != nil {
				return nil, err
			}
		}
		if s != nil && s.cache != nil {
			s.cache.ZeroPairQuality(ctx, accountID, userID, PairQualityEventResumed)
		}
		if s != nil && s.qualityLiveCache != nil {
			if err := s.qualityLiveCache.MarkUserResume(ctx, accountID, userID); err != nil {
				return nil, err
			}
		}
	}
	if err := s.setMemberPaused(ctx, userID, accountID, parsed == PairAdmissionPaused); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *UserSmartScheduleService) clearProbeMark(ctx context.Context, accountID, userID int64) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.ClearProbing(ctx, accountID, userID)
}

func (s *UserSmartScheduleService) probeCapForPair(ctx context.Context, accountID, userID int64) int {
	n := DefaultSmartScheduleWindowN
	memberCap := 0
	if s == nil || s.cache == nil || accountID <= 0 || userID <= 0 {
		return ProbeInFlightCap(n, memberCap)
	}
	bundle := s.cache.Lookup(ctx, userID)
	if bundle == nil {
		return ProbeInFlightCap(n, memberCap)
	}
	if s.accountRepo != nil {
		accounts, err := s.accountRepo.GetByIDs(ctx, []int64{accountID})
		if err == nil && len(accounts) > 0 && accounts[0] != nil {
			if policy := bundle.Policy(accounts[0].Platform); policy != nil {
				return ProbeInFlightCap(policy.WindowN(), policy.PairCap(accountID))
			}
		}
	}
	for _, policy := range bundle.Policies {
		if policy != nil && policy.HasAccount(accountID) {
			return ProbeInFlightCap(policy.WindowN(), policy.PairCap(accountID))
		}
	}
	return ProbeInFlightCap(n, memberCap)
}

func (s *UserSmartScheduleService) setMemberPaused(ctx context.Context, userID, accountID int64, paused bool) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if err := s.repo.SetMemberPaused(ctx, userID, accountID, paused); err != nil {
		return err
	}
	if s.cache != nil {
		return s.cache.ApplyMemberPaused(ctx, userID, accountID, paused)
	}
	return nil
}

func (s *UserSmartScheduleService) forcePairCooldown(ctx context.Context, accountID, userID int64, now time.Time) (time.Time, error) {
	minutes := DefaultSmartScheduleCooldownMinutes
	if s != nil && s.accountRepo != nil {
		accounts, err := s.accountRepo.GetByIDs(ctx, []int64{accountID})
		if err == nil && len(accounts) > 0 && accounts[0] != nil && s.cache != nil {
			if bundle := s.cache.Lookup(ctx, userID); bundle != nil {
				if policy := bundle.Policy(accounts[0].Platform); policy != nil {
					minutes = ClampSmartScheduleCooldownMinutes(policy.CooldownMinutes)
				}
			}
		}
	}
	if s != nil && s.cache != nil {
		return s.cache.SetCooldown(ctx, accountID, userID, minutes, now)
	}
	return now.Add(time.Duration(minutes) * time.Minute), nil
}

func (s *UserSmartScheduleService) currentPoolAccountIDs(ctx context.Context, userID int64, platform string) map[int64]struct{} {
	if s == nil || s.repo == nil || userID <= 0 {
		return nil
	}
	bundle, err := s.repo.ListByUser(ctx, userID)
	if err != nil || bundle == nil {
		return nil
	}
	policy := bundle.Policy(platform)
	if policy == nil || len(policy.AccountIDs) == 0 {
		return nil
	}
	return policy.AccountIDs
}

// sanitizePoolMembers keeps live accounts and strips already-in-pool IDs that
// GetByIDs cannot load (soft-deleted ghosts). Newly added unknown IDs still
// fail so a typo cannot silently disappear.
func (s *UserSmartScheduleService) sanitizePoolMembers(ctx context.Context, userID int64, platform string, members []SmartScheduleAccountMember) ([]SmartScheduleAccountMember, error) {
	if len(members) == 0 {
		return members, nil
	}
	if s == nil || s.accountRepo == nil {
		return nil, infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable")
	}
	ids := make([]int64, 0, len(members))
	seen := map[int64]struct{}{}
	for _, member := range members {
		if member.AccountID <= 0 {
			return nil, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_ACCOUNT", "invalid account id")
		}
		if _, ok := seen[member.AccountID]; ok {
			return nil, infraerrors.BadRequest("SMART_SCHEDULE_DUPLICATE_ACCOUNT", "duplicate account in pool")
		}
		seen[member.AccountID] = struct{}{}
		ids = append(ids, member.AccountID)
	}
	accounts, err := s.accountRepo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*Account, len(accounts))
	for _, acc := range accounts {
		if acc != nil {
			byID[acc.ID] = acc
		}
	}
	existing := s.currentPoolAccountIDs(ctx, userID, platform)
	kept := make([]SmartScheduleAccountMember, 0, len(members))
	for _, member := range members {
		acc := byID[member.AccountID]
		if acc == nil {
			if _, wasInPool := existing[member.AccountID]; wasInPool {
				continue
			}
			return nil, infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account not found")
		}
		if normalizeSmartSchedulePlatform(acc.Platform) != platform {
			return nil, infraerrors.BadRequest("SMART_SCHEDULE_PLATFORM_MISMATCH", "account platform does not match the selected tab")
		}
		kept = append(kept, member)
	}
	return kept, nil
}

func normalizeSmartScheduleWrite(write SmartSchedulePlatformWrite) (SmartSchedulePlatformWrite, error) {
	if write.CooldownMinutes <= 0 {
		write.CooldownMinutes = DefaultSmartScheduleCooldownMinutes
	}
	if write.CooldownMinutes < MinSmartScheduleCooldownMinutes || write.CooldownMinutes > MaxSmartScheduleCooldownMinutes {
		return SmartSchedulePlatformWrite{}, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_COOLDOWN", "cooldown_minutes must be between 1 and 1440")
	}
	if write.QualityMaxP50TTFTMs != nil && *write.QualityMaxP50TTFTMs < 1 {
		return SmartSchedulePlatformWrite{}, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", "quality_max_p50_ttft_ms must be >= 1")
	}
	if write.QualityMinSuccessRate != nil && (*write.QualityMinSuccessRate <= 0 || *write.QualityMinSuccessRate > 1) {
		return SmartSchedulePlatformWrite{}, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", "quality_min_success_rate must be in (0,1]")
	}
	if write.QualityWindowSamples == nil && write.QualityWindowN != nil {
		write.QualityWindowSamples = write.QualityWindowN
	}
	if write.QualityWindowSamples != nil && (*write.QualityWindowSamples < MinSmartScheduleWindowN || *write.QualityWindowSamples > MaxSmartScheduleWindowN) {
		return SmartSchedulePlatformWrite{}, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", "quality_window_samples must be between 1 and 100")
	}
	if write.QualityMinSuccessSamples != nil && *write.QualityMinSuccessSamples < 1 {
		return SmartSchedulePlatformWrite{}, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", "quality_min_success_samples must be >= 1")
	}
	if write.QualityMinTTFTSamples != nil && *write.QualityMinTTFTSamples < 1 {
		return SmartSchedulePlatformWrite{}, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", "quality_min_ttft_samples must be >= 1")
	}
	if write.QualityCondition != nil {
		cond := strings.ToLower(strings.TrimSpace(*write.QualityCondition))
		if cond == "" {
			write.QualityCondition = nil
		} else if cond != QualityHardCloseConditionOr && cond != QualityHardCloseConditionAnd {
			return SmartSchedulePlatformWrite{}, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", "quality_condition must be or or and")
		} else {
			write.QualityCondition = &cond
		}
	}
	if !qualityGateHasConfiguredColumn(write.QualityMaxP50TTFTMs, write.QualityMinSuccessRate, write.QualityMinSuccessSamples, write.QualityMinTTFTSamples, write.QualityCondition) {
		write.QualityMaxP50TTFTMs = nil
		write.QualityMinSuccessRate = nil
		write.QualityWindowSamples = nil
		write.QualityWindowN = nil
		write.QualityMinSuccessSamples = nil
		write.QualityMinTTFTSamples = nil
		write.QualityCondition = nil
	} else {
		n := NormalizeSmartScheduleWindowN(write.QualityWindowSamples, write.QualityMinSuccessSamples, write.QualityMinTTFTSamples)
		write.QualityWindowSamples, write.QualityMinSuccessSamples, write.QualityMinTTFTSamples = EchoSmartScheduleWindowN(n)
		write.QualityWindowN = write.QualityWindowSamples
	}
	outMembers := make([]SmartScheduleAccountMember, 0, len(write.Accounts))
	seen := map[int64]struct{}{}
	for _, member := range write.Accounts {
		if member.AccountID <= 0 {
			continue
		}
		if _, ok := seen[member.AccountID]; ok {
			continue
		}
		seen[member.AccountID] = struct{}{}
		if member.MaxConcurrency != nil && *member.MaxConcurrency < 1 {
			member.MaxConcurrency = nil
		}
		member.CurrentConcurrency = 0
		member.CooldownUntil = nil
		member.Priority = 0
		member.Paused = false
		outMembers = append(outMembers, member)
	}
	write.Accounts = outMembers
	return write, nil
}

func normalizeSmartScheduleSortAssignments(orders []SmartScheduleSortAssignment) ([]SmartScheduleSortAssignment, error) {
	if len(orders) == 0 {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_SORT_ORDER", "accounts is required")
	}
	out := make([]SmartScheduleSortAssignment, 0, len(orders))
	seen := map[int64]struct{}{}
	for _, order := range orders {
		if order.AccountID <= 0 {
			return nil, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_ACCOUNT", "invalid account id")
		}
		if _, ok := seen[order.AccountID]; ok {
			return nil, infraerrors.BadRequest("SMART_SCHEDULE_DUPLICATE_ACCOUNT", "duplicate account in pool")
		}
		seen[order.AccountID] = struct{}{}
		out = append(out, SmartScheduleSortAssignment{AccountID: order.AccountID, SortOrder: order.SortOrder})
	}
	return out, nil
}

func emptySmartScheduleView(userID int64) *UserSmartScheduleView {
	return bundleToView(userID, nil)
}

// hydrateAccountPriority fills read-only Priority from live accounts.priority.
// Writes ignore this field. It is never copied from membership sort_order.
func (s *UserSmartScheduleService) hydrateAccountPriority(ctx context.Context, view *UserSmartScheduleView) {
	if s == nil || s.accountRepo == nil || view == nil || len(view.Platforms) == 0 {
		return
	}
	ids := make([]int64, 0)
	seen := make(map[int64]struct{})
	for _, platform := range view.Platforms {
		for _, member := range platform.Accounts {
			if member.AccountID <= 0 {
				continue
			}
			if _, ok := seen[member.AccountID]; ok {
				continue
			}
			seen[member.AccountID] = struct{}{}
			ids = append(ids, member.AccountID)
		}
	}
	if len(ids) == 0 {
		return
	}
	accounts, err := s.accountRepo.GetByIDs(ctx, ids)
	if err != nil {
		return
	}
	byID := make(map[int64]*Account, len(accounts))
	for _, acc := range accounts {
		if acc != nil {
			byID[acc.ID] = acc
		}
	}
	dropMissingSmartScheduleMembers(view, byID)
}

// dropMissingSmartScheduleMembers removes IDs that GetByIDs did not load
// (soft-deleted) and turns an emptied platform off in the admin view only.
func dropMissingSmartScheduleMembers(view *UserSmartScheduleView, byID map[int64]*Account) {
	if view == nil || len(view.Platforms) == 0 {
		return
	}
	for platformKey, platform := range view.Platforms {
		kept := make([]SmartScheduleAccountMember, 0, len(platform.Accounts))
		for _, member := range platform.Accounts {
			acc := byID[member.AccountID]
			if acc == nil {
				continue
			}
			member.Priority = acc.Priority
			kept = append(kept, member)
		}
		platform.Accounts = kept
		if len(kept) == 0 {
			platform.Enabled = false
		}
		view.Platforms[platformKey] = platform
	}
}

// hydratePairCurrent fills CurrentConcurrency for every pool member from
// concurrency:account_user:{accountID}:{userID}. Uncapped members are included
// so the admin badge can show this user's occupancy; this does not acquire
// slots or change pair-cap enforcement.
func (s *UserSmartScheduleService) hydratePairCurrent(ctx context.Context, userID int64, view *UserSmartScheduleView) {
	if s == nil || s.pairConcurrency == nil || view == nil || userID <= 0 || len(view.Platforms) == 0 {
		return
	}
	ids := make([]int64, 0)
	seen := make(map[int64]struct{})
	for _, platform := range view.Platforms {
		for _, member := range platform.Accounts {
			if member.AccountID <= 0 {
				continue
			}
			if _, ok := seen[member.AccountID]; ok {
				continue
			}
			seen[member.AccountID] = struct{}{}
			ids = append(ids, member.AccountID)
		}
	}
	if len(ids) == 0 {
		return
	}
	counts, err := s.pairConcurrency.GetAccountUserConcurrencyBatch(ctx, ids, userID)
	if err != nil || counts == nil {
		return
	}
	for platformKey, platform := range view.Platforms {
		for i := range platform.Accounts {
			platform.Accounts[i].CurrentConcurrency = counts[platform.Accounts[i].AccountID]
		}
		view.Platforms[platformKey] = platform
	}
}

func (s *UserSmartScheduleService) hydratePairCooldown(ctx context.Context, userID int64, view *UserSmartScheduleView) {
	if s == nil || s.cache == nil || view == nil || userID <= 0 || len(view.Platforms) == 0 {
		return
	}
	ids := make([]int64, 0)
	seen := make(map[int64]struct{})
	for _, platform := range view.Platforms {
		for _, member := range platform.Accounts {
			if member.AccountID <= 0 {
				continue
			}
			if _, ok := seen[member.AccountID]; ok {
				continue
			}
			seen[member.AccountID] = struct{}{}
			ids = append(ids, member.AccountID)
		}
	}
	if len(ids) == 0 {
		return
	}
	untilByAccount := s.cache.GetCooldownUntilBatch(ctx, ids, userID, time.Now().UTC())
	if len(untilByAccount) == 0 {
		return
	}
	for platformKey, platform := range view.Platforms {
		for i := range platform.Accounts {
			until, ok := untilByAccount[platform.Accounts[i].AccountID]
			if !ok || until.IsZero() {
				continue
			}
			copied := until
			platform.Accounts[i].CooldownUntil = &copied
		}
		view.Platforms[platformKey] = platform
	}
}

func (s *UserSmartScheduleService) hydratePairProbing(ctx context.Context, userID int64, view *UserSmartScheduleView) {
	if s == nil || s.cache == nil || view == nil || userID <= 0 || len(view.Platforms) == 0 {
		return
	}
	ids := make([]int64, 0)
	seen := make(map[int64]struct{})
	for _, platform := range view.Platforms {
		for _, member := range platform.Accounts {
			if member.AccountID <= 0 {
				continue
			}
			if _, ok := seen[member.AccountID]; ok {
				continue
			}
			seen[member.AccountID] = struct{}{}
			ids = append(ids, member.AccountID)
		}
	}
	if len(ids) == 0 {
		return
	}
	probing := s.cache.IsProbingBatch(ctx, ids, userID)
	if len(probing) == 0 {
		return
	}
	for platformKey, platform := range view.Platforms {
		n := viewPolicyN(&platform)
		for i := range platform.Accounts {
			if !probing[platform.Accounts[i].AccountID] {
				continue
			}
			if platform.Accounts[i].Paused || platform.Accounts[i].CooldownUntil != nil {
				continue
			}
			platform.Accounts[i].Probing = true
			memberCap := 0
			if platform.Accounts[i].MaxConcurrency != nil {
				memberCap = *platform.Accounts[i].MaxConcurrency
			}
			cap := ProbeInFlightCap(n, memberCap)
			platform.Accounts[i].ProbeCap = &cap
		}
		view.Platforms[platformKey] = platform
	}
}

func (s *UserSmartScheduleService) hydratePairQuality(ctx context.Context, userID int64, view *UserSmartScheduleView) {
	if s == nil || s.cache == nil || view == nil || userID <= 0 || len(view.Platforms) == 0 {
		return
	}
	ids := make([]int64, 0)
	seen := make(map[int64]struct{})
	for _, platform := range view.Platforms {
		for _, member := range platform.Accounts {
			if member.AccountID <= 0 {
				continue
			}
			if _, ok := seen[member.AccountID]; ok {
				continue
			}
			seen[member.AccountID] = struct{}{}
			ids = append(ids, member.AccountID)
		}
	}
	if len(ids) == 0 {
		return
	}
	lives := s.cache.GetPairQualityBatch(ctx, ids, userID)
	now := time.Now().UTC()
	for platformKey, platform := range view.Platforms {
		n := DefaultSmartScheduleWindowN
		if policy := viewPolicyN(&platform); policy > 0 {
			n = policy
		}
		gate := QualityHardCloseSettings{}
		if platform.QualityMaxP50TTFTMs != nil || platform.QualityMinSuccessRate != nil {
			gate = fillUserQualityGateDefaults(QualityHardCloseSettings{
				MaxP50TTFTMs:      platform.QualityMaxP50TTFTMs,
				MinSuccessRate:    platform.QualityMinSuccessRate,
				MinSuccessSamples: n,
				MinTTFTSamples:    n,
				Condition:         derefString(platform.QualityCondition),
			})
		}
		for i := range platform.Accounts {
			live := lives[platform.Accounts[i].AccountID]
			viewSnap := SmartSchedulePairQualityView{N: n}
			if live != nil {
				viewSnap = live.View()
				viewSnap.N = n
			}
			viewSnap = aliasPairQualityView(viewSnap)
			platform.Accounts[i].PairQuality = &viewSnap
			if platform.Accounts[i].Paused || platform.Accounts[i].CooldownUntil != nil {
				continue
			}
			stats := loadLiveQualityForAdmission(ctx, s.qualityLiveCache, &Account{ID: platform.Accounts[i].AccountID}, true)
			if UserQualityResumeActive(stats, userID, now) {
				continue
			}
			if !qualityGateHasMetric(gate) {
				continue
			}
			blocked, _ := EvaluateAccountQualityHardClose(live.ToAccountQualityStats(), gate, false)
			platform.Accounts[i].WillCool = blocked
		}
		view.Platforms[platformKey] = platform
	}
}

func viewPolicyN(platform *SmartSchedulePlatformView) int {
	if platform == nil {
		return DefaultSmartScheduleWindowN
	}
	return NormalizeSmartScheduleWindowN(platform.QualityWindowSamples, platform.QualityMinSuccessSamples, platform.QualityMinTTFTSamples)
}

func (s *UserSmartScheduleService) GetPairQualityDetail(ctx context.Context, userID int64, platform string, accountID int64) (*SmartSchedulePairQualityDetail, error) {
	if userID <= 0 || accountID <= 0 {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_RESUME_INVALID", "account_id and user_id are required")
	}
	platform = normalizeSmartSchedulePlatform(platform)
	if !IsAllowedSmartSchedulePlatform(platform) {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_PLATFORM", "invalid platform")
	}
	view, err := s.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	row := view.Platforms[platform]
	found := false
	n := viewPolicyN(&row)
	for _, member := range row.Accounts {
		if member.AccountID == accountID {
			found = true
			if member.PairQuality != nil && member.PairQuality.N > 0 {
				n = member.PairQuality.N
			}
			break
		}
	}
	if !found {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account is not in this platform pool")
	}
	detail := &SmartSchedulePairQualityDetail{
		AccountID: accountID,
		UserID:    userID,
		N:         n,
		Live:      aliasPairQualityView(SmartSchedulePairQualityView{N: n}),
		Current:   aliasPairQualityView(SmartSchedulePairQualityView{N: n}),
		Snapshots: []PairQualitySnapshot{},
		Events:    []PairQualityEvent{},
	}
	if s.cache != nil {
		if live := s.cache.GetPairQuality(ctx, accountID, userID); live != nil {
			detail.Live = live.View()
			detail.Live.N = n
			detail.Live = aliasPairQualityView(detail.Live)
		}
		detail.Current = detail.Live
		if snaps := s.cache.ListPairQualitySnapshots(ctx, accountID, userID, 200); snaps != nil {
			detail.Snapshots = make([]PairQualitySnapshot, 0, len(snaps))
			for _, snap := range snaps {
				snap.N = n
				detail.Snapshots = append(detail.Snapshots, aliasPairQualitySnapshot(snap))
			}
		}
		if events := s.cache.ListPairQualityEvents(ctx, accountID, userID, 200); events != nil {
			detail.Events = make([]PairQualityEvent, 0, len(events))
			for _, event := range events {
				detail.Events = append(detail.Events, aliasPairQualityEvent(event))
			}
		}
	}
	return detail, nil
}

func (s *UserSmartScheduleService) GetPairQualityDetailForAccount(ctx context.Context, userID, accountID int64) (*SmartSchedulePairQualityDetail, error) {
	if userID <= 0 || accountID <= 0 {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_RESUME_INVALID", "account_id and user_id are required")
	}
	view, err := s.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	for platform, row := range view.Platforms {
		for _, member := range row.Accounts {
			if member.AccountID == accountID {
				return s.GetPairQualityDetail(ctx, userID, platform, accountID)
			}
		}
	}
	return nil, infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account is not in this platform pool")
}

func (s *UserSmartScheduleService) GetPairQualityBatch(ctx context.Context, userID int64, accountIDs []int64) (*SmartSchedulePairQualityBatch, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_RESUME_INVALID", "user_id is required")
	}
	out := &SmartSchedulePairQualityBatch{Pairs: map[string]SmartSchedulePairQualityView{}}
	view, err := s.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	wanted := map[int64]struct{}{}
	for _, id := range accountIDs {
		if id > 0 {
			wanted[id] = struct{}{}
		}
	}
	for _, platform := range view.Platforms {
		for _, member := range platform.Accounts {
			if member.AccountID <= 0 {
				continue
			}
			if len(wanted) > 0 {
				if _, ok := wanted[member.AccountID]; !ok {
					continue
				}
			}
			if member.PairQuality != nil {
				out.Pairs[strconv.FormatInt(member.AccountID, 10)] = *member.PairQuality
			}
		}
	}
	return out, nil
}

func pickDefaultSmartSchedulePlatform(view *UserSmartScheduleView) string {
	if view == nil || len(view.Platforms) == 0 {
		return PlatformAnthropic
	}
	type candidate struct {
		platform string
		members  int
		updated  time.Time
	}
	var enabled, withPool []candidate
	for _, platform := range AllowedQuotaPlatforms {
		row := view.Platforms[platform]
		item := candidate{platform: platform, members: len(row.Accounts), updated: row.UpdatedAt}
		if row.Enabled && item.members > 0 {
			enabled = append(enabled, item)
		}
		if item.members > 0 {
			withPool = append(withPool, item)
		}
	}
	pool := enabled
	if len(pool) == 0 {
		pool = withPool
	}
	if len(pool) == 0 {
		return PlatformAnthropic
	}
	best := pool[0]
	for _, item := range pool[1:] {
		if item.updated.After(best.updated) {
			best = item
			continue
		}
		if item.updated.Equal(best.updated) && item.members > best.members {
			best = item
		}
	}
	return best.platform
}

func emptySmartScheduleSummary() UserSmartScheduleSummary {
	return UserSmartScheduleSummary{
		EnabledPlatforms: []string{},
		PoolCounts:       map[string]int{},
	}
}

func summarizeSmartScheduleBundle(bundle *UserSmartScheduleBundle) UserSmartScheduleSummary {
	summary := emptySmartScheduleSummary()
	if bundle == nil {
		return summary
	}
	for _, platform := range AllowedQuotaPlatforms {
		policy := bundle.Policy(platform)
		if policy == nil || !policy.Enabled || policy.MemberCount() == 0 {
			continue
		}
		summary.EnabledPlatforms = append(summary.EnabledPlatforms, platform)
		summary.PoolCounts[platform] = policy.MemberCount()
	}
	return summary
}

func bundleToView(userID int64, bundle *UserSmartScheduleBundle) *UserSmartScheduleView {
	view := &UserSmartScheduleView{
		UserID:    userID,
		Platforms: make(map[string]SmartSchedulePlatformView, len(AllowedQuotaPlatforms)),
	}
	for _, platform := range AllowedQuotaPlatforms {
		policy := (*SmartSchedulePlatformPolicy)(nil)
		if bundle != nil {
			policy = bundle.Policy(platform)
		}
		view.Platforms[platform] = policyToView(platform, policy)
	}
	return view
}

func policyToView(platform string, policy *SmartSchedulePlatformPolicy) SmartSchedulePlatformView {
	view := SmartSchedulePlatformView{
		CooldownMinutes: DefaultSmartScheduleCooldownMinutes,
		Accounts:        []SmartScheduleAccountMember{},
	}
	if policy == nil {
		return view
	}
	view.Enabled = policy.Enabled && policy.MemberCount() > 0
	view.QualityMaxP50TTFTMs = policy.QualityMaxP50TTFTMs
	view.QualityMinSuccessRate = policy.QualityMinSuccessRate
	if policy.HasQualityMetrics() || policy.QualityWindowSamples != nil || policy.QualityMinSuccessSamples != nil || policy.QualityMinTTFTSamples != nil {
		n := policy.WindowN()
		view.QualityWindowSamples, view.QualityMinSuccessSamples, view.QualityMinTTFTSamples = EchoSmartScheduleWindowN(n)
		view.QualityWindowN = view.QualityWindowSamples
	}
	view.QualityCondition = policy.QualityCondition
	if policy.CooldownMinutes >= MinSmartScheduleCooldownMinutes {
		view.CooldownMinutes = policy.CooldownMinutes
	}
	view.UpdatedAt = policy.UpdatedAt
	accountIDs := make([]int64, 0, len(policy.AccountIDs))
	for accountID := range policy.AccountIDs {
		accountIDs = append(accountIDs, accountID)
	}
	sort.Slice(accountIDs, func(i, j int) bool {
		return compareSmartScheduleMemberIDs(accountIDs[i], accountIDs[j], policy.SortOrders)
	})
	for _, accountID := range accountIDs {
		member := SmartScheduleAccountMember{AccountID: accountID, Platform: platform}
		if capN := policy.PairCap(accountID); capN >= 1 {
			copied := capN
			member.MaxConcurrency = &copied
		}
		if n, ok := policy.SortOrders[accountID]; ok {
			copied := n
			member.SortOrder = &copied
		}
		member.Paused = policy.IsPaused(accountID)
		view.Accounts = append(view.Accounts, member)
	}
	return view
}

func compareSmartScheduleMemberIDs(left, right int64, sortOrders map[int64]int) bool {
	leftOrder, leftOK := sortOrders[left]
	rightOrder, rightOK := sortOrders[right]
	if !leftOK && !rightOK {
		return left < right
	}
	if !leftOK {
		return false
	}
	if !rightOK {
		return true
	}
	if leftOrder != rightOrder {
		return leftOrder < rightOrder
	}
	return left < right
}
