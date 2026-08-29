package service

import (
	"context"
	"math"
	"sort"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	UnbindSubscriptionActionPreview            = "preview"
	UnbindSubscriptionActionSkipNoSubscription = "skip_no_subscription"
	UnbindSubscriptionActionSkipEmpty          = "skip_empty"
	UnbindSubscriptionActionApplied            = "applied"
	UnbindSubscriptionActionFailed             = "failed"
)

// UnbindSubscriptionGroupsByRateInput is the admin preview/apply payload.
type UnbindSubscriptionGroupsByRateInput struct {
	MinRateMultiplier float64
	Platform          string
	DryRun            bool
	AllowEmptyGroups  bool
}

// UnbindSubscriptionGroupRef is a group id/name pair in the preview table.
type UnbindSubscriptionGroupRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// UnbindSubscriptionGroupsByRateAccount is one matched account row.
type UnbindSubscriptionGroupsByRateAccount struct {
	ID           int64                         `json:"id"`
	Name         string                        `json:"name"`
	Platform     string                        `json:"platform"`
	Rate         float64                       `json:"rate"`
	Action       string                        `json:"action"`
	RemoveGroups []UnbindSubscriptionGroupRef  `json:"remove_groups"`
	KeepGroups   []UnbindSubscriptionGroupRef  `json:"keep_groups"`
	WouldBeEmpty bool                          `json:"would_be_empty"`
	Error        string                        `json:"error,omitempty"`
}

// UnbindSubscriptionGroupsByRateResult is the shared preview/apply response.
type UnbindSubscriptionGroupsByRateResult struct {
	Matched               int                                     `json:"matched"`
	WouldApply            int                                     `json:"would_apply"`
	SkippedNoSubscription int                                     `json:"skipped_no_subscription"`
	SkippedWouldBeEmpty   int                                     `json:"skipped_would_be_empty"`
	Applied               int                                     `json:"applied"`
	Failed                int                                     `json:"failed"`
	Accounts              []UnbindSubscriptionGroupsByRateAccount `json:"accounts"`
}

type unbindSubscriptionPlan struct {
	account      Account
	rate         float64
	remove       []UnbindSubscriptionGroupRef
	keep         []UnbindSubscriptionGroupRef
	keepIDs      []int64
	wouldBeEmpty bool
	skipNoSub    bool
	skipEmpty    bool
}

// UnbindSubscriptionGroupsByRate drops subscription (quota) groups from accounts
// whose BillingRateMultiplier() is strictly greater than min_rate. Preview
// (DryRun) never writes. Apply re-scans live DB and calls BindGroups per account.
func (s *adminServiceImpl) UnbindSubscriptionGroupsByRate(ctx context.Context, input *UnbindSubscriptionGroupsByRateInput) (*UnbindSubscriptionGroupsByRateResult, error) {
	if input == nil {
		return nil, infraerrors.BadRequest("INVALID_REQUEST", "request is required")
	}
	if math.IsNaN(input.MinRateMultiplier) || math.IsInf(input.MinRateMultiplier, 0) {
		return nil, infraerrors.BadRequest("INVALID_MIN_RATE_MULTIPLIER", "min_rate_multiplier must be a finite number")
	}

	plans, err := s.planUnbindSubscriptionGroupsByRate(ctx, input.MinRateMultiplier, input.Platform, input.AllowEmptyGroups)
	if err != nil {
		return nil, err
	}

	result := &UnbindSubscriptionGroupsByRateResult{
		Accounts: make([]UnbindSubscriptionGroupsByRateAccount, 0, len(plans)),
	}
	for _, plan := range plans {
		row := unbindSubscriptionRowFromPlan(plan)
		switch {
		case plan.skipNoSub:
			row.Action = UnbindSubscriptionActionSkipNoSubscription
			result.SkippedNoSubscription++
		case plan.skipEmpty:
			row.Action = UnbindSubscriptionActionSkipEmpty
			result.SkippedWouldBeEmpty++
		default:
			result.WouldApply++
			if input.DryRun {
				row.Action = UnbindSubscriptionActionPreview
			} else {
				if err := s.applyUnbindSubscriptionPlan(ctx, plan); err != nil {
					row.Action = UnbindSubscriptionActionFailed
					row.Error = err.Error()
					result.Failed++
				} else {
					row.Action = UnbindSubscriptionActionApplied
					result.Applied++
				}
			}
		}
		result.Matched++
		result.Accounts = append(result.Accounts, row)
	}
	return result, nil
}

