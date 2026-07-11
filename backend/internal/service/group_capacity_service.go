package service

import (
	"context"
	"time"
)

// GroupCapacitySummary holds aggregated capacity for a single group.
type GroupCapacitySummary struct {
	GroupID         int64 `json:"group_id"`
	ConcurrencyUsed int   `json:"concurrency_used"`
	ConcurrencyMax  int   `json:"concurrency_max"`
	SessionsUsed    int   `json:"sessions_used"`
	SessionsMax     int   `json:"sessions_max"`
	RPMUsed         int   `json:"rpm_used"`
	RPMMax          int   `json:"rpm_max"`
}

type GroupAccountCapacityRow struct {
	GroupID             int64
	AccountID           int64
	Concurrency         int
	Extra               map[string]any
	SessionWindowStart  *time.Time
	SessionWindowEnd    *time.Time
	SessionWindowStatus string
}

type groupCapacityActiveGroupIDLister interface {
	ListActiveIDs(ctx context.Context) ([]int64, error)
}

type groupCapacityAccountLister interface {
	ListSchedulableCapacityByGroupIDs(ctx context.Context, groupIDs []int64) ([]GroupAccountCapacityRow, error)
}

// GroupCapacityService aggregates per-group capacity from runtime data.
type GroupCapacityService struct {
	accountRepo        AccountRepository
	groupRepo          GroupRepository
	concurrencyService *ConcurrencyService
	sessionLimitCache  SessionLimitCache
	rpmCache           RPMCache
}

// NewGroupCapacityService creates a new GroupCapacityService.
func NewGroupCapacityService(
	accountRepo AccountRepository,
	groupRepo GroupRepository,
	concurrencyService *ConcurrencyService,
	sessionLimitCache SessionLimitCache,
	rpmCache RPMCache,
) *GroupCapacityService {
	return &GroupCapacityService{
		accountRepo:        accountRepo,
		groupRepo:          groupRepo,
		concurrencyService: concurrencyService,
		sessionLimitCache:  sessionLimitCache,
		rpmCache:           rpmCache,
	}
}

// GetAllGroupCapacity returns capacity summary for all active groups.
func (s *GroupCapacityService) GetAllGroupCapacity(ctx context.Context) ([]GroupCapacitySummary, error) {
	groupIDs, err := s.listActiveGroupIDs(ctx)
	if err != nil {
		return nil, err
	}
	if lister, ok := s.accountRepo.(groupCapacityAccountLister); ok {
		return s.getGroupCapacitiesBatch(ctx, groupIDs, lister)
	}
	return s.getGroupCapacitiesSequential(ctx, groupIDs), nil
}

func (s *GroupCapacityService) listActiveGroupIDs(ctx context.Context) ([]int64, error) {
	if lister, ok := s.groupRepo.(groupCapacityActiveGroupIDLister); ok {
		return lister.ListActiveIDs(ctx)
	}
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(groups))
	for i := range groups {
		ids = append(ids, groups[i].ID)
	}
	return ids, nil
}

func (s *GroupCapacityService) getGroupCapacitiesSequential(ctx context.Context, groupIDs []int64) []GroupCapacitySummary {
	results := make([]GroupCapacitySummary, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		cap, err := s.getGroupCapacity(ctx, groupID)
		if err != nil {
			continue
		}
		cap.GroupID = groupID
		results = append(results, cap)
	}
	return results
}

type groupCapacityAccountRef struct{ groupID, accountID int64 }

