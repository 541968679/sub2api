package service

import (
	"context"
	"math"
)

// accountCheaperThenPreferred reports whether candidate has a strictly lower
// EffectiveUpstreamRate than preferred. Same rate is not cheaper.
func accountCheaperThenPreferred(candidate, preferred *Account) bool {
	if candidate == nil || preferred == nil {
		return false
	}
	return compareUpstreamRate(candidate, preferred) < 0
}

func accountScheduleLoadRate(loadByID map[int64]*AccountLoadInfo, accountID int64) int {
	if loadByID == nil {
		return 0
	}
	info := loadByID[accountID]
	if info == nil {
		return 0
	}
	return info.LoadRate
}

func accountHasScheduleHeadroom(loadByID map[int64]*AccountLoadInfo, accountID int64) bool {
	return accountScheduleLoadRate(loadByID, accountID) < 100
}

func cheaperSchedulablePeerExists(sticky *Account, candidates []*Account) bool {
	if sticky == nil {
		return false
	}
	for _, acc := range candidates {
		if acc == nil || acc.ID == sticky.ID {
			continue
		}
		if accountCheaperThenPreferred(acc, sticky) {
			return true
		}
	}
	return false
}

func minSchedulableUpstreamRate(candidates []*Account) (float64, bool) {
	minRate := math.MaxFloat64
	found := false
	for _, acc := range candidates {
		if acc == nil {
			continue
		}
		rate := acc.EffectiveUpstreamRate()
		if !found || rate < minRate {
			minRate = rate
			found = true
		}
	}
	return minRate, found
}

func hasHigherUpstreamRateHeadroom(candidates []*Account, minRate float64, loadByID map[int64]*AccountLoadInfo) bool {
	for _, acc := range candidates {
		if acc == nil {
			continue
		}
		if acc.EffectiveUpstreamRate() <= minRate {
			continue
		}
		if accountHasScheduleHeadroom(loadByID, acc.ID) {
			return true
		}
	}
	return false
}

func withScheduleUserIDForSticky(ctx context.Context, userID int64) context.Context {
	return withScheduleUserID(ctx, userID)
}

// cheaperTierEligiblePeers is the shared ruler for overflow escape and WaitPlan skip:
// admitsScheduleUser (paused / cooling / quality / pool miss), not pair_full, same fallback partition.
func cheaperTierEligiblePeers(
	ctx context.Context,
	lookup SmartScheduleLookup,
	sticky *Account,
	candidates []*Account,
	pairCounts map[int64]int,
) []*Account {
	if sticky == nil {
		return nil
	}
	out := make([]*Account, 0, len(candidates))
	for _, acc := range candidates {
		if acc == nil {
			continue
		}
		if acc.IsFallbackOnly() != sticky.IsFallbackOnly() {
			continue
		}
		if !admitsScheduleUser(ctx, acc, nil, lookup) {
			continue
		}
		if isPairConcurrencyFull(ctx, acc, pairCounts[acc.ID], lookup) {
			continue
		}
		out = append(out, acc)
	}
	return out
}

func cheaperTierAdmittedPeers(ctx context.Context, lookup SmartScheduleLookup, selected *Account, candidates []*Account) []*Account {
	if selected == nil {
		return nil
	}
	out := make([]*Account, 0, len(candidates))
	for _, acc := range candidates {
		if acc == nil {
			continue
		}
		if acc.IsFallbackOnly() != selected.IsFallbackOnly() {
			continue
		}
		if !admitsScheduleUser(ctx, acc, nil, lookup) {
			continue
		}
		out = append(out, acc)
	}
	return out
}

// shouldEscapeSessionStickyForCheaperTier is session-sticky only.
// overflow=false always keeps the pin (protect prefix cache). overflow=true
// may steal once when a strictly cheaper eligible peer has LoadRate<100.
func shouldEscapeSessionStickyForCheaperTier(
	ctx context.Context,
	lookup SmartScheduleLookup,
	userID int64,
	sticky *Account,
	candidates []*Account,
	loadByID map[int64]*AccountLoadInfo,
	pairCounts map[int64]int,
	overflow bool,
) bool {
	if !overflow || sticky == nil || len(candidates) == 0 {
		return false
	}
	ctx = withScheduleUserIDForSticky(ctx, userID)
	eligible := cheaperTierEligiblePeers(ctx, lookup, sticky, candidates, pairCounts)
	minRate, ok := minSchedulableUpstreamRate(eligible)
	if !ok || sticky.EffectiveUpstreamRate() <= minRate {
		return false
	}
	for _, acc := range eligible {
		if acc == nil || acc.EffectiveUpstreamRate() != minRate {
			continue
		}
		if accountHasScheduleHeadroom(loadByID, acc.ID) {
			return true
		}
	}
	return false
}

