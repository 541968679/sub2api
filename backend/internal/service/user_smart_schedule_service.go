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

func (s *UserSmartScheduleService) CooldownActive(ctx context.Context, accountID, userID int64, platform string, now time.Time) bool {
	if s == nil || s.cache == nil {
		return false
	}
	return s.cache.CooldownActive(ctx, accountID, userID, platform, now)
}

func (s *UserSmartScheduleService) StartCooldown(ctx context.Context, accountID, userID int64, platform string, minutes int, now time.Time) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.StartCooldown(ctx, accountID, userID, platform, minutes, now)
}

func (s *UserSmartScheduleService) StartCooldownWithReason(ctx context.Context, accountID, userID int64, platform string, minutes int, now time.Time, reason string) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.StartCooldownWithReason(ctx, accountID, userID, platform, minutes, now, reason)
}

func (s *UserSmartScheduleService) GetPairQuality(ctx context.Context, accountID, userID int64, platform string) *PairQualityLive {
	if s == nil || s.cache == nil {
		return nil
	}
	return s.cache.GetPairQuality(ctx, accountID, userID, platform)
}

func (s *UserSmartScheduleService) IsProbing(ctx context.Context, accountID, userID int64, platform string) bool {
	if s == nil || s.cache == nil {
		return false
	}
	return s.cache.IsProbing(ctx, accountID, userID, platform)
}

func (s *UserSmartScheduleService) MarkProbing(ctx context.Context, accountID, userID int64, platform string) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.MarkProbing(ctx, accountID, userID, platform)
}

func (s *UserSmartScheduleService) ClearProbing(ctx context.Context, accountID, userID int64, platform string) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.ClearProbing(ctx, accountID, userID, platform)
}

func (s *UserSmartScheduleService) GraduateProbing(ctx context.Context, accountID, userID int64, platform string) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.GraduateProbing(ctx, accountID, userID, platform)
}

func (s *UserSmartScheduleService) IsPinned(ctx context.Context, accountID, userID int64, platform string) bool {
	if s == nil || s.cache == nil {
		return false
	}
	return s.cache.IsPinned(ctx, accountID, userID, platform)
}

func (s *UserSmartScheduleService) MarkPinned(ctx context.Context, accountID, userID int64, platform string) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.MarkPinned(ctx, accountID, userID, platform)
}

func (s *UserSmartScheduleService) ClearPinned(ctx context.Context, accountID, userID int64, platform string) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.ClearPinned(ctx, accountID, userID, platform)
}

func (s *UserSmartScheduleService) PairResumeActive(ctx context.Context, accountID, userID int64, platform string, now time.Time) bool {
	if s == nil || s.cache == nil {
		return false
	}
	return s.cache.PairResumeActive(ctx, accountID, userID, platform, now)
}

func (s *UserSmartScheduleService) ObservePairCompletion(ctx context.Context, obs PairQualityObservation) {
	if s == nil || s.cache == nil || obs.AccountID <= 0 || obs.UserID <= 0 {
		return
	}
	bundle := s.Lookup(ctx, obs.UserID)
	if bundle == nil {
		return
	}
	platform := normalizeSmartSchedulePlatform(obs.Platform)
	if platform == "" {
		platform = uniqueSmartScheduleMembershipPlatform(bundle, obs.AccountID)
	}
	if platform == "" {
		return
	}
	policy := bundle.EnabledPolicy(platform)
	if policy == nil || !policy.HasAccount(obs.AccountID) {
		return
	}
	if policy.IsPaused(obs.AccountID) {
		return
	}
	now := time.Now().UTC()
	pinned := s.cache.IsPinned(ctx, obs.AccountID, obs.UserID, platform)
	if pinned {
		s.cache.IngestPairQuality(ctx, obs.AccountID, obs.UserID, platform, policy.TTFTStorageN(), policy.SuccessWindowN(), obs.Success, obs.FirstTokenMs, obs.DurationMs)
		s.ingestSoftCooldownForCoolingPeers(ctx, obs, platform, policy, now)
		return
	}
	if s.cache.CooldownActive(ctx, obs.AccountID, obs.UserID, platform, now) {
		return
	}
	live := s.cache.IngestPairQuality(ctx, obs.AccountID, obs.UserID, platform, policy.TTFTStorageN(), policy.SuccessWindowN(), obs.Success, obs.FirstTokenMs, obs.DurationMs)
	probing := s.cache.IsProbing(ctx, obs.AccountID, obs.UserID, platform)
	if pairQualityResumeBlocksEvaluate(ctx, s.cache, probing, obs.AccountID, obs.UserID, platform, now) {
		s.ingestSoftCooldownForCoolingPeers(ctx, obs, platform, policy, now)
		return
	}
	clearLeftoverPairResumeIfProbing(ctx, s.cache, probing, obs.AccountID, obs.UserID, platform, now)
	evaluateSmartSchedulePairQuality(ctx, s.cache, obs.AccountID, obs.UserID, platform, policy, live, now)
	s.ingestSoftCooldownForCoolingPeers(ctx, obs, platform, policy, now)
}