func (s *GroupCapacityService) getGroupCapacitiesBatch(ctx context.Context, groupIDs []int64, lister groupCapacityAccountLister) ([]GroupCapacitySummary, error) {
	results := make([]GroupCapacitySummary, len(groupIDs))
	groupIndex := make(map[int64]int, len(groupIDs))
	for i, id := range groupIDs {
		results[i].GroupID = id
		groupIndex[id] = i
	}
	if len(groupIDs) == 0 {
		return results, nil
	}
	rows, err := lister.ListSchedulableCapacityByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}

	refs := make([]groupCapacityAccountRef, 0, len(rows))
	seenRefs := make(map[groupCapacityAccountRef]struct{}, len(rows))
	seenAccounts := make(map[int64]struct{}, len(rows))
	accountIDs := make([]int64, 0, len(rows))
	sessionTimeouts := make(map[int64]time.Duration)
	for _, row := range rows {
		idx, ok := groupIndex[row.GroupID]
		if !ok || row.AccountID <= 0 {
			continue
		}
		ref := groupCapacityAccountRef{row.GroupID, row.AccountID}
		if _, ok := seenRefs[ref]; ok {
			continue
		}
		seenRefs[ref] = struct{}{}
		refs = append(refs, ref)
		if _, ok := seenAccounts[row.AccountID]; !ok {
			seenAccounts[row.AccountID] = struct{}{}
			accountIDs = append(accountIDs, row.AccountID)
		}
		acc := Account{ID: row.AccountID, Concurrency: row.Concurrency, Extra: row.Extra, SessionWindowStart: row.SessionWindowStart, SessionWindowEnd: row.SessionWindowEnd, SessionWindowStatus: row.SessionWindowStatus}
		results[idx].ConcurrencyMax += acc.Concurrency
		if max := acc.GetMaxSessions(); max > 0 {
			results[idx].SessionsMax += max
			timeout := time.Duration(acc.GetSessionIdleTimeoutMinutes()) * time.Minute
			if timeout <= 0 {
				timeout = 5 * time.Minute
			}
			sessionTimeouts[acc.ID] = timeout
		}
		if max := acc.GetBaseRPM(); max > 0 {
			results[idx].RPMMax += max
		}
	}
	if len(accountIDs) == 0 {
		return results, nil
	}

	concurrencyMap := map[int64]int{}
	if s.concurrencyService != nil {
		concurrencyMap, _ = s.concurrencyService.GetAccountConcurrencyBatch(ctx, accountIDs)
	}
	sessionIDs := accountIDsForCapacity(refs, groupIndex, results, func(v GroupCapacitySummary) bool { return v.SessionsMax > 0 })
	var sessionsMap map[int64]int
	if len(sessionIDs) > 0 && s.sessionLimitCache != nil {
		sessionsMap, _ = s.sessionLimitCache.GetActiveSessionCountBatch(ctx, sessionIDs, sessionTimeouts)
	}
	rpmIDs := accountIDsForCapacity(refs, groupIndex, results, func(v GroupCapacitySummary) bool { return v.RPMMax > 0 })
	var rpmMap map[int64]int
	if len(rpmIDs) > 0 && s.rpmCache != nil {
		rpmMap, _ = s.rpmCache.GetRPMBatch(ctx, rpmIDs)
	}
	for _, ref := range refs {
		idx := groupIndex[ref.groupID]
		results[idx].ConcurrencyUsed += concurrencyMap[ref.accountID]
		results[idx].SessionsUsed += sessionsMap[ref.accountID]
		results[idx].RPMUsed += rpmMap[ref.accountID]
	}
	return results, nil
}

func accountIDsForCapacity(refs []groupCapacityAccountRef, groupIndex map[int64]int, results []GroupCapacitySummary, include func(GroupCapacitySummary) bool) []int64 {
	seen := make(map[int64]struct{})
	ids := make([]int64, 0)
	for _, ref := range refs {
		idx, ok := groupIndex[ref.groupID]
		if !ok || !include(results[idx]) {
			continue
		}
		if _, ok := seen[ref.accountID]; ok {
			continue
		}
		seen[ref.accountID] = struct{}{}
		ids = append(ids, ref.accountID)
	}
	return ids
}

func (s *GroupCapacityService) getGroupCapacity(ctx context.Context, groupID int64) (GroupCapacitySummary, error) {
	accounts, err := s.accountRepo.ListSchedulableByGroupID(ctx, groupID)
	if err != nil {
		return GroupCapacitySummary{}, err
	}
	if len(accounts) == 0 {
		return GroupCapacitySummary{}, nil
	}

	// Collect account IDs and config values
	accountIDs := make([]int64, 0, len(accounts))
	sessionTimeouts := make(map[int64]time.Duration)
	var concurrencyMax, sessionsMax, rpmMax int

	for i := range accounts {
		acc := &accounts[i]
		accountIDs = append(accountIDs, acc.ID)
		concurrencyMax += acc.Concurrency

		if ms := acc.GetMaxSessions(); ms > 0 {
			sessionsMax += ms
			timeout := time.Duration(acc.GetSessionIdleTimeoutMinutes()) * time.Minute
			if timeout <= 0 {
				timeout = 5 * time.Minute
			}
			sessionTimeouts[acc.ID] = timeout
		}

		if rpm := acc.GetBaseRPM(); rpm > 0 {
			rpmMax += rpm
		}
	}

	// Batch query runtime data from Redis
	concurrencyMap, _ := s.concurrencyService.GetAccountConcurrencyBatch(ctx, accountIDs)

	var sessionsMap map[int64]int
	if sessionsMax > 0 && s.sessionLimitCache != nil {
		sessionsMap, _ = s.sessionLimitCache.GetActiveSessionCountBatch(ctx, accountIDs, sessionTimeouts)
	}

	var rpmMap map[int64]int
	if rpmMax > 0 && s.rpmCache != nil {
		rpmMap, _ = s.rpmCache.GetRPMBatch(ctx, accountIDs)
	}

	// Aggregate
	var concurrencyUsed, sessionsUsed, rpmUsed int
	for _, id := range accountIDs {
		concurrencyUsed += concurrencyMap[id]
		if sessionsMap != nil {
			sessionsUsed += sessionsMap[id]
		}
		if rpmMap != nil {
			rpmUsed += rpmMap[id]
		}
	}

	return GroupCapacitySummary{
		ConcurrencyUsed: concurrencyUsed,
		ConcurrencyMax:  concurrencyMax,
		SessionsUsed:    sessionsUsed,
		SessionsMax:     sessionsMax,
		RPMUsed:         rpmUsed,
		RPMMax:          rpmMax,
	}, nil
}
