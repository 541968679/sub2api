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

// shouldEscapeSessionStickyForCheaperTier is session-sticky only.
// Unpooled is judged with sticky.Platform via lookupEnabledSmartPolicy — never a group platform.
// userID<=0 is the same fail-open as admitsScheduleUser (treated as unpooled).
func shouldEscapeSessionStickyForCheaperTier(
	ctx context.Context,
	lookup SmartScheduleLookup,
	userID int64,
	sticky *Account,
	candidates []*Account,
	loadByID map[int64]*AccountLoadInfo,
) bool {
	if sticky == nil || len(candidates) == 0 {
		return false
	}
	if lookupEnabledSmartPolicy(ctx, lookup, userID, sticky.Platform) != nil {
		return false
	}
	minRate, ok := minSchedulableUpstreamRate(candidates)
	if !ok || sticky.EffectiveUpstreamRate() <= minRate {
		return false
	}
	for _, acc := range candidates {
		if acc == nil || acc.EffectiveUpstreamRate() != minRate {
			continue
		}
		if accountHasScheduleHeadroom(loadByID, acc.ID) {
			return true
		}
	}
	return false
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
) bool {
	if account == nil {
		return false
	}
	if !isUnpooledScheduleUser(ctx, lookup, userID, account.Platform) {
		return false
	}
	minRate, ok := minSchedulableUpstreamRate(candidates)
	if !ok || account.EffectiveUpstreamRate() > minRate {
		return false
	}
	return hasHigherUpstreamRateHeadroom(candidates, minRate, loadByID)
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

func (s *GatewayService) shouldEscapeSessionStickyForCheaperTier(ctx context.Context, sticky *Account, candidates []*Account) bool {
	if s == nil || sticky == nil || !cheaperSchedulablePeerExists(sticky, candidates) {
		return false
	}
	userID := scheduleUserIDFromContext(ctx, 0)
	return shouldEscapeSessionStickyForCheaperTier(ctx, s.smartScheduleCache, userID, sticky, candidates, scheduleLoadMap(ctx, s.concurrencyService, candidates))
}

func (s *GatewayService) escapeSessionStickyIfCheaperTier(ctx context.Context, groupID *int64, sessionHash string, sticky *Account, candidates []*Account) bool {
	if s == nil || s.cache == nil || sessionHash == "" {
		return false
	}
	if !s.shouldEscapeSessionStickyForCheaperTier(ctx, sticky, candidates) {
		return false
	}
	_ = s.cache.DeleteSessionAccountID(ctx, derefGroupID(groupID), sessionHash)
	return true
}

func (s *OpenAIGatewayService) shouldEscapeSessionStickyForCheaperTier(ctx context.Context, sticky *Account, candidates []*Account) bool {
	if s == nil || sticky == nil || !cheaperSchedulablePeerExists(sticky, candidates) {
		return false
	}
	userID := scheduleUserIDFromContext(ctx, 0)
	return shouldEscapeSessionStickyForCheaperTier(ctx, s.smartScheduleCache, userID, sticky, candidates, scheduleLoadMap(ctx, s.concurrencyService, candidates))
}

func (s *OpenAIGatewayService) escapeSessionStickyIfCheaperTier(ctx context.Context, groupID *int64, sessionHash string, sticky *Account, candidates []*Account) bool {
	if s == nil || sessionHash == "" {
		return false
	}
	if !s.shouldEscapeSessionStickyForCheaperTier(ctx, sticky, candidates) {
		return false
	}
	_ = s.deleteStickySessionAccountID(ctx, groupID, sessionHash)
	return true
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
