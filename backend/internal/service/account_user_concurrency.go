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

// acquireAccountAndPairSlot acquires the account slot, then the pair slot when
// PairMaxConcurrency>=1. pairFull is true only when the pair cap rejected the
// acquire after the account slot succeeded (account slot is released).
func acquireAccountAndPairSlot(ctx context.Context, svc *ConcurrencyService, account *Account, userID int64, pairMax int) (*AcquireResult, bool, error) {
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
	if max <= 0 {
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
	return acquireAccountAndPairSlot(ctx, s.concurrencyService, account, scheduleUserIDFromContext(ctx, 0), resolvePairMaxConcurrency(ctx, account, s.smartScheduleCache))
}

func (s *OpenAIGatewayService) tryAcquireAccountAndPairSlot(ctx context.Context, account *Account) (*AcquireResult, bool, error) {
	if s == nil {
		return &AcquireResult{Acquired: true, ReleaseFunc: func() {}}, false, nil
	}
	return acquireAccountAndPairSlot(ctx, s.concurrencyService, account, scheduleUserIDFromContext(ctx, 0), resolvePairMaxConcurrency(ctx, account, s.smartScheduleCache))
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
