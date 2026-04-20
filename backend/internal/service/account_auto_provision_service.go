package service

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"sync"
	"time"
)

type accountAutoProvisionCreditsCacheEntry struct {
	fetchedAt time.Time
	value     float64
	ok        bool
}

const (
	accountAutoProvisionManualResetRequiredKey    = "auto_provision_manual_reset_required"
	accountAutoProvisionManualResetReasonKey      = "auto_provision_manual_reset_reason"
	accountAutoProvisionManualResetTriggeredAtKey = "auto_provision_manual_reset_at"
	accountAutoProvisionNeedsReauthCountKey       = "auto_provision_needs_reauth_count"
)

type AccountAutoProvisionService struct {
	accountRepo         AccountRepository
	groupRepo           GroupRepository
	adminService        AdminService
	settingService      *SettingService
	concurrencyService  *ConcurrencyService
	accountUsageService *AccountUsageService
	interval            time.Duration
	stopCh              chan struct{}
	stopOnce            sync.Once
	wg                  sync.WaitGroup

	mu          sync.Mutex
	lastRunAt   time.Time
	creditsByID map[string]accountAutoProvisionCreditsCacheEntry
}

func NewAccountAutoProvisionService(
	accountRepo AccountRepository,
	groupRepo GroupRepository,
	adminService AdminService,
	settingService *SettingService,
	concurrencyService *ConcurrencyService,
	accountUsageService *AccountUsageService,
	interval time.Duration,
) *AccountAutoProvisionService {
	return &AccountAutoProvisionService{
		accountRepo:         accountRepo,
		groupRepo:           groupRepo,
		adminService:        adminService,
		settingService:      settingService,
		concurrencyService:  concurrencyService,
		accountUsageService: accountUsageService,
		interval:            interval,
		stopCh:              make(chan struct{}),
		creditsByID:         make(map[string]accountAutoProvisionCreditsCacheEntry),
	}
}

func (s *AccountAutoProvisionService) Start() {
	if s == nil || s.accountRepo == nil || s.groupRepo == nil || s.adminService == nil || s.settingService == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *AccountAutoProvisionService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *AccountAutoProvisionService) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	settings, err := s.settingService.GetAccountAutoProvisionSettings(ctx)
	if err != nil {
		log.Printf("[AccountAutoProvision] failed to load settings: %v", err)
		return
	}
	if settings == nil || !settings.Enabled {
		return
	}
	if !s.shouldRunNow(settings.CheckIntervalSeconds) {
		return
	}

	state, err := s.settingService.GetAccountAutoProvisionState(ctx)
	if err != nil {
		log.Printf("[AccountAutoProvision] failed to load state: %v", err)
		state = DefaultAccountAutoProvisionState()
	}

	actedGroups := make(map[int64]struct{})
	actionsTaken := 0
	stateChanged := false
	now := time.Now()

	for _, rule := range settings.Rules {
		if !rule.Enabled || actionsTaken >= settings.MaxActionsPerRun {
			continue
		}
		for _, groupID := range rule.GroupIDs {
			if actionsTaken >= settings.MaxActionsPerRun {
				break
			}
			if _, exists := actedGroups[groupID]; exists {
				continue
			}
			remainingActions := settings.MaxActionsPerRun - actionsTaken
			changed, appliedCount := s.evaluateRuleForGroup(ctx, now, &rule, groupID, state, remainingActions)
			if changed {
				stateChanged = true
			}
			if appliedCount > 0 {
				actedGroups[groupID] = struct{}{}
				actionsTaken += appliedCount
			}
		}
	}

	if stateChanged {
		if err := s.settingService.SetAccountAutoProvisionState(ctx, state); err != nil {
			log.Printf("[AccountAutoProvision] failed to persist state: %v", err)
		}
	}
	s.markRunNow(now)
}