func sessionStickyOverflowOnBind(
	ctx context.Context,
	lookup SmartScheduleLookup,
	userID int64,
	selected *Account,
	candidates []*Account,
	previous *Account,
) bool {
	if selected == nil {
		return false
	}
	ctx = withScheduleUserIDForSticky(ctx, userID)
	if previous != nil && selected.EffectiveUpstreamRate() > previous.EffectiveUpstreamRate() {
		return true
	}
	admitted := cheaperTierAdmittedPeers(ctx, lookup, selected, candidates)
	minRate, ok := minSchedulableUpstreamRate(admitted)
	return ok && selected.EffectiveUpstreamRate() > minRate
}

func isUnpooledScheduleUser(ctx context.Context, lookup SmartScheduleLookup, userID int64, platform string) bool {
	return lookupEnabledSmartPolicy(ctx, lookup, userID, platform) == nil
}

func shouldSkipMinRateWaitPlan(
	ctx context.Context,
	lookup SmartScheduleLookup,
	userID int64,
	account *Account,
	candidates []*Account,
	loadByID map[int64]*AccountLoadInfo,
	pairCounts map[int64]int,
) bool {
	if account == nil {
		return false
	}
	ctx = withScheduleUserIDForSticky(ctx, userID)
	eligible := cheaperTierEligiblePeers(ctx, lookup, account, candidates, pairCounts)
	minRate, ok := minSchedulableUpstreamRate(eligible)
	if !ok || account.EffectiveUpstreamRate() > minRate {
		return false
	}
	return hasHigherUpstreamRateHeadroom(eligible, minRate, loadByID)
}

func isBetterSchedulableAccount(candidate, current *Account, preferOAuthWhenUnused bool) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
	}
	if accountCheaperThenPreferred(candidate, current) {
		return true
	}
	if accountCheaperThenPreferred(current, candidate) {
		return false
	}
	if candidate.Priority < current.Priority {
		return true
	}
	if candidate.Priority > current.Priority {
		return false
	}
	switch {
	case candidate.LastUsedAt == nil && current.LastUsedAt != nil:
		return true
	case candidate.LastUsedAt != nil && current.LastUsedAt == nil:
		return false
	case candidate.LastUsedAt == nil && current.LastUsedAt == nil:
		return preferOAuthWhenUnused && candidate.Type == AccountTypeOAuth && current.Type != AccountTypeOAuth
	default:
		return candidate.LastUsedAt.Before(*current.LastUsedAt)
	}
}

func accountPointers(accounts []Account) []*Account {
	out := make([]*Account, 0, len(accounts))
	for i := range accounts {
		out = append(out, &accounts[i])
	}
	return out
}

func scheduleLoadMap(ctx context.Context, concurrency *ConcurrencyService, accounts []*Account) map[int64]*AccountLoadInfo {
	if concurrency == nil || len(accounts) == 0 {
		return nil
	}
	req := make([]AccountWithConcurrency, 0, len(accounts))
	for _, acc := range accounts {
		if acc == nil {
			continue
		}
		req = append(req, AccountWithConcurrency{
			ID:             acc.ID,
			MaxConcurrency: acc.EffectiveLoadFactor(),
		})
	}
	if len(req) == 0 {
		return nil
	}
	loadMap, err := concurrency.GetAccountsLoadBatch(ctx, req)
	if err != nil {
		return nil
	}
	return loadMap
}

func (s *GatewayService) readSessionBinding(ctx context.Context, groupID *int64, sessionHash string) StickySessionBinding {
	if s == nil || s.cache == nil || sessionHash == "" {
		return StickySessionBinding{}
	}
	binding, err := s.cache.GetSessionBinding(ctx, derefGroupID(groupID), sessionHash)
	if err != nil {
		return StickySessionBinding{}
	}
	return binding
}

func (s *GatewayService) shouldEscapeSessionStickyForCheaperTier(ctx context.Context, sticky *Account, candidates []*Account, overflow bool) bool {
	if s == nil || sticky == nil || !overflow || !cheaperSchedulablePeerExists(sticky, candidates) {
		return false
	}
	userID := scheduleUserIDFromContext(ctx, 0)
	return shouldEscapeSessionStickyForCheaperTier(
		ctx, s.smartScheduleCache, userID, sticky, candidates,
		scheduleLoadMap(ctx, s.concurrencyService, candidates),
		loadPairConcurrencyCounts(ctx, s.concurrencyService, candidates, userID, s.smartScheduleCache),
		true,
	)
}