func (s *UserSmartScheduleService) ingestSoftCooldownForCoolingPeers(ctx context.Context, obs PairQualityObservation, platform string, policy *SmartSchedulePlatformPolicy, now time.Time) {
	if s == nil || s.cache == nil || policy == nil || !policy.SoftCooldown || obs.UserID <= 0 {
		return
	}
	ids := make([]int64, 0, len(policy.AccountIDs))
	for accountID := range policy.AccountIDs {
		if accountID <= 0 || accountID == obs.AccountID || policy.IsPaused(accountID) {
			continue
		}
		ids = append(ids, accountID)
	}
	if len(ids) == 0 {
		return
	}
	untilByAccount := s.cache.GetCooldownUntilBatch(ctx, ids, obs.UserID, platform, now)
	for accountID, until := range untilByAccount {
		if until.IsZero() || !until.After(now) {
			continue
		}
		if s.cache.IsPinned(ctx, accountID, obs.UserID, platform) {
			continue
		}
		if s.cache.IsCooldownHard(ctx, accountID, obs.UserID, platform) {
			continue
		}
		live := s.cache.IngestSoftCooldown(ctx, accountID, obs.UserID, platform, policy.TTFTStorageN(), policy.SuccessWindowN(), obs.Success, obs.FirstTokenMs, obs.DurationMs, policy.CooldownMinutes)
		if !softCooldownMeets(live, policy) {
			continue
		}
		s.cache.SoftEndCooldown(ctx, accountID, obs.UserID, platform, softCooldownMeetDetail(live, policy))
	}
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
	s.hydratePairResume(ctx, userID, view)
	s.hydratePairProbing(ctx, userID, view)
	s.hydratePairPinned(ctx, userID, view)
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
	if bundle, listErr := s.repo.ListByUser(ctx, userID); listErr == nil && bundle != nil {
		normalized = overlayExistingSmartScheduleWindows(write, normalized, bundle.Policy(platform))
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
		Enabled:                        from.Enabled,
		QualityMaxP50TTFTMs:            from.QualityMaxP50TTFTMs,
		QualityMaxP50DurationMs:        from.QualityMaxP50DurationMs,
		QualityMaxSlowInWindow:         from.QualityMaxSlowInWindow,
		QualityMaxConsecutiveSlow:      from.QualityMaxConsecutiveSlow,
		QualitySchedWindowN:            from.QualitySchedWindowN,
		QualitySchedMaxSlowInWindow:    from.QualitySchedMaxSlowInWindow,
		QualitySchedMaxConsecutiveSlow: from.QualitySchedMaxConsecutiveSlow,
		QualityMinSuccessRate:          from.QualityMinSuccessRate,
		QualityWindowSamples:           from.QualityWindowSamples,
		QualityWindowN:                 from.QualityWindowN,
		QualityMinSuccessSamples:       from.QualityMinSuccessSamples,
		QualityMinTTFTSamples:          from.QualityMinTTFTSamples,
		QualityCondition:               from.QualityCondition,
		CooldownMinutes:                from.CooldownMinutes,
		SoftCooldown:                   from.SoftCooldown,
		ProbeLatencyV2:                 from.ProbeLatencyV2,
		ProbeConcurrencyMode:           from.ProbeConcurrencyMode,
		ProbeConcurrency:               from.ProbeConcurrency,
		Accounts:                       to.Accounts,
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
	case PairAdmissionPaused, PairAdmissionCooling, PairAdmissionProbing, PairAdmissionResumed, PairAdmissionSelectable, PairAdmissionPinned:
		return state, nil
	default:
		return "", infraerrors.BadRequest("SMART_SCHEDULE_ADMISSION_INVALID", "state must be paused, cooling, probing, resumed, selectable, or pinned")
	}
}

func (s *UserSmartScheduleService) ResumePair(ctx context.Context, accountID, userID int64) error {
	_, err := s.SetPairAdmission(ctx, accountID, userID, PairAdmissionResumed)
	return err
}

