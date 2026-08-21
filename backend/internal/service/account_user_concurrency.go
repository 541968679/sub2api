package service

import "context"

func combineReleaseFuncs(first, second func()) func() {
	if first == nil && second == nil {
		return func() {}
	}
	if first == nil {
		return second
	}
	if second == nil {
		return first
	}
	return func() {
		second()
		first()
	}
}

func isPairConcurrencyFull(ctx context.Context, account *Account, current int, lookup SmartScheduleLookup) bool {
	if account == nil {
		return false
	}
	max := resolvePairMaxConcurrency(ctx, account, lookup)
	return max > 0 && current >= max
}

// resolvePairSlotAcquire returns the real pair cap and whether the hot path
// should write concurrency:account_user:{accountID}:{userID}.
// Closed-pool members always track occupancy (count-only when cap is 0/null).
// 999 is UI-only (UNCAPPED_PAIR_DISPLAY_MAX) and must never be used as a cap.
func resolvePairSlotAcquire(ctx context.Context, account *Account, lookup SmartScheduleLookup) (pairMax int, trackOccupancy bool) {
	if account == nil {
		return 0, false
	}
	userID := scheduleUserIDFromContext(ctx, 0)
	if policy := lookupEnabledSmartPolicy(ctx, lookup, userID, account.Platform); policy != nil {
		if !policy.HasAccount(account.ID) {
			return 0, false
		}
		memberCap := policy.PairCap(account.ID)
		if lookup != nil && lookup.IsProbing(ctx, account.ID, userID) {
			return policy.ProbeInFlightCap(memberCap), true
		}
		return memberCap, true
	}
	return account.PairMaxConcurrency(userID), false
}

func pairConcurrencyAccountIDs(ctx context.Context, accounts []*Account, userID int64, lookup SmartScheduleLookup) []int64 {
	if userID <= 0 {
		return nil
	}
	ids := make([]int64, 0)
	seen := make(map[int64]struct{})
	for _, acc := range accounts {
		if acc == nil || acc.ID <= 0 {
			continue
		}
		max := resolvePairMaxConcurrency(withScheduleUserID(ctx, userID), acc, lookup)
		if max <= 0 {
			continue
		}
		if _, ok := seen[acc.ID]; ok {
			continue
		}
		seen[acc.ID] = struct{}{}
		ids = append(ids, acc.ID)
	}
	return ids
}

func loadPairConcurrencyCounts(ctx context.Context, svc *ConcurrencyService, accounts []*Account, userID int64, lookup SmartScheduleLookup) map[int64]int {
	ids := pairConcurrencyAccountIDs(ctx, accounts, userID, lookup)
	if len(ids) == 0 || svc == nil {
		return map[int64]int{}
	}
	counts, err := svc.GetAccountUserConcurrencyBatch(ctx, ids, userID)
	if err != nil || counts == nil {
		return map[int64]int{}
	}
	return counts
}

// acquireAccountAndPairSlot acquires the account slot, then the user×account
// pair slot when pairMax>=1 or trackOccupancy is set (closed-pool count-only).
// pairFull is true only when a real cap (pairMax>=1) rejected the pair acquire
// after the account slot succeeded (account slot is released). Uncapped
// occupancy tracking never returns pairFull and never uses 999 as a limit.
func acquireAccountAndPairSlot(ctx context.Context, svc *ConcurrencyService, account *Account, userID int64, pairMax int, trackOccupancy bool) (*AcquireResult, bool, error) {
	if account == nil {
		return &AcquireResult{Acquired: false}, false, nil
	}
	if svc == nil {
		return &AcquireResult{Acquired: true, ReleaseFunc: func() {}}, false, nil
	}

	result, err := svc.AcquireAccountSlot(ctx, account.ID, account.Concurrency)
	if err != nil {
		return nil, false, err
	}
	if result == nil || !result.Acquired {
		return result, false, nil
	}

	max := pairMax
	if max < 0 {
		max = account.PairMaxConcurrency(userID)
	}
	if max <= 0 && !trackOccupancy {
		return result, false, nil
	}

	pair, err := svc.AcquireAccountUserSlot(ctx, account.ID, userID, max)
	if err != nil {
		if result.ReleaseFunc != nil {
			result.ReleaseFunc()
		}
		return nil, false, err
	}
	if pair == nil || !pair.Acquired {
		if max <= 0 {
			// Count-only miss must not block the request or report pair_full.
			return result, false, nil
		}
		if result.ReleaseFunc != nil {
			result.ReleaseFunc()
		}
		return &AcquireResult{Acquired: false}, true, nil
	}

	result.ReleaseFunc = combineReleaseFuncs(result.ReleaseFunc, pair.ReleaseFunc)
	return result, false, nil
}