func (s *GatewayService) escapeSessionStickyIfCheaperTier(ctx context.Context, groupID *int64, sessionHash string, sticky *Account, candidates []*Account, overflow bool) bool {
	if s == nil || s.cache == nil || sessionHash == "" {
		return false
	}
	if s.shouldEscapeSessionStickyForCheaperTier(ctx, sticky, candidates, overflow) {
		_ = s.cache.DeleteSessionAccountID(ctx, derefGroupID(groupID), sessionHash)
		return true
	}
	platform := ""
	if sticky != nil {
		platform = sticky.Platform
	}
	if shouldEscapeSessionStickyForPublicQuality(
		ctx, s.publicSchedule, s.smartScheduleCache, scheduleUserIDFromContext(ctx, 0), platform, sticky, candidates,
	) {
		_ = s.cache.DeleteSessionAccountID(ctx, derefGroupID(groupID), sessionHash)
		return true
	}
	return false
}

func (s *GatewayService) bindStickySessionAfterSelect(ctx context.Context, groupID *int64, sessionHash string, selected *Account, candidates []*Account, previous *Account) {
	if s == nil || s.cache == nil || sessionHash == "" || selected == nil || selected.ID <= 0 {
		return
	}
	userID := scheduleUserIDFromContext(ctx, 0)
	overflow := sessionStickyOverflowOnBind(ctx, s.smartScheduleCache, userID, selected, candidates, previous)
	_ = s.cache.SetSessionBinding(ctx, derefGroupID(groupID), sessionHash, StickySessionBinding{AccountID: selected.ID, Overflow: overflow}, stickySessionTTL)
}

func (s *OpenAIGatewayService) shouldEscapeSessionStickyForCheaperTier(ctx context.Context, sticky *Account, candidates []*Account, overflow bool) bool {
	if s == nil || sticky == nil || !overflow || !cheaperSchedulablePeerExists(sticky, candidates) {
		return false
	}
	userID := scheduleUserIDFromContext(ctx, 0)
	return shouldEscapeSessionStickyForCheaperTier(
		ctx, s.smartScheduleCache, userID, sticky, candidates,
		scheduleLoadMap(ctx, s.concurrencyService, candidates),
		loadPairConcurrencyCounts(ctx, s.concurrencyService, candidates, userID, s.smartScheduleCache),
		true,
	)
}

func (s *OpenAIGatewayService) escapeSessionStickyIfCheaperTier(ctx context.Context, groupID *int64, sessionHash string, sticky *Account, candidates []*Account, overflow bool) bool {
	if s == nil || sessionHash == "" {
		return false
	}
	if s.shouldEscapeSessionStickyForCheaperTier(ctx, sticky, candidates, overflow) {
		_ = s.deleteStickySessionAccountID(ctx, groupID, sessionHash)
		return true
	}
	platform := ""
	if sticky != nil {
		platform = sticky.Platform
	}
	if shouldEscapeSessionStickyForPublicQuality(
		ctx, s.publicSchedule, s.smartScheduleCache, scheduleUserIDFromContext(ctx, 0), platform, sticky, candidates,
	) {
		_ = s.deleteStickySessionAccountID(ctx, groupID, sessionHash)
		return true
	}
	return false
}

func (s *GatewayService) loadStickyEscapeCandidates(ctx context.Context, groupID *int64, platform string, isolated bool, excludedIDs map[int64]struct{}, loaded []Account) []*Account {
	accounts := loaded
	if len(accounts) == 0 && s != nil {
		listed, _, err := s.listSchedulableAccounts(ctx, groupID, platform, isolated)
		if err != nil {
			return nil
		}
		accounts = listed
	}
	return excludedAccountIDsFilter(excludedIDs, accountPointers(accounts))
}

func excludedAccountIDsFilter(excludedIDs map[int64]struct{}, accounts []*Account) []*Account {
	if len(accounts) == 0 || len(excludedIDs) == 0 {
		return accounts
	}
	out := make([]*Account, 0, len(accounts))
	for _, acc := range accounts {
		if acc == nil {
			continue
		}
		if _, excluded := excludedIDs[acc.ID]; excluded {
			continue
		}
		out = append(out, acc)
	}
	return out
}