func (s *AccountAutoProvisionService) appendStateLog(
	state *AccountAutoProvisionState,
	level string,
	action string,
	rule *AccountAutoProvisionRule,
	group *Group,
	account *Account,
	message string,
) {
	if state == nil {
		return
	}
	entry := AccountAutoProvisionLogEntry{
		OccurredAt: time.Now(),
		Level:      level,
		Action:     action,
		Message:    message,
	}
	if rule != nil {
		entry.RuleID = rule.ID
		entry.RuleName = rule.Name
	}
	if group != nil {
		entry.GroupID = group.ID
		entry.GroupName = group.Name
	}
	if account != nil {
		entry.AccountID = account.ID
		entry.AccountName = account.Name
	}
	state.RecentLogs = append([]AccountAutoProvisionLogEntry{entry}, state.RecentLogs...)
	if len(state.RecentLogs) > 200 {
		state.RecentLogs = state.RecentLogs[:200]
	}
}

func (s *AccountAutoProvisionService) evaluateRuleForGroup(
	ctx context.Context,
	now time.Time,
	rule *AccountAutoProvisionRule,
	groupID int64,
	state *AccountAutoProvisionState,
	maxActions int,
) (bool, int) {
	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		log.Printf("[AccountAutoProvision] rule=%s group=%d load group failed: %v", rule.ID, groupID, err)
		s.appendStateLog(state, "error", "load_group_failed", rule, &Group{ID: groupID}, nil, err.Error())
		return false, 0
	}
	if group == nil || !group.IsActive() {
		return false, 0
	}

	accounts, err := s.accountRepo.ListByGroup(ctx, groupID)
	if err != nil {
		log.Printf("[AccountAutoProvision] rule=%s group=%d load accounts failed: %v", rule.ID, groupID, err)
		s.appendStateLog(state, "error", "load_group_accounts_failed", rule, group, nil, err.Error())
		return false, 0
	}

	healthy, lowCreditRemovedCount, usageRemovedCount, stateChanged := s.reconcileMonitoredGroupAccounts(ctx, now, rule, group, accounts, state)
	if snapshot, ok := buildLastHealthySnapshot(healthy); ok {
		key := strconv.FormatInt(groupID, 10)
		current, exists := state.LastHealthySnapshots[key]
		if !exists || !accountAutoProvisionSnapshotEqual(current, snapshot) {
			state.LastHealthySnapshots[key] = snapshot
			stateChanged = true
		}
	}

	normalCount := len(healthy)
	concurrencyUsed, concurrencyMax := s.measureConcurrency(ctx, healthy)

	triggerReasons := make([]string, 0, 3)
	normalDeficit := 0
	if rule.NormalAccountCountBelow > 0 && normalCount < rule.NormalAccountCountBelow {
		normalDeficit = rule.NormalAccountCountBelow - normalCount
		triggerReasons = append(triggerReasons, fmt.Sprintf("normal_count=%d<threshold=%d", normalCount, rule.NormalAccountCountBelow))
	}
	concurrencyTriggered := false
	if rule.ConcurrencyUtilizationAbove > 0 && concurrencyMax > 0 {
		utilization := (float64(concurrencyUsed) / float64(concurrencyMax)) * 100
		if utilization >= rule.ConcurrencyUtilizationAbove {
			concurrencyTriggered = true
			triggerReasons = append(triggerReasons, fmt.Sprintf("concurrency=%.2f%%>=threshold=%.2f%%", utilization, rule.ConcurrencyUtilizationAbove))
		}
	}
	lowCreditCount := 0
	if lowCreditRemovedCount > 0 {
		lowCreditCount = lowCreditRemovedCount
		triggerReasons = append(triggerReasons, fmt.Sprintf("ai_credits_low_removed=%d<threshold=%.2f", lowCreditRemovedCount, rule.AICreditsBelow))
	}
	if usageRemovedCount > 0 {
		triggerReasons = append(triggerReasons, fmt.Sprintf("usage_window_removed=%d", usageRemovedCount))
	}
	if len(triggerReasons) == 0 {
		return stateChanged, 0
	}

	triggerKey := accountAutoProvisionTriggerKey(rule.ID, groupID)
	if cooldownActive(state.LastTriggered[triggerKey], now, rule.CooldownMinutes) {
		return stateChanged, 0
	}

	template := rule.Template
	if rule.ProvisionMode == AccountAutoProvisionModeCloneLastHealth {
		if snapshot, ok := state.LastHealthySnapshots[strconv.FormatInt(groupID, 10)]; ok {
			template = snapshot.Template
		} else {
			log.Printf("[AccountAutoProvision] rule=%s group=%d has no healthy snapshot, fallback to explicit template", rule.ID, groupID)
		}
	}

	missingCount := computeProvisionCount(normalDeficit, concurrencyTriggered, maxInt(lowCreditCount, usageRemovedCount))
	if maxActions > 0 && missingCount > maxActions {
		missingCount = maxActions
	}
	if missingCount <= 0 {
		return stateChanged, 0
	}

	candidates, err := s.accountRepo.ListSchedulableUngroupedByPlatform(ctx, group.Platform)
	if err != nil {
		log.Printf("[AccountAutoProvision] rule=%s group=%d load candidates failed: %v", rule.ID, groupID, err)
		s.appendStateLog(state, "error", "load_candidates_failed", rule, group, nil, err.Error())
		return stateChanged, 0
	}
	appliedCount := 0
	remaining := buildLocalProvisionCandidates(group, candidates)
	for appliedCount < missingCount {
		candidate := selectUngroupedCandidate(remaining)
		if candidate == nil {
			if appliedCount == 0 {
				log.Printf("[AccountAutoProvision] rule=%s group=%d triggered but no ungrouped candidate is available (%v)", rule.ID, groupID, triggerReasons)
				s.appendStateLog(state, "warn", "no_candidate_available", rule, group, nil, joinRemovalReasons(triggerReasons))
			}
			break
		}
		if reasons, ok := s.validateCandidateForProvision(ctx, now, rule, group, candidate); ok {
			log.Printf("[AccountAutoProvision] rule=%s group=%d skipped candidate=%d name=%s reasons=%s", rule.ID, groupID, candidate.ID, candidate.Name, joinRemovalReasons(reasons))
			s.appendStateLog(state, "info", "candidate_skipped", rule, group, candidate, joinRemovalReasons(reasons))
			remaining = removeCandidateByID(remaining, candidate.ID)
			continue
		}
		if err := s.applyProvisionTemplate(ctx, groupID, candidate.ID, group.Platform, template); err != nil {
			log.Printf("[AccountAutoProvision] rule=%s group=%d candidate=%d apply failed: %v", rule.ID, groupID, candidate.ID, err)
			s.appendStateLog(state, "error", "candidate_apply_failed", rule, group, candidate, err.Error())
			break
		}
		s.appendStateLog(state, "info", "candidate_applied", rule, group, candidate, "candidate provisioned into monitored group")
		appliedCount++
		remaining = removeCandidateByID(remaining, candidate.ID)
	}
	if appliedCount == 0 {
		return stateChanged, 0
	}
	state.LastTriggered[triggerKey] = now
	stateChanged = true
	log.Printf("[AccountAutoProvision] rule=%s group=%d applied=%d desired=%d reasons=%v", rule.ID, groupID, appliedCount, missingCount, triggerReasons)
	s.appendStateLog(state, "info", "group_provisioned", rule, group, nil, fmt.Sprintf("applied=%d desired=%d reasons=%s", appliedCount, missingCount, joinRemovalReasons(triggerReasons)))
	return stateChanged, appliedCount
}