func (s *GatewayService) tryAcquireAccountAndPairSlot(ctx context.Context, account *Account) (*AcquireResult, bool, error) {
	if s == nil {
		return &AcquireResult{Acquired: true, ReleaseFunc: func() {}}, false, nil
	}
	pairMax, track := resolvePairSlotAcquire(ctx, account, s.smartScheduleCache)
	return acquireAccountAndPairSlot(ctx, s.concurrencyService, account, scheduleUserIDFromContext(ctx, 0), pairMax, track)
}

func (s *OpenAIGatewayService) tryAcquireAccountAndPairSlot(ctx context.Context, account *Account) (*AcquireResult, bool, error) {
	if s == nil {
		return &AcquireResult{Acquired: true, ReleaseFunc: func() {}}, false, nil
	}
	pairMax, track := resolvePairSlotAcquire(ctx, account, s.smartScheduleCache)
	return acquireAccountAndPairSlot(ctx, s.concurrencyService, account, scheduleUserIDFromContext(ctx, 0), pairMax, track)
}

func (s *GatewayService) pairCountsForSelection(ctx context.Context, accounts []*Account) map[int64]int {
	if s == nil {
		return map[int64]int{}
	}
	return loadPairConcurrencyCounts(ctx, s.concurrencyService, accounts, scheduleUserIDFromContext(ctx, 0), s.smartScheduleCache)
}

func (s *OpenAIGatewayService) pairCountsForSelection(ctx context.Context, accounts []*Account) map[int64]int {
	if s == nil {
		return map[int64]int{}
	}
	return loadPairConcurrencyCounts(ctx, s.concurrencyService, accounts, scheduleUserIDFromContext(ctx, 0), s.smartScheduleCache)
}

func markPairFullID(ids map[int64]struct{}, accountID int64) {
	if ids == nil || accountID <= 0 {
		return
	}
	ids[accountID] = struct{}{}
}

func skipPairFullWait(ctx context.Context, account *Account, pairCounts map[int64]int, pairFullIDs map[int64]struct{}, lookup SmartScheduleLookup) bool {
	if account == nil {
		return true
	}
	if _, ok := pairFullIDs[account.ID]; ok {
		return true
	}
	return isPairConcurrencyFull(ctx, account, pairCounts[account.ID], lookup)
}

// attachPairSlotHoldingAccount acquires the pair slot while already holding the
// account slot. pairFull releases the account slot and is true only for a real
// cap (pairMax>=1). Callers must not forward with only the account slot.
func attachPairSlotHoldingAccount(ctx context.Context, svc *ConcurrencyService, lookup SmartScheduleLookup, account *Account, accountRelease func()) (func(), bool, error) {
	if accountRelease == nil {
		accountRelease = func() {}
	}
	if account == nil {
		accountRelease()
		return nil, false, nil
	}
	if svc == nil {
		return accountRelease, false, nil
	}

	pairMax, track := resolvePairSlotAcquire(ctx, account, lookup)
	userID := scheduleUserIDFromContext(ctx, 0)
	max := pairMax
	if max < 0 {
		max = account.PairMaxConcurrency(userID)
	}
	if max <= 0 && !track {
		return accountRelease, false, nil
	}

	pair, err := svc.AcquireAccountUserSlot(ctx, account.ID, userID, max)
	if err != nil {
		accountRelease()
		return nil, false, err
	}
	if pair == nil || !pair.Acquired {
		if max <= 0 {
			return accountRelease, false, nil
		}
		accountRelease()
		return nil, true, nil
	}
	return combineReleaseFuncs(accountRelease, pair.ReleaseFunc), false, nil
}

// AttachPairSlotAfterAccountWait acquires the user×account pair slot after a
// WaitPlan woke with only the account slot. pairFull means the account slot
// was released and the caller must reselect.
func (s *GatewayService) AttachPairSlotAfterAccountWait(ctx context.Context, account *Account, accountRelease func()) (func(), bool, error) {
	if s == nil {
		if accountRelease == nil {
			return func() {}, false, nil
		}
		return accountRelease, false, nil
	}
	return attachPairSlotHoldingAccount(ctx, s.concurrencyService, s.smartScheduleCache, account, accountRelease)
}

// NewOpenAIGatewayServiceWithConcurrency builds a service that can attach
// pair slots. Used by handler tests that cannot set the unexported field.
func NewOpenAIGatewayServiceWithConcurrency(conc *ConcurrencyService) *OpenAIGatewayService {
	return &OpenAIGatewayService{concurrencyService: conc}
}

// AttachPairSlotAfterAccountWait acquires the user×account pair slot after a
// WaitPlan woke with only the account slot. pairFull means the account slot
// was released and the caller must reselect.
func (s *OpenAIGatewayService) AttachPairSlotAfterAccountWait(ctx context.Context, account *Account, accountRelease func()) (func(), bool, error) {
	if s == nil {
		if accountRelease == nil {
			return func() {}, false, nil
		}
		return accountRelease, false, nil
	}
	return attachPairSlotHoldingAccount(ctx, s.concurrencyService, s.smartScheduleCache, account, accountRelease)
}
