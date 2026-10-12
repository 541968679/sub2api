package service

import (
	"context"
)

func isUnpooledScheduleUser(ctx context.Context, lookup SmartScheduleLookup, userID int64, platform string) bool {
	return lookupEnabledSmartPolicy(ctx, lookup, userID, platform) == nil
}

func isBetterSchedulableAccount(candidate, current *Account, preferOAuthWhenUnused bool) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
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

func (s *GatewayService) escapeSessionStickyForQuality(ctx context.Context, groupID *int64, sessionHash string, sticky *Account, candidates []*Account) bool {
	if s == nil || s.cache == nil || sessionHash == "" || sticky == nil {
		return false
	}
	userID := scheduleUserIDFromContext(ctx, 0)
	platform := sticky.Platform
	if shouldEscapeSessionStickyForPublicQuality(
		ctx, s.publicSchedule, s.smartScheduleCache, userID, platform, sticky, candidates,
	) {
		_ = s.cache.DeleteSessionAccountID(ctx, derefGroupID(groupID), sessionHash)
		return true
	}
	if !stickyNeedsPublicHealthEscape(ctx, s.publicSchedule, s.smartScheduleCache, userID, platform, sticky) {
		return false
	}
	if shouldEscapeDemotedStickyForHealthierPeer(
		ctx, s.publicSchedule, s.smartScheduleCache, userID, platform, sticky, candidates,
		scheduleLoadMap(ctx, s.concurrencyService, candidates), nil,
	) {
		_ = s.cache.DeleteSessionAccountID(ctx, derefGroupID(groupID), sessionHash)
		return true
	}
	return false
}

func (s *GatewayService) bindStickySessionAfterSelect(ctx context.Context, groupID *int64, sessionHash string, selected *Account, _ []*Account, _ *Account) {
	if s == nil || s.cache == nil || sessionHash == "" || selected == nil || selected.ID <= 0 {
		return
	}
	_ = s.cache.SetSessionBinding(ctx, derefGroupID(groupID), sessionHash, StickySessionBinding{AccountID: selected.ID, Overflow: false}, stickySessionTTL)
}

func (s *OpenAIGatewayService) escapeSessionStickyForQuality(ctx context.Context, groupID *int64, sessionHash string, sticky *Account, candidates []*Account) bool {
	if s == nil || sessionHash == "" || sticky == nil {
		return false
	}
	userID := scheduleUserIDFromContext(ctx, 0)
	platform := sticky.Platform
	if shouldEscapeSessionStickyForPublicQuality(
		ctx, s.publicSchedule, s.smartScheduleCache, userID, platform, sticky, candidates,
	) {
		_ = s.deleteStickySessionAccountID(ctx, groupID, sessionHash)
		return true
	}
	if !stickyNeedsPublicHealthEscape(ctx, s.publicSchedule, s.smartScheduleCache, userID, platform, sticky) {
		return false
	}
	if shouldEscapeDemotedStickyForHealthierPeer(
		ctx, s.publicSchedule, s.smartScheduleCache, userID, platform, sticky, candidates,
		scheduleLoadMap(ctx, s.concurrencyService, candidates), nil,
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