func (s *AccountAutoProvisionService) applyProvisionTemplate(
	ctx context.Context,
	groupID int64,
	accountID int64,
	platform string,
	template AccountAutoProvisionTemplate,
) error {
	account, err := s.adminService.GetAccount(ctx, accountID)
	if err != nil {
		return err
	}
	if account == nil {
		return fmt.Errorf("candidate account not found")
	}

	groupIDs := []int64{groupID}
	input := &UpdateAccountInput{
		GroupIDs:                              &groupIDs,
		Concurrency:                           intPtr(template.Concurrency),
		Priority:                              cloneIntPtr(template.Priority),
		LoadFactor:                            cloneIntPtr(template.LoadFactor),
		ProxyID:                               cloneInt64Ptr(template.ProxyID),
		SuppressAutoProvisionManualResetClear: true,
	}

	if platform == PlatformAntigravity {
		extra := cloneAnyMap(account.Extra)
		if template.AllowOverages {
			extra["allow_overages"] = true
		} else {
			delete(extra, "allow_overages")
		}
		input.Extra = extra
	}

	if _, err := s.adminService.UpdateAccount(ctx, accountID, input); err != nil {
		return err
	}

	if account.Schedulable != template.Schedulable {
		if _, err := s.adminService.SetAccountSchedulable(ctx, accountID, template.Schedulable); err != nil {
			return err
		}
	}
	return nil
}