func (s *adminServiceImpl) planUnbindSubscriptionGroupsByRate(ctx context.Context, minRate float64, platform string, allowEmpty bool) ([]unbindSubscriptionPlan, error) {
	platform = strings.TrimSpace(platform)
	accounts, err := s.accountRepo.ListAllWithFilters(ctx, platform, "", "", "", 0, "")
	if err != nil {
		return nil, err
	}

	plans := make([]unbindSubscriptionPlan, 0)
	for _, account := range accounts {
		rate := account.BillingRateMultiplier()
		if rate <= minRate {
			continue
		}
		remove, keep, keepIDs := classifyAccountGroupsForUnbind(account)
		plan := unbindSubscriptionPlan{
			account:      account,
			rate:         rate,
			remove:       remove,
			keep:         keep,
			keepIDs:      keepIDs,
			wouldBeEmpty: len(keepIDs) == 0,
		}
		if len(remove) == 0 {
			plan.skipNoSub = true
		} else if plan.wouldBeEmpty && !allowEmpty {
			plan.skipEmpty = true
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func (s *adminServiceImpl) applyUnbindSubscriptionPlan(ctx context.Context, plan unbindSubscriptionPlan) error {
	// Always pass a non-nil slice so BindGroups(id, []) is distinguishable from an omitted list.
	keepIDs := append([]int64{}, plan.keepIDs...)
	if err := s.validateAccountGroupBindings(ctx, plan.account.Platform, plan.account.Extra, keepIDs); err != nil {
		return err
	}
	return s.accountRepo.BindGroups(ctx, plan.account.ID, keepIDs)
}

func unbindSubscriptionRowFromPlan(plan unbindSubscriptionPlan) UnbindSubscriptionGroupsByRateAccount {
	remove := plan.remove
	if remove == nil {
		remove = []UnbindSubscriptionGroupRef{}
	}
	keep := plan.keep
	if keep == nil {
		keep = []UnbindSubscriptionGroupRef{}
	}
	return UnbindSubscriptionGroupsByRateAccount{
		ID:           plan.account.ID,
		Name:         plan.account.Name,
		Platform:     plan.account.Platform,
		Rate:         plan.rate,
		RemoveGroups: remove,
		KeepGroups:   keep,
		WouldBeEmpty: plan.wouldBeEmpty,
	}
}

func classifyAccountGroupsForUnbind(account Account) (remove, keep []UnbindSubscriptionGroupRef, keepIDs []int64) {
	remove = []UnbindSubscriptionGroupRef{}
	keep = []UnbindSubscriptionGroupRef{}
	keepIDs = []int64{}

	byID := make(map[int64]*Group, len(account.Groups)+len(account.AccountGroups))
	for _, group := range account.Groups {
		if group != nil {
			byID[group.ID] = group
		}
	}
	for i := range account.AccountGroups {
		ag := account.AccountGroups[i]
		if ag.Group != nil && byID[ag.GroupID] == nil {
			byID[ag.GroupID] = ag.Group
		}
	}

	orderedIDs := make([]int64, 0, len(account.GroupIDs)+len(account.Groups)+len(account.AccountGroups))
	seen := make(map[int64]struct{}, len(account.GroupIDs)+len(account.Groups)+len(account.AccountGroups))
	appendID := func(id int64) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		orderedIDs = append(orderedIDs, id)
	}
	for _, id := range account.GroupIDs {
		appendID(id)
	}
	for _, group := range account.Groups {
		if group != nil {
			appendID(group.ID)
		}
	}
	if len(account.GroupIDs) == 0 && len(account.Groups) == 0 && len(account.AccountGroups) > 0 {
		ags := append([]AccountGroup(nil), account.AccountGroups...)
		sort.SliceStable(ags, func(i, j int) bool {
			if ags[i].Priority != ags[j].Priority {
				return ags[i].Priority < ags[j].Priority
			}
			return ags[i].GroupID < ags[j].GroupID
		})
		for _, ag := range ags {
			appendID(ag.GroupID)
		}
	}

	for _, id := range orderedIDs {
		group := byID[id]
		ref := UnbindSubscriptionGroupRef{ID: id}
		drop := false
		if group != nil {
			ref.Name = group.Name
			drop = group.IsSubscriptionType()
		}
		if drop {
			remove = append(remove, ref)
			continue
		}
		keep = append(keep, ref)
		keepIDs = append(keepIDs, id)
	}
	return remove, keep, keepIDs
}