func (s *UserSmartScheduleService) SetPairAdmission(ctx context.Context, accountID, userID int64, state string, platformHint ...string) (*PairAdmissionResult, error) {
	if accountID <= 0 || userID <= 0 {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_RESUME_INVALID", "account_id and user_id are required")
	}
	parsed, err := ParsePairAdmissionState(state)
	if err != nil {
		return nil, err
	}
	platform := ""
	if len(platformHint) > 0 {
		platform = platformHint[0]
	}
	platform = s.resolvePairAdmissionPlatform(ctx, accountID, userID, platform)
	now := time.Now().UTC()
	result := &PairAdmissionResult{AccountID: accountID, UserID: userID, State: parsed}
	if parsed != PairAdmissionProbing {
		s.clearProbeMark(ctx, accountID, userID, platform)
	}
	if parsed != PairAdmissionPinned {
		s.clearPinMark(ctx, accountID, userID, platform)
	}
	switch parsed {
	case PairAdmissionPaused:
		if s != nil && s.cache != nil {
			if err := s.cache.ClearCooldown(ctx, accountID, userID, platform); err != nil {
				return nil, err
			}
		}
		s.clearPairResume(ctx, accountID, userID, platform)
	case PairAdmissionCooling:
		until, err := s.forcePairCooldown(ctx, accountID, userID, platform, now)
		if err != nil {
			return nil, err
		}
		result.CooldownUntil = &until
		s.clearPairResume(ctx, accountID, userID, platform)
	case PairAdmissionProbing:
		if s != nil && s.cache != nil {
			if err := s.cache.ClearCooldown(ctx, accountID, userID, platform); err != nil {
				return nil, err
			}
			outcome := s.cache.EnterProbe(ctx, accountID, userID, platform)
			switch outcome {
			case ProbeAdmissionCooling:
				result.State = PairAdmissionCooling
				until := s.cache.GetCooldownUntilBatch(ctx, []int64{accountID}, userID, platform, now)[accountID]
				if !until.IsZero() {
					result.CooldownUntil = &until
				}
			case ProbeAdmissionSelectable:
				result.State = PairAdmissionSelectable
			default:
				result.Probing = true
				cap := s.probeCapForPair(ctx, accountID, userID, platform)
				result.ProbeCap = &cap
			}
		} else {
			result.Probing = true
			cap := s.probeCapForPair(ctx, accountID, userID, platform)
			result.ProbeCap = &cap
		}
		s.clearPairResume(ctx, accountID, userID, platform)
	case PairAdmissionSelectable:
		if s != nil && s.cache != nil {
			if err := s.cache.ClearCooldown(ctx, accountID, userID, platform); err != nil {
				return nil, err
			}
			s.cache.ZeroPairQuality(ctx, accountID, userID, platform, PairQualityEventSelectable)
		}
		s.clearPairResume(ctx, accountID, userID, platform)
	case PairAdmissionPinned:
		if s != nil && s.cache != nil {
			if err := s.cache.ClearCooldown(ctx, accountID, userID, platform); err != nil {
				return nil, err
			}
			s.cache.MarkPinned(ctx, accountID, userID, platform)
		}
		s.clearPairResume(ctx, accountID, userID, platform)
		result.Pinned = true
	default:
		result.State = PairAdmissionResumed
		if s != nil && s.cache != nil {
			if err := s.cache.ClearCooldown(ctx, accountID, userID, platform); err != nil {
				return nil, err
			}
		}
		if s != nil && s.cache != nil {
			s.cache.ZeroPairQuality(ctx, accountID, userID, platform, PairQualityEventResumed)
		}
		if err := s.markPairResume(ctx, accountID, userID, platform); err != nil {
			return nil, err
		}
	}
	if err := s.setMemberPaused(ctx, userID, accountID, platform, parsed == PairAdmissionPaused); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *UserSmartScheduleService) resolvePairAdmissionPlatform(ctx context.Context, accountID, userID int64, platform string) string {
	platform = normalizeSmartSchedulePlatform(platform)
	if platform != "" {
		return platform
	}
	if s != nil && s.accountRepo != nil {
		accounts, err := s.accountRepo.GetByIDs(ctx, []int64{accountID})
		if err == nil && len(accounts) > 0 && accounts[0] != nil {
			return normalizeSmartSchedulePlatform(accounts[0].Platform)
		}
	}
	if s != nil && s.cache != nil {
		if bundle := s.cache.Lookup(ctx, userID); bundle != nil {
			if found := uniqueSmartScheduleMembershipPlatform(bundle, accountID); found != "" {
				return found
			}
		}
	}
	return ""
}

func (s *UserSmartScheduleService) clearProbeMark(ctx context.Context, accountID, userID int64, platform string) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.ClearProbing(ctx, accountID, userID, platform)
}

func (s *UserSmartScheduleService) clearPinMark(ctx context.Context, accountID, userID int64, platform string) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.ClearPinned(ctx, accountID, userID, platform)
}

func (s *UserSmartScheduleService) ClearPairResume(ctx context.Context, accountID, userID int64, platform string) {
	s.clearPairResume(ctx, accountID, userID, platform)
}

func (s *UserSmartScheduleService) clearPairResume(ctx context.Context, accountID, userID int64, platform string) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.ClearPairResume(ctx, accountID, userID, platform)
}

func (s *UserSmartScheduleService) markPairResume(ctx context.Context, accountID, userID int64, platform string) error {
	if s == nil || s.cache == nil {
		return nil
	}
	return s.cache.MarkPairResume(ctx, accountID, userID, platform)
}

func (s *UserSmartScheduleService) probeCapForPair(ctx context.Context, accountID, userID int64, platform string) int {
	n := DefaultSmartScheduleWindowN
	memberCap := 0
	if s == nil || s.cache == nil || accountID <= 0 || userID <= 0 {
		return ProbeInFlightCap(n, memberCap)
	}
	bundle := s.cache.Lookup(ctx, userID)
	if bundle == nil {
		return ProbeInFlightCap(n, memberCap)
	}
	platform = normalizeSmartSchedulePlatform(platform)
	if platform != "" {
		if policy := bundle.Policy(platform); policy != nil {
			return policy.ProbeInFlightCap(policy.PairCap(accountID))
		}
	}
	return ProbeInFlightCap(n, memberCap)
}

func (s *UserSmartScheduleService) setMemberPaused(ctx context.Context, userID, accountID int64, platform string, paused bool) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if err := s.repo.SetMemberPaused(ctx, userID, accountID, platform, paused); err != nil {
		return err
	}
	if s.cache != nil {
		return s.cache.ApplyMemberPaused(ctx, userID, accountID, platform, paused)
	}
	return nil
}

func (s *UserSmartScheduleService) forcePairCooldown(ctx context.Context, accountID, userID int64, platform string, now time.Time) (time.Time, error) {
	minutes := DefaultSmartScheduleCooldownMinutes
	if s != nil && s.cache != nil {
		if bundle := s.cache.Lookup(ctx, userID); bundle != nil {
			if policy := bundle.Policy(platform); policy != nil {
				minutes = ClampSmartScheduleCooldownMinutes(policy.CooldownMinutes)
			}
		}
	}
	if s != nil && s.cache != nil {
		manualReason := formatSmartScheduleCooldownDetail(CooldownPhaseManual, "", []SmartScheduleCooldownReason{{Code: "manual", Detail: "切换到冷却"}})
		return s.cache.SetCooldownWithReason(ctx, accountID, userID, platform, minutes, now, manualReason)
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
		if !smartScheduleAccountMatchesTab(acc, platform) {
			return nil, infraerrors.BadRequest("SMART_SCHEDULE_PLATFORM_MISMATCH", "account platform does not match the selected tab")
		}
		kept = append(kept, member)
	}
	return kept, nil
}