func (s *AccountAutoProvisionService) measureConcurrency(ctx context.Context, accounts []Account) (int, int) {
	if len(accounts) == 0 {
		return 0, 0
	}
	accountIDs := make([]int64, 0, len(accounts))
	concurrencyMax := 0
	for _, account := range accounts {
		accountIDs = append(accountIDs, account.ID)
		concurrencyMax += account.Concurrency
	}
	if s.concurrencyService == nil {
		return 0, concurrencyMax
	}
	concurrencyMap, err := s.concurrencyService.GetAccountConcurrencyBatch(ctx, accountIDs)
	if err != nil {
		return 0, concurrencyMax
	}
	concurrencyUsed := 0
	for _, accountID := range accountIDs {
		concurrencyUsed += concurrencyMap[accountID]
	}
	return concurrencyUsed, concurrencyMax
}

func (s *AccountAutoProvisionService) getAccountAICredits(
	ctx context.Context,
	rule *AccountAutoProvisionRule,
	accountID int64,
) (float64, bool) {
	cacheKey := fmt.Sprintf("%s:%d", rule.ID, accountID)
	now := time.Now()

	s.mu.Lock()
	entry, exists := s.creditsByID[cacheKey]
	s.mu.Unlock()
	if exists && now.Sub(entry.fetchedAt) < time.Duration(rule.AICreditsCheckIntervalMinutes)*time.Minute {
		return entry.value, entry.ok
	}

	value, ok := s.fetchAccountAICredits(ctx, accountID)

	s.mu.Lock()
	s.creditsByID[cacheKey] = accountAutoProvisionCreditsCacheEntry{
		fetchedAt: now,
		value:     value,
		ok:        ok,
	}
	s.mu.Unlock()
	return value, ok
}

func (s *AccountAutoProvisionService) fetchAccountAICredits(ctx context.Context, accountID int64) (float64, bool) {
	if s.accountUsageService == nil {
		return 0, false
	}

	total := 0.0
	usage, err := s.accountUsageService.GetUsage(ctx, accountID)
	if err != nil || usage == nil {
		return 0, false
	}
	if len(usage.AICredits) == 0 {
		return 0, false
	}
	for _, credit := range usage.AICredits {
		total += credit.Amount
	}
	return total, true
}

