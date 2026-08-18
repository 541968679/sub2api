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
	if err := s.validatePoolMembers(ctx, platform, normalized.Accounts); err != nil {
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
	case PairAdmissionPaused, PairAdmissionCooling, PairAdmissionResumed, PairAdmissionSelectable:
		return state, nil
	default:
		return "", infraerrors.BadRequest("SMART_SCHEDULE_ADMISSION_INVALID", "state must be paused, cooling, resumed, or selectable")
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
	if err := s.setMemberPaused(ctx, userID, accountID, parsed == PairAdmissionPaused); err != nil {
		return nil, err
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
		return &PairAdmissionResult{AccountID: accountID, UserID: userID, State: parsed}, nil
	case PairAdmissionCooling:
		until := s.forcePairCooldown(ctx, accountID, userID, now)
		if s != nil && s.qualityLiveCache != nil {
			if err := s.qualityLiveCache.ClearUserResume(ctx, accountID, userID); err != nil {
				return nil, err
			}
		}
		return &PairAdmissionResult{AccountID: accountID, UserID: userID, State: parsed, CooldownUntil: &until}, nil
	case PairAdmissionSelectable:
		if s != nil && s.cache != nil {
			if err := s.cache.ClearCooldown(ctx, accountID, userID); err != nil {
				return nil, err
			}
		}
		if s != nil && s.qualityLiveCache != nil {
			if err := s.qualityLiveCache.MarkUserQualityWindow(ctx, accountID, userID); err != nil {
				return nil, err
			}
		}
		return &PairAdmissionResult{AccountID: accountID, UserID: userID, State: parsed}, nil
	default:
		if s != nil && s.cache != nil {
			if err := s.cache.ClearCooldown(ctx, accountID, userID); err != nil {
				return nil, err
			}
		}
		if s != nil && s.qualityLiveCache != nil {
			if err := s.qualityLiveCache.MarkUserResume(ctx, accountID, userID); err != nil {
				return nil, err
			}
		}
		return &PairAdmissionResult{AccountID: accountID, UserID: userID, State: PairAdmissionResumed}, nil
	}
}

func (s *UserSmartScheduleService) setMemberPaused(ctx context.Context, userID, accountID int64, paused bool) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if err := s.repo.SetMemberPaused(ctx, userID, accountID, paused); err != nil {
		return err
	}
	if s.cache != nil {
		return s.cache.Invalidate(ctx, userID)
	}
	return nil
}

func (s *UserSmartScheduleService) forcePairCooldown(ctx context.Context, accountID, userID int64, now time.Time) time.Time {
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
	return now.Add(time.Duration(minutes) * time.Minute)
}

func (s *UserSmartScheduleService) validatePoolMembers(ctx context.Context, platform string, members []SmartScheduleAccountMember) error {
	if len(members) == 0 {
		return nil
	}
	if s == nil || s.accountRepo == nil {
		return infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable")
	}
	ids := make([]int64, 0, len(members))
	seen := map[int64]struct{}{}
	for _, member := range members {
		if member.AccountID <= 0 {
			return infraerrors.BadRequest("SMART_SCHEDULE_INVALID_ACCOUNT", "invalid account id")
		}
		if _, ok := seen[member.AccountID]; ok {
			return infraerrors.BadRequest("SMART_SCHEDULE_DUPLICATE_ACCOUNT", "duplicate account in pool")
		}
		seen[member.AccountID] = struct{}{}
		ids = append(ids, member.AccountID)
	}
	accounts, err := s.accountRepo.GetByIDs(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[int64]*Account, len(accounts))
	for _, acc := range accounts {
		if acc != nil {
			byID[acc.ID] = acc
		}
	}
	for _, member := range members {
		acc := byID[member.AccountID]
		if acc == nil {
			return infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account not found")
		}
		if normalizeSmartSchedulePlatform(acc.Platform) != platform {
			return infraerrors.BadRequest("SMART_SCHEDULE_PLATFORM_MISMATCH", "account platform does not match the selected tab")
		}
	}
	return nil
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
		write.QualityMinSuccessSamples = nil
		write.QualityMinTTFTSamples = nil
		write.QualityCondition = nil
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
	if err != nil || len(accounts) == 0 {
		return
	}
	byID := make(map[int64]*Account, len(accounts))
	for _, acc := range accounts {
		if acc != nil {
			byID[acc.ID] = acc
		}
	}
	for platformKey, platform := range view.Platforms {
		for i := range platform.Accounts {
			if acc := byID[platform.Accounts[i].AccountID]; acc != nil {
				platform.Accounts[i].Priority = acc.Priority
			}
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
	view.QualityMinSuccessSamples = policy.QualityMinSuccessSamples
	view.QualityMinTTFTSamples = policy.QualityMinTTFTSamples
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