func validateSmartScheduleWindowField(name string, value *int) error {
	if value == nil {
		return nil
	}
	if *value < MinSmartScheduleWindowN || *value > MaxSmartScheduleWindowN {
		return infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", name+" must be between 1 and 100")
	}
	return nil
}

func splitSmartScheduleWindowWrite(shared, ttft, success *int) (int, int) {
	if ttft == nil && success == nil {
		if shared != nil {
			n := ClampSmartScheduleWindowN(*shared)
			return n, n
		}
		return DefaultSmartScheduleWindowN, DefaultSmartScheduleWindowN
	}
	// One or both metric columns are present. Do not copy quality_window_n onto the other.
	resolvedTTFT := DefaultSmartScheduleWindowN
	resolvedSuccess := DefaultSmartScheduleWindowN
	if ttft != nil {
		resolvedTTFT = ClampSmartScheduleWindowN(*ttft)
	}
	if success != nil {
		resolvedSuccess = ClampSmartScheduleWindowN(*success)
	}
	return resolvedTTFT, resolvedSuccess
}

// overlayExistingSmartScheduleWindows keeps the stored N when a PUT omits that column.
// Legacy writes that only send quality_window_n still set both.
func overlayExistingSmartScheduleWindows(write, normalized SmartSchedulePlatformWrite, existing *SmartSchedulePlatformPolicy) SmartSchedulePlatformWrite {
	if existing == nil {
		return normalized
	}
	legacyShared := write.QualityMinTTFTSamples == nil && write.QualityMinSuccessSamples == nil &&
		(write.QualityWindowN != nil || write.QualityWindowSamples != nil)
	if legacyShared {
		return normalized
	}
	if write.QualityMinTTFTSamples == nil {
		n := existing.TTFTWindowN()
		normalized.QualityMinTTFTSamples = &n
	}
	if write.QualityMinSuccessSamples == nil {
		n := existing.SuccessWindowN()
		normalized.QualityMinSuccessSamples = &n
	}
	if write.QualityMaxSlowInWindow == nil {
		normalized.QualityMaxSlowInWindow = existing.QualityMaxSlowInWindow
	}
	if write.QualityMaxConsecutiveSlow == nil {
		normalized.QualityMaxConsecutiveSlow = existing.QualityMaxConsecutiveSlow
	}
	if write.QualityMaxP50DurationMs == nil {
		normalized.QualityMaxP50DurationMs = existing.QualityMaxP50DurationMs
	}
	if write.QualitySchedWindowN == nil {
		normalized.QualitySchedWindowN = existing.QualitySchedWindowN
	}
	if write.QualitySchedMaxSlowInWindow == nil {
		normalized.QualitySchedMaxSlowInWindow = existing.QualitySchedMaxSlowInWindow
	}
	if write.QualitySchedMaxConsecutiveSlow == nil {
		normalized.QualitySchedMaxConsecutiveSlow = existing.QualitySchedMaxConsecutiveSlow
	}
	if normalized.QualityMinTTFTSamples != nil && normalized.QualityMinSuccessSamples != nil {
		normalized.QualityWindowSamples, normalized.QualityWindowN = echoCompatSmartScheduleWindowN(
			*normalized.QualityMinTTFTSamples,
			*normalized.QualityMinSuccessSamples,
		)
	}
	return normalized
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
	if write.QualityMaxP50DurationMs != nil && *write.QualityMaxP50DurationMs < 1 {
		return SmartSchedulePlatformWrite{}, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", "quality_max_p50_duration_ms must be >= 1")
	}
	if write.QualityMaxSlowInWindow != nil && (*write.QualityMaxSlowInWindow < 1 || (write.QualityMinTTFTSamples != nil && *write.QualityMaxSlowInWindow > *write.QualityMinTTFTSamples)) {
		return SmartSchedulePlatformWrite{}, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", "quality_max_slow_in_window must be between 1 and N首字")
	}
	if write.QualityMaxConsecutiveSlow != nil && (*write.QualityMaxConsecutiveSlow < 1 || (write.QualityMinTTFTSamples != nil && *write.QualityMaxConsecutiveSlow > *write.QualityMinTTFTSamples)) {
		return SmartSchedulePlatformWrite{}, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", "quality_max_consecutive_slow must be between 1 and N首字")
	}
	schedN := DefaultSmartScheduleSchedN
	if write.QualitySchedWindowN != nil && *write.QualitySchedWindowN > 0 {
		schedN = ClampSmartScheduleWindowN(*write.QualitySchedWindowN)
	}
	if write.QualitySchedWindowN != nil && (*write.QualitySchedWindowN < 1 || *write.QualitySchedWindowN > MaxSmartScheduleWindowN) {
		return SmartSchedulePlatformWrite{}, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", "quality_sched_window_n must be between 1 and 100")
	}
	if write.QualitySchedMaxSlowInWindow != nil && (*write.QualitySchedMaxSlowInWindow < 1 || *write.QualitySchedMaxSlowInWindow > schedN) {
		return SmartSchedulePlatformWrite{}, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", "quality_sched_max_slow_in_window must be between 1 and sched N")
	}
	if write.QualitySchedMaxConsecutiveSlow != nil && (*write.QualitySchedMaxConsecutiveSlow < 1 || *write.QualitySchedMaxConsecutiveSlow > schedN) {
		return SmartSchedulePlatformWrite{}, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", "quality_sched_max_consecutive_slow must be between 1 and sched N")
	}
	if write.QualityMinSuccessRate != nil && (*write.QualityMinSuccessRate <= 0 || *write.QualityMinSuccessRate > 1) {
		return SmartSchedulePlatformWrite{}, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", "quality_min_success_rate must be in (0,1]")
	}
	if write.QualityWindowSamples == nil && write.QualityWindowN != nil {
		write.QualityWindowSamples = write.QualityWindowN
	}
	if err := validateSmartScheduleWindowField("quality_window_samples", write.QualityWindowSamples); err != nil {
		return SmartSchedulePlatformWrite{}, err
	}
	if err := validateSmartScheduleWindowField("quality_min_success_samples", write.QualityMinSuccessSamples); err != nil {
		return SmartSchedulePlatformWrite{}, err
	}
	if err := validateSmartScheduleWindowField("quality_min_ttft_samples", write.QualityMinTTFTSamples); err != nil {
		return SmartSchedulePlatformWrite{}, err
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
	ttft, success := splitSmartScheduleWindowWrite(write.QualityWindowSamples, write.QualityMinTTFTSamples, write.QualityMinSuccessSamples)
	write.QualityMinTTFTSamples = &ttft
	write.QualityMinSuccessSamples = &success
	write.QualityWindowSamples, write.QualityWindowN = echoCompatSmartScheduleWindowN(ttft, success)
	if !qualityGateHasConfiguredColumn(write.QualityMaxP50TTFTMs, write.QualityMinSuccessRate, write.QualityMinSuccessSamples, write.QualityMinTTFTSamples, write.QualityCondition) {
		write.QualityMaxP50TTFTMs = nil
		write.QualityMinSuccessRate = nil
		write.QualityCondition = nil
	}
	mode, custom, err := NormalizeProbeConcurrencyWrite(write.ProbeConcurrencyMode, write.ProbeConcurrency)
	if err != nil {
		return SmartSchedulePlatformWrite{}, err
	}
	write.ProbeConcurrencyMode = mode
	write.ProbeConcurrency = custom
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
// concurrency:account_user:{accountID}:{userID}:{platform}. Uncapped members
// are included so the admin badge can show this user's occupancy; this does
// not acquire slots or change pair-cap enforcement.
func (s *UserSmartScheduleService) hydratePairCurrent(ctx context.Context, userID int64, view *UserSmartScheduleView) {
	if s == nil || s.pairConcurrency == nil || view == nil || userID <= 0 || len(view.Platforms) == 0 {
		return
	}
	for platformKey, platform := range view.Platforms {
		ids := smartScheduleViewAccountIDs(platform)
		if len(ids) == 0 {
			continue
		}
		counts, err := s.pairConcurrency.GetAccountUserConcurrencyBatch(WithScheduleLookupPlatform(ctx, platformKey), ids, userID)
		if err != nil || counts == nil {
			continue
		}
		for i := range platform.Accounts {
			platform.Accounts[i].CurrentConcurrency = counts[platform.Accounts[i].AccountID]
		}
		view.Platforms[platformKey] = platform
	}
}

func smartScheduleViewAccountIDs(platform SmartSchedulePlatformView) []int64 {
	ids := make([]int64, 0, len(platform.Accounts))
	seen := map[int64]struct{}{}
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
	return ids
}

func (s *UserSmartScheduleService) hydratePairCooldown(ctx context.Context, userID int64, view *UserSmartScheduleView) {
	if s == nil || s.cache == nil || view == nil || userID <= 0 || len(view.Platforms) == 0 {
		return
	}
	now := time.Now().UTC()
	for platformKey, platform := range view.Platforms {
		ids := smartScheduleViewAccountIDs(platform)
		if len(ids) == 0 {
			continue
		}
		untilByAccount := s.cache.GetCooldownUntilBatch(ctx, ids, userID, platformKey, now)
		if len(untilByAccount) == 0 {
			continue
		}
		var softLives map[int64]*PairQualityLive
		if platform.SoftCooldown {
			softLives = s.cache.GetSoftCooldownBatch(ctx, ids, userID, platformKey)
		}
		policy := platformViewToPolicy(&platform)
		for i := range platform.Accounts {
			until, ok := untilByAccount[platform.Accounts[i].AccountID]
			if !ok || until.IsZero() {
				continue
			}
			copied := until
			platform.Accounts[i].CooldownUntil = &copied
			if reason := s.cache.GetCooldownReason(ctx, platform.Accounts[i].AccountID, userID, platformKey); reason != "" {
				reasonCopy := reason
				platform.Accounts[i].CooldownReason = &reasonCopy
			}
			if platform.SoftCooldown {
				var live *PairQualityLive
				if softLives != nil {
					live = softLives[platform.Accounts[i].AccountID]
				}
				platform.Accounts[i].SoftCooldownProgress = softCooldownProgressView(live, policy)
			}
		}
		view.Platforms[platformKey] = platform
	}
}

func (s *UserSmartScheduleService) hydratePairResume(ctx context.Context, userID int64, view *UserSmartScheduleView) {
	if s == nil || s.cache == nil || view == nil || userID <= 0 || len(view.Platforms) == 0 {
		return
	}
	now := time.Now().UTC()
	for platformKey, platform := range view.Platforms {
		ids := smartScheduleViewAccountIDs(platform)
		if len(ids) == 0 {
			continue
		}
		untilByAccount := s.cache.GetPairResumeUntilBatch(ctx, ids, userID, platformKey, now)
		if len(untilByAccount) == 0 {
			continue
		}
		for i := range platform.Accounts {
			live, ok := untilByAccount[platform.Accounts[i].AccountID]
			if !ok || !live.Active(now) {
				continue
			}
			if !live.WatchUntil.IsZero() {
				watch := live.WatchUntil
				platform.Accounts[i].ResumeUntil = &watch
			}
			if !live.ChipUntil.IsZero() {
				chip := live.ChipUntil
				platform.Accounts[i].ResumeChipUntil = &chip
			}
		}
		view.Platforms[platformKey] = platform
	}
}

func (s *UserSmartScheduleService) hydratePairProbing(ctx context.Context, userID int64, view *UserSmartScheduleView) {
	if s == nil || s.cache == nil || view == nil || userID <= 0 || len(view.Platforms) == 0 {
		return
	}
	for platformKey, platform := range view.Platforms {
		ids := smartScheduleViewAccountIDs(platform)
		if len(ids) == 0 {
			continue
		}
		probing := s.cache.IsProbingBatch(ctx, ids, userID, platformKey)
		if len(probing) == 0 {
			continue
		}
		desired := viewProbeDesired(&platform)
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
			cap := ProbeInFlightCap(desired, memberCap)
			platform.Accounts[i].ProbeCap = &cap
		}
		view.Platforms[platformKey] = platform
	}
}

func (s *UserSmartScheduleService) hydratePairPinned(ctx context.Context, userID int64, view *UserSmartScheduleView) {
	if s == nil || s.cache == nil || view == nil || userID <= 0 || len(view.Platforms) == 0 {
		return
	}
	for platformKey, platform := range view.Platforms {
		ids := smartScheduleViewAccountIDs(platform)
		if len(ids) == 0 {
			continue
		}
		pinned := s.cache.IsPinnedBatch(ctx, ids, userID, platformKey)
		if len(pinned) == 0 {
			continue
		}
		for i := range platform.Accounts {
			if !pinned[platform.Accounts[i].AccountID] {
				continue
			}
			if platform.Accounts[i].Paused || platform.Accounts[i].CooldownUntil != nil {
				continue
			}
			platform.Accounts[i].Pinned = true
			platform.Accounts[i].Probing = false
			platform.Accounts[i].ProbeCap = nil
		}
		view.Platforms[platformKey] = platform
	}
}

func (s *UserSmartScheduleService) hydratePairQuality(ctx context.Context, userID int64, view *UserSmartScheduleView) {
	if s == nil || s.cache == nil || view == nil || userID <= 0 || len(view.Platforms) == 0 {
		return
	}
	now := time.Now().UTC()
	for platformKey, platform := range view.Platforms {
		ids := smartScheduleViewAccountIDs(platform)
		if len(ids) == 0 {
			continue
		}
		lives := s.cache.GetPairQualityBatch(ctx, ids, userID, platformKey)
		var softLives map[int64]*PairQualityLive
		if platform.SoftCooldown {
			softLives = s.cache.GetSoftCooldownBatch(ctx, ids, userID, platformKey)
		}
		nTTFT := viewPolicyTTFTN(&platform)
		nOK := viewPolicySuccessN(&platform)
		n := maxSmartScheduleWindowN(nTTFT, nOK)
		policy := platformViewToPolicy(&platform)
		for i := range platform.Accounts {
			live := lives[platform.Accounts[i].AccountID]
			viewSnap := SmartSchedulePairQualityView{N: n, NTTFT: nTTFT, NSuccess: nOK, NOK: nOK}
			if live != nil {
				viewSnap = live.View()
				viewSnap.NTTFT = nTTFT
				viewSnap.NSuccess = nOK
				viewSnap.NOK = nOK
				viewSnap.N = n
			}
			viewSnap = aliasPairQualityView(viewSnap)
			resumeActive := s.cache.PairResumeActive(ctx, platform.Accounts[i].AccountID, userID, platformKey, now)
			cooling := platform.Accounts[i].CooldownUntil != nil
			phase := pairQualityMetricsPhase(
				platform.Accounts[i].Probing,
				cooling,
				platform.SoftCooldown,
				platform.Accounts[i].Pinned || resumeActive,
			)
			var softLive *PairQualityLive
			if softLives != nil {
				softLive = softLives[platform.Accounts[i].AccountID]
			}
			attachPairQualityPhaseMetrics(&viewSnap, live, softLive, policy, phase)
			if platform.Accounts[i].Paused || cooling || platform.Accounts[i].Pinned || resumeActive {
				platform.Accounts[i].PairQuality = &viewSnap
				continue
			}
			if policy.HasQualityMetrics() {
				knobs := SchedQualityKnobs(policy)
				phaseLabel := CooldownPhaseSelectable
				if platform.Accounts[i].Probing {
					knobs = ProbeQualityKnobs(policy)
					phaseLabel = CooldownPhaseProbe
				}
				ev := EvalQuality(live, knobs)
				if ev.State == LatencyEvalFail {
					platform.Accounts[i].WillCool = true
					reason := formatSmartScheduleCooldownDetail(phaseLabel, CooldownSamplePair, ev.Reasons)
					platform.Accounts[i].QualityReason = &reason
					viewSnap.QualityReason = reason
				}
			}
			platform.Accounts[i].PairQuality = &viewSnap
		}
		view.Platforms[platformKey] = platform
	}
}

func viewPolicyTTFTN(platform *SmartSchedulePlatformView) int {
	if platform == nil {
		return DefaultSmartScheduleWindowN
	}
	return resolveSmartScheduleMetricN(platform.QualityMinTTFTSamples, platform.QualityWindowSamples)
}

func viewPolicySuccessN(platform *SmartSchedulePlatformView) int {
	if platform == nil {
		return DefaultSmartScheduleWindowN
	}
	return resolveSmartScheduleMetricN(platform.QualityMinSuccessSamples, platform.QualityWindowSamples)
}

func viewPolicyN(platform *SmartSchedulePlatformView) int {
	return maxSmartScheduleWindowN(viewPolicyTTFTN(platform), viewPolicySuccessN(platform))
}

func viewProbeDesired(platform *SmartSchedulePlatformView) int {
	if platform == nil {
		return DefaultSmartScheduleWindowN
	}
	mode, custom := EchoProbeConcurrency(platform.ProbeConcurrencyMode, platform.ProbeConcurrency)
	if mode == ProbeConcurrencyModeCustom && custom != nil {
		return *custom
	}
	return viewPolicySuccessN(platform)
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
	nTTFT := viewPolicyTTFTN(&row)
	nOK := viewPolicySuccessN(&row)
	n := maxSmartScheduleWindowN(nTTFT, nOK)
	for _, member := range row.Accounts {
		if member.AccountID == accountID {
			found = true
			if member.PairQuality != nil {
				if member.PairQuality.NTTFT > 0 {
					nTTFT = member.PairQuality.NTTFT
				}
				if member.PairQuality.NSuccess > 0 {
					nOK = member.PairQuality.NSuccess
				}
				n = maxSmartScheduleWindowN(nTTFT, nOK)
			}
			break
		}
	}
	if !found {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account is not in this platform pool")
	}
	emptyView := aliasPairQualityView(SmartSchedulePairQualityView{N: n, NTTFT: nTTFT, NSuccess: nOK, NOK: nOK})
	policy := platformViewToPolicy(&row)
	var member *SmartScheduleAccountMember
	for i := range row.Accounts {
		if row.Accounts[i].AccountID == accountID {
			member = &row.Accounts[i]
			break
		}
	}
	phase := MetricsPhaseSched
	if member != nil {
		phase = pairQualityMetricsPhase(
			member.Probing,
			member.CooldownUntil != nil,
			row.SoftCooldown,
			member.Pinned || member.ResumeUntil != nil,
		)
		if member.PairQuality != nil {
			emptyView = *member.PairQuality
		}
	}
	if member == nil || member.PairQuality == nil {
		attachPairQualityPhaseMetrics(&emptyView, nil, nil, policy, phase)
	}
	detail := &SmartSchedulePairQualityDetail{
		AccountID: accountID,
		UserID:    userID,
		N:         n,
		NTTFT:     nTTFT,
		NSuccess:  nOK,
		Live:      emptyView,
		Current:   emptyView,
		Snapshots: []PairQualitySnapshot{},
		Events:    []PairQualityEvent{},
	}
	if s.cache != nil {
		if live := s.cache.GetPairQuality(ctx, accountID, userID, platform); live != nil {
			detail.Live = live.View()
			detail.Live.N = n
			detail.Live.NTTFT = nTTFT
			detail.Live.NSuccess = nOK
			detail.Live.NOK = nOK
			detail.Live = aliasPairQualityView(detail.Live)
			var softLive *PairQualityLive
			if row.SoftCooldown {
				softLive = s.cache.GetSoftCooldown(ctx, accountID, userID, platform)
			}
			attachPairQualityPhaseMetrics(&detail.Live, live, softLive, policy, phase)
			if member != nil && member.QualityReason != nil {
				detail.Live.QualityReason = *member.QualityReason
			}
		}
		detail.Current = detail.Live
		if snaps := s.cache.ListPairQualitySnapshots(ctx, accountID, userID, platform, 200); snaps != nil {
			detail.Snapshots = make([]PairQualitySnapshot, 0, len(snaps))
			for _, snap := range snaps {
				snap.N = n
				snap.NTTFT = nTTFT
				snap.NSuccess = nOK
				snap.NOK = nOK
				detail.Snapshots = append(detail.Snapshots, aliasPairQualitySnapshot(snap))
			}
		}
		if events := s.cache.ListPairQualityEvents(ctx, accountID, userID, platform, 200); events != nil {
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
	found := uniqueSmartScheduleMembershipPlatform(&UserSmartScheduleBundle{Policies: membershipPoliciesFromView(view)}, accountID)
	if found == "" {
		if smartScheduleAccountInMultiplePools(view, accountID) {
			return nil, infraerrors.BadRequest("SMART_SCHEDULE_PLATFORM_REQUIRED", "account is in multiple platform pools")
		}
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account is not in this platform pool")
	}
	return s.GetPairQualityDetail(ctx, userID, found, accountID)
}

func (s *UserSmartScheduleService) GetPairQualityBatch(ctx context.Context, userID int64, accountIDs []int64, platform string) (*SmartSchedulePairQualityBatch, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("SMART_SCHEDULE_RESUME_INVALID", "user_id is required")
	}
	platform = normalizeSmartSchedulePlatform(platform)
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
	memberships := map[int64]int{}
	if platform == "" {
		for _, row := range view.Platforms {
			for _, member := range row.Accounts {
				if member.AccountID > 0 {
					memberships[member.AccountID]++
				}
			}
		}
	}
	for platformKey, row := range view.Platforms {
		if platform != "" && platformKey != platform {
			continue
		}
		for _, member := range row.Accounts {
			if member.AccountID <= 0 {
				continue
			}
			if len(wanted) > 0 {
				if _, ok := wanted[member.AccountID]; !ok {
					continue
				}
			}
			if platform == "" && memberships[member.AccountID] > 1 {
				continue
			}
			if member.PairQuality != nil {
				out.Pairs[strconv.FormatInt(member.AccountID, 10)] = *member.PairQuality
			}
		}
	}
	return out, nil
}

func membershipPoliciesFromView(view *UserSmartScheduleView) map[string]*SmartSchedulePlatformPolicy {
	if view == nil {
		return nil
	}
	out := make(map[string]*SmartSchedulePlatformPolicy, len(view.Platforms))
	for platform, row := range view.Platforms {
		policy := &SmartSchedulePlatformPolicy{AccountIDs: map[int64]struct{}{}}
		for _, member := range row.Accounts {
			if member.AccountID > 0 {
				policy.AccountIDs[member.AccountID] = struct{}{}
			}
		}
		out[platform] = policy
	}
	return out
}

func smartScheduleAccountInMultiplePools(view *UserSmartScheduleView, accountID int64) bool {
	if view == nil || accountID <= 0 {
		return false
	}
	found := 0
	for _, row := range view.Platforms {
		for _, member := range row.Accounts {
			if member.AccountID == accountID {
				found++
				break
			}
		}
	}
	return found > 1
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

func platformViewToPolicy(view *SmartSchedulePlatformView) *SmartSchedulePlatformPolicy {
	if view == nil {
		return nil
	}
	policy := &SmartSchedulePlatformPolicy{
		Enabled:                        view.Enabled,
		QualityMaxP50TTFTMs:            view.QualityMaxP50TTFTMs,
		QualityMinSuccessRate:          view.QualityMinSuccessRate,
		QualityWindowSamples:           view.QualityWindowSamples,
		QualityMinSuccessSamples:       view.QualityMinSuccessSamples,
		QualityMinTTFTSamples:          view.QualityMinTTFTSamples,
		QualityCondition:               view.QualityCondition,
		CooldownMinutes:                view.CooldownMinutes,
		SoftCooldown:                   view.SoftCooldown,
		ProbeLatencyV2:                 view.ProbeLatencyV2,
		ProbeConcurrencyMode:           view.ProbeConcurrencyMode,
		ProbeConcurrency:               view.ProbeConcurrency,
		QualityMaxSlowInWindow:         view.QualityMaxSlowInWindow,
		QualityMaxConsecutiveSlow:      view.QualityMaxConsecutiveSlow,
		QualityMaxP50DurationMs:        view.QualityMaxP50DurationMs,
		QualitySchedWindowN:            view.QualitySchedWindowN,
		QualitySchedMaxSlowInWindow:    view.QualitySchedMaxSlowInWindow,
		QualitySchedMaxConsecutiveSlow: view.QualitySchedMaxConsecutiveSlow,
	}
	return policy
}

func policyToView(platform string, policy *SmartSchedulePlatformPolicy) SmartSchedulePlatformView {
	view := SmartSchedulePlatformView{
		CooldownMinutes:      DefaultSmartScheduleCooldownMinutes,
		ProbeConcurrencyMode: ProbeConcurrencyModeFollowN,
		Accounts:             []SmartScheduleAccountMember{},
	}
	if policy == nil {
		return view
	}
	view.Enabled = policy.Enabled && policy.MemberCount() > 0
	view.QualityMaxP50TTFTMs = policy.QualityMaxP50TTFTMs
	view.QualityMaxSlowInWindow = policy.QualityMaxSlowInWindow
	view.QualityMaxConsecutiveSlow = policy.QualityMaxConsecutiveSlow
	view.QualityMaxP50DurationMs = policy.QualityMaxP50DurationMs
	view.QualitySchedWindowN = policy.QualitySchedWindowN
	view.QualitySchedMaxSlowInWindow = policy.QualitySchedMaxSlowInWindow
	view.QualitySchedMaxConsecutiveSlow = policy.QualitySchedMaxConsecutiveSlow
	view.QualityMinSuccessRate = policy.QualityMinSuccessRate
	if policy.HasQualityMetrics() || policy.QualityWindowSamples != nil || policy.QualityMinSuccessSamples != nil || policy.QualityMinTTFTSamples != nil {
		ttft := policy.TTFTWindowN()
		success := policy.SuccessWindowN()
		view.QualityMinTTFTSamples = &ttft
		view.QualityMinSuccessSamples = &success
		view.QualityWindowSamples, view.QualityWindowN = echoCompatSmartScheduleWindowN(ttft, success)
	}
	view.QualityCondition = policy.QualityCondition
	view.ProbeConcurrencyMode, view.ProbeConcurrency = EchoProbeConcurrency(policy.ProbeConcurrencyMode, policy.ProbeConcurrency)
	if policy.CooldownMinutes >= MinSmartScheduleCooldownMinutes {
		view.CooldownMinutes = policy.CooldownMinutes
	}
	view.SoftCooldown = policy.SoftCooldown
	view.ProbeLatencyV2 = policy.ProbeLatencyV2
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