func buildLocalProvisionCandidates(group *Group, candidates []Account) []Account {
	filtered := make([]Account, 0, len(candidates))
	for _, candidate := range candidates {
		if isAccountAutoProvisionManualResetRequired(&candidate) {
			continue
		}
		if group.RequireOAuthOnly && candidate.Type == AccountTypeAPIKey {
			continue
		}
		if group.RequirePrivacySet && !candidate.IsPrivacySet() {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func selectUngroupedCandidate(candidates []Account) *Account {
	if len(candidates) == 0 {
		return nil
	}
	filtered := make([]Account, 0, len(candidates))
	for _, candidate := range candidates {
		if isAccountAutoProvisionManualResetRequired(&candidate) {
			continue
		}
		filtered = append(filtered, candidate)
	}
	if len(filtered) == 0 {
		return nil
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Priority != filtered[j].Priority {
			return filtered[i].Priority < filtered[j].Priority
		}
		return filtered[i].ID < filtered[j].ID
	})
	return &filtered[0]
}

func removeCandidateByID(candidates []Account, accountID int64) []Account {
	filtered := make([]Account, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ID == accountID {
			continue
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func (s *AccountAutoProvisionService) validateCandidateForProvision(
	ctx context.Context,
	now time.Time,
	rule *AccountAutoProvisionRule,
	group *Group,
	account *Account,
) ([]string, bool) {
	if account == nil {
		return []string{"candidate_nil"}, true
	}
	reasons := make([]string, 0, 6)
	if rule.AICreditsBelow > 0 && group.Platform == PlatformAntigravity {
		credits, ok := s.getAccountAICredits(ctx, rule, account.ID)
		if !ok {
			reasons = append(reasons, "candidate_ai_credits_unknown")
		} else if credits < rule.AICreditsBelow {
			reasons = append(reasons, fmt.Sprintf("candidate_ai_credits=%.2f<threshold=%.2f", credits, rule.AICreditsBelow))
		}
	}
	if usageReasons, ok := s.collectUsageRemovalReasons(ctx, now, account); ok {
		for _, reason := range usageReasons {
			reasons = append(reasons, "candidate_"+reason)
		}
	}
	return reasons, len(reasons) > 0
}

func (s *AccountAutoProvisionService) reconcileMonitoredGroupAccounts(
	ctx context.Context,
	now time.Time,
	rule *AccountAutoProvisionRule,
	group *Group,
	accounts []Account,
	state *AccountAutoProvisionState,
) ([]Account, int, int, bool) {
	healthy := make([]Account, 0, len(accounts))
	lowCreditRemovedCount := 0
	usageRemovedCount := 0
	stateChanged := false
	for _, account := range accounts {
		if !isAccountEligibleForGroup(&account, group) {
			if s.removeAccountFromMonitoredGroup(ctx, now, rule, group, &account, "unhealthy") {
				stateChanged = true
				s.appendStateLog(state, "warn", "account_removed", rule, group, &account, "unhealthy")
			}
			continue
		}
		if rule.AICreditsBelow > 0 && group.Platform == PlatformAntigravity {
			credits, ok := s.getAccountAICredits(ctx, rule, account.ID)
			if ok && credits < rule.AICreditsBelow {
				message := fmt.Sprintf("ai_credits=%.2f<threshold=%.2f", credits, rule.AICreditsBelow)
				if s.removeAccountFromMonitoredGroup(ctx, now, rule, group, &account, message) {
					stateChanged = true
					lowCreditRemovedCount++
					s.appendStateLog(state, "warn", "account_removed", rule, group, &account, message)
				}
				continue
			}
		}
		if reasons, ok := s.collectUsageRemovalReasons(ctx, now, &account); ok {
			message := joinRemovalReasons(reasons)
			if s.removeAccountFromMonitoredGroup(ctx, now, rule, group, &account, message) {
				stateChanged = true
				usageRemovedCount++
				s.appendStateLog(state, "warn", "account_removed", rule, group, &account, message)
			}
			continue
		}
		healthy = append(healthy, account)
	}
	return healthy, lowCreditRemovedCount, usageRemovedCount, stateChanged
}

func (s *AccountAutoProvisionService) removeAccountFromMonitoredGroup(
	ctx context.Context,
	now time.Time,
	rule *AccountAutoProvisionRule,
	group *Group,
	account *Account,
	reason string,
) bool {
	if account == nil || group == nil {
		return false
	}
	remainingGroupIDs := make([]int64, 0, len(account.GroupIDs))
	for _, currentGroupID := range account.GroupIDs {
		if currentGroupID == group.ID {
			continue
		}
		remainingGroupIDs = append(remainingGroupIDs, currentGroupID)
	}
	extra := cloneAnyMap(account.Extra)
	extra[accountAutoProvisionManualResetRequiredKey] = true
	extra[accountAutoProvisionManualResetReasonKey] = reason
	extra[accountAutoProvisionManualResetTriggeredAtKey] = now.UTC().Format(time.RFC3339)
	input := &UpdateAccountInput{
		GroupIDs:                              &remainingGroupIDs,
		Extra:                                 extra,
		SuppressAutoProvisionManualResetClear: true,
	}
	if _, err := s.adminService.UpdateAccount(ctx, account.ID, input); err != nil {
		log.Printf("[AccountAutoProvision] rule=%s group=%d remove account=%d failed: %v", rule.ID, group.ID, account.ID, err)
		return false
	}
	log.Printf("[AccountAutoProvision] rule=%s group=%d removed account=%d name=%s reason=%s manual_reset_required=true", rule.ID, group.ID, account.ID, account.Name, reason)
	return true
}

func computeProvisionCount(normalDeficit int, concurrencyTriggered bool, removedCount int) int {
	missingCount := normalDeficit
	if removedCount > missingCount {
		missingCount = removedCount
	}
	if concurrencyTriggered && missingCount < 1 {
		missingCount = 1
	}
	if missingCount < 1 && (normalDeficit > 0 || removedCount > 0) {
		missingCount = 1
	}
	return missingCount
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func (s *AccountAutoProvisionService) collectUsageRemovalReasons(
	ctx context.Context,
	now time.Time,
	account *Account,
) ([]string, bool) {
	if account == nil || s.accountUsageService == nil {
		return nil, false
	}
	usage, err := s.accountUsageService.GetUsage(ctx, account.ID)
	if err != nil || usage == nil {
		return nil, false
	}

	reasons := make([]string, 0, 8)
	if usage.IsForbidden {
		if usage.ForbiddenType != "" {
			reasons = append(reasons, fmt.Sprintf("usage_forbidden_type=%s", usage.ForbiddenType))
		} else {
			reasons = append(reasons, "usage_forbidden=true")
		}
	}
	if usage.IsBanned {
		reasons = append(reasons, "usage_is_banned=true")
	}
	if usage.NeedsVerify {
		reasons = append(reasons, "usage_needs_verify=true")
	}
	if usage.NeedsReauth {
		if reauthReason, shouldRemove := s.evaluateNeedsReauthRemoval(ctx, account); shouldRemove {
			reasons = append(reasons, reauthReason)
		}
	} else {
		s.resetNeedsReauthCount(ctx, account)
	}
	if usage.ErrorCode == errorCodeForbidden && !containsReason(reasons, "usage_forbidden") {
		reasons = append(reasons, "usage_error_code=forbidden")
	}
	if usageWindowExhausted(now, usage.FiveHour) {
		reasons = append(reasons, fmt.Sprintf("five_hour_utilization=%.2f>=100", usage.FiveHour.Utilization))
	}
	if usageWindowExhausted(now, usage.SevenDay) {
		reasons = append(reasons, fmt.Sprintf("seven_day_utilization=%.2f>=100", usage.SevenDay.Utilization))
	}
	if usageWindowExhausted(now, usage.SevenDaySonnet) {
		reasons = append(reasons, fmt.Sprintf("seven_day_sonnet_utilization=%.2f>=100", usage.SevenDaySonnet.Utilization))
	}
	return reasons, len(reasons) > 0
}

func usageWindowExhausted(now time.Time, progress *UsageProgress) bool {
	if progress == nil {
		return false
	}
	if progress.Utilization < 100 {
		return false
	}
	if progress.ResetsAt != nil && !now.Before(*progress.ResetsAt) {
		return false
	}
	return true
}

func joinRemovalReasons(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	joined := reasons[0]
	for i := 1; i < len(reasons); i++ {
		joined += "; " + reasons[i]
	}
	return joined
}

func containsReason(reasons []string, needle string) bool {
	for _, reason := range reasons {
		if len(reason) >= len(needle) && reason[:len(needle)] == needle {
			return true
		}
	}
	return false
}

func (s *AccountAutoProvisionService) evaluateNeedsReauthRemoval(ctx context.Context, account *Account) (string, bool) {
	if account == nil {
		return "", false
	}
	hasRefreshToken := account.GetCredential("refresh_token") != ""
	currentCount := getAccountAutoProvisionNeedsReauthCount(account)

	if hasRefreshToken {
		if currentCount < 3 {
			s.setNeedsReauthCount(ctx, account, currentCount+1)
			return "", false
		}
		return fmt.Sprintf("usage_needs_reauth=true; consecutive_count=%d; refresh_token_present=true", currentCount), true
	}
	return fmt.Sprintf("usage_needs_reauth=true; consecutive_count=%d; refresh_token_present=false", currentCount), true
}

func getAccountAutoProvisionNeedsReauthCount(account *Account) int {
	if account == nil || account.Extra == nil {
		return 1
	}
	raw, ok := account.Extra[accountAutoProvisionNeedsReauthCountKey]
	if !ok {
		return 1
	}
	switch value := raw.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 1
	}
}

func setAccountAutoProvisionNeedsReauthCount(account *Account, count int) {
	if account == nil {
		return
	}
	account.Extra = cloneAnyMap(account.Extra)
	account.Extra[accountAutoProvisionNeedsReauthCountKey] = count
}

func (s *AccountAutoProvisionService) setNeedsReauthCount(ctx context.Context, account *Account, count int) {
	if account == nil || s.accountRepo == nil {
		return
	}
	setAccountAutoProvisionNeedsReauthCount(account, count)
	if err := s.accountRepo.Update(ctx, account); err != nil {
		log.Printf("[AccountAutoProvision] persist needs_reauth_count account=%d failed: %v", account.ID, err)
	}
}

func (s *AccountAutoProvisionService) resetNeedsReauthCount(ctx context.Context, account *Account) {
	if account == nil || account.Extra == nil || s.accountRepo == nil {
		return
	}
	if _, ok := account.Extra[accountAutoProvisionNeedsReauthCountKey]; !ok {
		return
	}
	account.Extra = cloneAnyMap(account.Extra)
	delete(account.Extra, accountAutoProvisionNeedsReauthCountKey)
	if err := s.accountRepo.Update(ctx, account); err != nil {
		log.Printf("[AccountAutoProvision] reset needs_reauth_count account=%d failed: %v", account.ID, err)
	}
}

func isAccountEligibleForGroup(account *Account, group *Group) bool {
	if account == nil || group == nil {
		return false
	}
	if !account.IsSchedulable() {
		return false
	}
	if group.RequireOAuthOnly && account.Type == AccountTypeAPIKey {
		return false
	}
	if group.RequirePrivacySet && !account.IsPrivacySet() {
		return false
	}
	return true
}

func isAccountAutoProvisionManualResetRequired(account *Account) bool {
	if account == nil || account.Extra == nil {
		return false
	}
	raw, ok := account.Extra[accountAutoProvisionManualResetRequiredKey]
	if !ok {
		return false
	}
	flag, ok := raw.(bool)
	return ok && flag
}

func clearAccountAutoProvisionManualResetMarker(account *Account) bool {
	if account == nil || account.Extra == nil {
		return false
	}
	_, hasFlag := account.Extra[accountAutoProvisionManualResetRequiredKey]
	_, hasReason := account.Extra[accountAutoProvisionManualResetReasonKey]
	_, hasAt := account.Extra[accountAutoProvisionManualResetTriggeredAtKey]
	if !hasFlag && !hasReason && !hasAt {
		return false
	}
	account.Extra = cloneAnyMap(account.Extra)
	delete(account.Extra, accountAutoProvisionManualResetRequiredKey)
	delete(account.Extra, accountAutoProvisionManualResetReasonKey)
	delete(account.Extra, accountAutoProvisionManualResetTriggeredAtKey)
	return true
}

func buildLastHealthySnapshot(accounts []Account) (AccountAutoProvisionSnapshot, bool) {
	if len(accounts) == 0 {
		return AccountAutoProvisionSnapshot{}, false
	}
	best := accounts[0]
	for _, candidate := range accounts[1:] {
		if isMoreRecentHealthyCandidate(candidate, best) {
			best = candidate
		}
	}
	return AccountAutoProvisionSnapshot{
		SourceAccountID:   best.ID,
		SourceAccountName: best.Name,
		CapturedAt:        time.Now(),
		Template: AccountAutoProvisionTemplate{
			ProxyID:       cloneInt64Ptr(best.ProxyID),
			Concurrency:   best.Concurrency,
			Priority:      autoProvisionIntPtr(best.Priority),
			LoadFactor:    cloneIntPtr(best.LoadFactor),
			Schedulable:   best.Schedulable,
			AllowOverages: best.IsOveragesEnabled(),
		},
	}, true
}

func isMoreRecentHealthyCandidate(left, right Account) bool {
	if left.LastUsedAt != nil && right.LastUsedAt != nil {
		if !left.LastUsedAt.Equal(*right.LastUsedAt) {
			return left.LastUsedAt.After(*right.LastUsedAt)
		}
	}
	if left.LastUsedAt != nil && right.LastUsedAt == nil {
		return true
	}
	if left.LastUsedAt == nil && right.LastUsedAt != nil {
		return false
	}
	if left.Priority != right.Priority {
		return left.Priority < right.Priority
	}
	return left.ID < right.ID
}

func accountAutoProvisionSnapshotEqual(left, right AccountAutoProvisionSnapshot) bool {
	return left.SourceAccountID == right.SourceAccountID &&
		left.SourceAccountName == right.SourceAccountName &&
		accountAutoProvisionTemplateEqual(left.Template, right.Template)
}

func accountAutoProvisionTemplateEqual(left, right AccountAutoProvisionTemplate) bool {
	return int64PtrEqual(left.ProxyID, right.ProxyID) &&
		left.Concurrency == right.Concurrency &&
		intPtrEqual(left.Priority, right.Priority) &&
		intPtrEqual(left.LoadFactor, right.LoadFactor) &&
		left.Schedulable == right.Schedulable &&
		left.AllowOverages == right.AllowOverages
}

func cooldownActive(lastTriggered time.Time, now time.Time, cooldownMinutes int) bool {
	if cooldownMinutes <= 0 || lastTriggered.IsZero() {
		return false
	}
	return now.Before(lastTriggered.Add(time.Duration(cooldownMinutes) * time.Minute))
}

func accountAutoProvisionTriggerKey(ruleID string, groupID int64) string {
	return fmt.Sprintf("%s:%d", ruleID, groupID)
}

func (s *AccountAutoProvisionService) shouldRunNow(checkIntervalSeconds int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastRunAt.IsZero() {
		return true
	}
	return time.Since(s.lastRunAt) >= time.Duration(checkIntervalSeconds)*time.Second
}

func (s *AccountAutoProvisionService) markRunNow(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastRunAt = now
}

func cloneAnyMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return map[string]any{}
	}
	target := make(map[string]any, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func autoProvisionIntPtr(value int) *int {
	return &value
}

func intPtrEqual(left, right *int) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func int64PtrEqual(left, right *int64) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}
