package service

import (
	"context"
	"time"
)

// GroupCapacitySummary holds aggregated capacity for a single group.
//
// Semantics:
//   - ConcurrencyMax / SessionsMax / RPMMax: sum of schedulable account limits in the group
//     (account-side capacity pool available to the group).
//   - ConcurrencyUsed: live request count through this group's API keys (group-scoped).
//     Account-level concurrency is shared across groups and must NOT be used here.
//   - SessionsUsed / RPMUsed: only for accounts that configure the matching limit in this group
//     (still account-scoped runtime counters; admission is account-wide).
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

// GroupCapacityAPIKeyIDLister lists active API key IDs bound to groups.
type GroupCapacityAPIKeyIDLister interface {
	// ListActiveAPIKeyIDsByGroupIDs returns groupID -> active API key IDs.
	ListActiveAPIKeyIDsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]int64, error)
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
	apiKeyIDLister     GroupCapacityAPIKeyIDLister
}

// NewGroupCapacityService creates a new GroupCapacityService.
// apiKeyIDLister may be nil (concurrency used falls back to 0 rather than account totals).
func NewGroupCapacityService(
	accountRepo AccountRepository,
	groupRepo GroupRepository,
	concurrencyService *ConcurrencyService,
	sessionLimitCache SessionLimitCache,
	rpmCache RPMCache,
	apiKeyIDLister GroupCapacityAPIKeyIDLister,
) *GroupCapacityService {
	return &GroupCapacityService{
		accountRepo:        accountRepo,
		groupRepo:          groupRepo,
		concurrencyService: concurrencyService,
		sessionLimitCache:  sessionLimitCache,
		rpmCache:           rpmCache,
		apiKeyIDLister:     apiKeyIDLister,
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

type groupCapacityAccountRef struct {
	groupID, accountID int64
	hasSessions        bool
	hasRPM             bool
}

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
	seenRefs := make(map[struct{ g, a int64 }]struct{}, len(rows))
	sessionTimeouts := make(map[int64]time.Duration)
	for _, row := range rows {
		idx, ok := groupIndex[row.GroupID]
		if !ok || row.AccountID <= 0 {
			continue
		}
		key := struct{ g, a int64 }{row.GroupID, row.AccountID}
		if _, ok := seenRefs[key]; ok {
			continue
		}
		seenRefs[key] = struct{}{}

		acc := Account{ID: row.AccountID, Concurrency: row.Concurrency, Extra: row.Extra, SessionWindowStart: row.SessionWindowStart, SessionWindowEnd: row.SessionWindowEnd, SessionWindowStatus: row.SessionWindowStatus}
		results[idx].ConcurrencyMax += acc.Concurrency

		ref := groupCapacityAccountRef{groupID: row.GroupID, accountID: row.AccountID}
		if max := acc.GetMaxSessions(); max > 0 {
			results[idx].SessionsMax += max
			ref.hasSessions = true
			timeout := time.Duration(acc.GetSessionIdleTimeoutMinutes()) * time.Minute
			if timeout <= 0 {
				timeout = 5 * time.Minute
			}
			sessionTimeouts[acc.ID] = timeout
		}
		if max := acc.GetBaseRPM(); max > 0 {
			results[idx].RPMMax += max
			ref.hasRPM = true
		}
		refs = append(refs, ref)
	}

	// Group-scoped concurrency used: sum live API-key stats slots for keys in this group.
	s.fillGroupConcurrencyUsed(ctx, groupIDs, groupIndex, results)

	// Sessions / RPM: only accounts that configure the limit (not every account in the group).
	sessionIDs := accountIDsForCapacityFlag(refs, func(r groupCapacityAccountRef) bool { return r.hasSessions })
	var sessionsMap map[int64]int
	if len(sessionIDs) > 0 && s.sessionLimitCache != nil {
		sessionsMap, _ = s.sessionLimitCache.GetActiveSessionCountBatch(ctx, sessionIDs, sessionTimeouts)
	}
	rpmIDs := accountIDsForCapacityFlag(refs, func(r groupCapacityAccountRef) bool { return r.hasRPM })
	var rpmMap map[int64]int
	if len(rpmIDs) > 0 && s.rpmCache != nil {
		rpmMap, _ = s.rpmCache.GetRPMBatch(ctx, rpmIDs)
	}
	for _, ref := range refs {
		idx := groupIndex[ref.groupID]
		if ref.hasSessions {
			results[idx].SessionsUsed += sessionsMap[ref.accountID]
		}
		if ref.hasRPM {
			results[idx].RPMUsed += rpmMap[ref.accountID]
		}
	}
	return results, nil
}

func accountIDsForCapacityFlag(refs []groupCapacityAccountRef, include func(groupCapacityAccountRef) bool) []int64 {
	seen := make(map[int64]struct{})
	ids := make([]int64, 0)
	for _, ref := range refs {
		if !include(ref) {
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

// fillGroupConcurrencyUsed sets ConcurrencyUsed from per-API-key live request counts
// (group-scoped), not account-wide concurrency.
func (s *GroupCapacityService) fillGroupConcurrencyUsed(ctx context.Context, groupIDs []int64, groupIndex map[int64]int, results []GroupCapacitySummary) {
	if s.apiKeyIDLister == nil || s.concurrencyService == nil {
		return
	}
	byGroup, err := s.apiKeyIDLister.ListActiveAPIKeyIDsByGroupIDs(ctx, groupIDs)
	if err != nil || len(byGroup) == 0 {
		return
	}
	keyIDs := make([]int64, 0)
	seen := make(map[int64]struct{})
	for _, ids := range byGroup {
		for _, id := range ids {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			keyIDs = append(keyIDs, id)
		}
	}
	if len(keyIDs) == 0 {
		return
	}
	counts, err := s.concurrencyService.GetAPIKeyConcurrencyBatch(ctx, keyIDs)
	if err != nil || counts == nil {
		return
	}
	for groupID, ids := range byGroup {
		idx, ok := groupIndex[groupID]
		if !ok {
			continue
		}
		for _, keyID := range ids {
			results[idx].ConcurrencyUsed += counts[keyID]
		}
	}
}

func (s *GroupCapacityService) getGroupCapacity(ctx context.Context, groupID int64) (GroupCapacitySummary, error) {
	accounts, err := s.accountRepo.ListSchedulableByGroupID(ctx, groupID)
	if err != nil {
		return GroupCapacitySummary{}, err
	}

	sessionTimeouts := make(map[int64]time.Duration)
	sessionAccountIDs := make([]int64, 0)
	rpmAccountIDs := make([]int64, 0)
	var concurrencyMax, sessionsMax, rpmMax int

	for i := range accounts {
		acc := &accounts[i]
		concurrencyMax += acc.Concurrency

		if ms := acc.GetMaxSessions(); ms > 0 {
			sessionsMax += ms
			sessionAccountIDs = append(sessionAccountIDs, acc.ID)
			timeout := time.Duration(acc.GetSessionIdleTimeoutMinutes()) * time.Minute
			if timeout <= 0 {
				timeout = 5 * time.Minute
			}
			sessionTimeouts[acc.ID] = timeout
		}

		if rpm := acc.GetBaseRPM(); rpm > 0 {
			rpmMax += rpm
			rpmAccountIDs = append(rpmAccountIDs, acc.ID)
		}
	}

	var sessionsMap map[int64]int
	if len(sessionAccountIDs) > 0 && s.sessionLimitCache != nil {
		sessionsMap, _ = s.sessionLimitCache.GetActiveSessionCountBatch(ctx, sessionAccountIDs, sessionTimeouts)
	}

	var rpmMap map[int64]int
	if len(rpmAccountIDs) > 0 && s.rpmCache != nil {
		rpmMap, _ = s.rpmCache.GetRPMBatch(ctx, rpmAccountIDs)
	}

	var sessionsUsed, rpmUsed int
	for _, id := range sessionAccountIDs {
		sessionsUsed += sessionsMap[id]
	}
	for _, id := range rpmAccountIDs {
		rpmUsed += rpmMap[id]
	}

	out := GroupCapacitySummary{
		GroupID:        groupID,
		ConcurrencyMax: concurrencyMax,
		SessionsUsed:   sessionsUsed,
		SessionsMax:    sessionsMax,
		RPMUsed:        rpmUsed,
		RPMMax:         rpmMax,
	}
	// Group-scoped concurrency used via API keys.
	tmp := []GroupCapacitySummary{out}
	s.fillGroupConcurrencyUsed(ctx, []int64{groupID}, map[int64]int{groupID: 0}, tmp)
	out.ConcurrencyUsed = tmp[0].ConcurrencyUsed
	return out, nil
}
