package service

import (
	"container/heap"
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	openAIAccountScheduleLayerPreviousResponse = "previous_response_id"
	openAIAccountScheduleLayerSessionSticky    = "session_hash"
	openAIAccountScheduleLayerLoadBalance      = "load_balance"
	openAIAdvancedSchedulerSettingKey          = "openai_advanced_scheduler_enabled"
)

const (
	openAIAdvancedSchedulerSettingCacheTTL  = 5 * time.Second
	openAIAdvancedSchedulerSettingDBTimeout = 2 * time.Second
)

const (
	openAIQuotaHeadroomNeutralFactor      = 0.5
	openAIQuotaHeadroomSecondaryLowRemain = 0.10
	openAIQuotaHeadroomSnapshotStaleAfter = 8 * time.Hour
)

type cachedOpenAIAdvancedSchedulerSetting struct {
	enabled                     bool
	stickyWeightedEnabled       bool
	subscriptionPriorityEnabled bool
	lbTopKOverride              int
	weightOverrides             map[string]float64
	expiresAt                   int64
}

type openAIAdvancedSchedulerRuntimeSettings struct {
	enabled                     bool
	stickyWeightedEnabled       bool
	subscriptionPriorityEnabled bool
	lbTopKOverride              int
	weightOverrides             map[string]float64
}

var openAIAdvancedSchedulerSettingCache atomic.Value // *cachedOpenAIAdvancedSchedulerSetting
var openAIAdvancedSchedulerSettingSF singleflight.Group

type OpenAIAccountScheduleRequest struct {
	GroupID                 *int64
	Platform                string
	SessionHash             string
	StickyAccountID         int64
	StickyPreviousAccountID int64
	StickyWeighted          bool
	SubscriptionPriority    bool
	PreserveStickyBinding   bool
	PreviousResponseID      string
	PreviousResponseCanMove bool
	RequestedModel          string
	RequiredTransport       OpenAIUpstreamTransport
	RequiredCapability      OpenAIEndpointCapability
	RequiredImageCapability OpenAIImagesCapability
	RequireCompact          bool
	RequireClaudeGPTBridge  bool
	ExcludedIDs             map[int64]struct{}
}

type OpenAIAccountScheduleDecision struct {
	Layer               string
	StickyPreviousHit   bool
	StickySessionHit    bool
	CandidateCount      int
	TopK                int
	LatencyMs           int64
	LoadSkew            float64
	SelectedAccountID   int64
	SelectedAccountType string
}

type OpenAIAccountSchedulerMetricsSnapshot struct {
	SelectTotal              int64
	StickyPreviousHitTotal   int64
	StickySessionHitTotal    int64
	LoadBalanceSelectTotal   int64
	AccountSwitchTotal       int64
	SchedulerLatencyMsTotal  int64
	SchedulerLatencyMsAvg    float64
	StickyHitRatio           float64
	AccountSwitchRate        float64
	LoadSkewAvg              float64
	RuntimeStatsAccountCount int
}

type OpenAIAccountScheduler interface {
	Select(ctx context.Context, req OpenAIAccountScheduleRequest) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error)
	ReportResult(accountID int64, success bool, firstTokenMs *int)
	ReportSwitch()
	SnapshotMetrics() OpenAIAccountSchedulerMetricsSnapshot
}

type openAIAccountSchedulerMetrics struct {
	selectTotal            atomic.Int64
	stickyPreviousHitTotal atomic.Int64
	stickySessionHitTotal  atomic.Int64
	loadBalanceSelectTotal atomic.Int64
	accountSwitchTotal     atomic.Int64
	latencyMsTotal         atomic.Int64
	loadSkewMilliTotal     atomic.Int64
}

func (m *openAIAccountSchedulerMetrics) recordSelect(decision OpenAIAccountScheduleDecision) {
	if m == nil {
		return
	}
	m.selectTotal.Add(1)
	m.latencyMsTotal.Add(decision.LatencyMs)
	m.loadSkewMilliTotal.Add(int64(math.Round(decision.LoadSkew * 1000)))
	if decision.StickyPreviousHit {
		m.stickyPreviousHitTotal.Add(1)
	}
	if decision.StickySessionHit {
		m.stickySessionHitTotal.Add(1)
	}
	if decision.Layer == openAIAccountScheduleLayerLoadBalance {
		m.loadBalanceSelectTotal.Add(1)
	}
}

func (m *openAIAccountSchedulerMetrics) recordSwitch() {
	if m == nil {
		return
	}
	m.accountSwitchTotal.Add(1)
}

type openAIAccountRuntimeStats struct {
	accounts     sync.Map
	accountCount atomic.Int64
}

type openAIAccountRuntimeStat struct {
	errorRateEWMABits atomic.Uint64
	ttftEWMABits      atomic.Uint64
}

func newOpenAIAccountRuntimeStats() *openAIAccountRuntimeStats {
	return &openAIAccountRuntimeStats{}
}

func (s *openAIAccountRuntimeStats) loadOrCreate(accountID int64) *openAIAccountRuntimeStat {
	if value, ok := s.accounts.Load(accountID); ok {
		stat, _ := value.(*openAIAccountRuntimeStat)
		if stat != nil {
			return stat
		}
	}

	stat := &openAIAccountRuntimeStat{}
	stat.ttftEWMABits.Store(math.Float64bits(math.NaN()))
	actual, loaded := s.accounts.LoadOrStore(accountID, stat)
	if !loaded {
		s.accountCount.Add(1)
		return stat
	}
	existing, _ := actual.(*openAIAccountRuntimeStat)
	if existing != nil {
		return existing
	}
	return stat
}

func updateEWMAAtomic(target *atomic.Uint64, sample float64, alpha float64) {
	for {
		oldBits := target.Load()
		oldValue := math.Float64frombits(oldBits)
		newValue := alpha*sample + (1-alpha)*oldValue
		if target.CompareAndSwap(oldBits, math.Float64bits(newValue)) {
			return
		}
	}
}

func (s *openAIAccountRuntimeStats) report(accountID int64, success bool, firstTokenMs *int) {
	if s == nil || accountID <= 0 {
		return
	}
	const alpha = 0.2
	stat := s.loadOrCreate(accountID)

	errorSample := 1.0
	if success {
		errorSample = 0.0
	}
	updateEWMAAtomic(&stat.errorRateEWMABits, errorSample, alpha)

	if firstTokenMs != nil && *firstTokenMs > 0 {
		ttft := float64(*firstTokenMs)
		ttftBits := math.Float64bits(ttft)
		for {
			oldBits := stat.ttftEWMABits.Load()
			oldValue := math.Float64frombits(oldBits)
			if math.IsNaN(oldValue) {
				if stat.ttftEWMABits.CompareAndSwap(oldBits, ttftBits) {
					break
				}
				continue
			}
			newValue := alpha*ttft + (1-alpha)*oldValue
			if stat.ttftEWMABits.CompareAndSwap(oldBits, math.Float64bits(newValue)) {
				break
			}
		}
	}
}

func (s *openAIAccountRuntimeStats) snapshot(accountID int64) (errorRate float64, ttft float64, hasTTFT bool) {
	if s == nil || accountID <= 0 {
		return 0, 0, false
	}
	value, ok := s.accounts.Load(accountID)
	if !ok {
		return 0, 0, false
	}
	stat, _ := value.(*openAIAccountRuntimeStat)
	if stat == nil {
		return 0, 0, false
	}
	errorRate = clamp01(math.Float64frombits(stat.errorRateEWMABits.Load()))
	ttftValue := math.Float64frombits(stat.ttftEWMABits.Load())
	if math.IsNaN(ttftValue) {
		return errorRate, 0, false
	}
	return errorRate, ttftValue, true
}

func (s *openAIAccountRuntimeStats) size() int {
	if s == nil {
		return 0
	}
	return int(s.accountCount.Load())
}

type defaultOpenAIAccountScheduler struct {
	service *OpenAIGatewayService
	metrics openAIAccountSchedulerMetrics
	stats   *openAIAccountRuntimeStats
}

type openAIStickyEscapeConfig struct {
	enabled   bool
	ttftMs    float64
	errorRate float64
}

func newDefaultOpenAIAccountScheduler(service *OpenAIGatewayService, stats *openAIAccountRuntimeStats) OpenAIAccountScheduler {
	if stats == nil {
		stats = newOpenAIAccountRuntimeStats()
	}
	return &defaultOpenAIAccountScheduler{
		service: service,
		stats:   stats,
	}
}

func (s *defaultOpenAIAccountScheduler) Select(
	ctx context.Context,
	req OpenAIAccountScheduleRequest,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	decision := OpenAIAccountScheduleDecision{}
	start := time.Now()
	defer func() {
		decision.LatencyMs = time.Since(start).Milliseconds()
		s.metrics.recordSelect(decision)
	}()

	req.Platform = normalizeOpenAICompatiblePlatform(req.Platform)
	previousResponseID := strings.TrimSpace(req.PreviousResponseID)
	if previousResponseID != "" && req.Platform == PlatformOpenAI && (!req.StickyWeighted || !req.PreviousResponseCanMove) {
		selection, err := s.service.selectAccountByPreviousResponseIDForCapability(
			ctx,
			req.GroupID,
			previousResponseID,
			req.RequestedModel,
			req.ExcludedIDs,
			req.RequiredCapability,
			req.RequiredImageCapability,
			req.RequireCompact,
		)
		if err != nil {
			return nil, decision, err
		}
		if selection != nil && selection.Account != nil {
			if !s.isAccountTransportCompatible(selection.Account, req.RequiredTransport) || !s.isAccountRequestCompatible(selection.Account, req) {
				if selection.ReleaseFunc != nil {
					selection.ReleaseFunc()
				}
				selection = nil
			}
		}
		if selection != nil && selection.Account != nil {
			decision.Layer = openAIAccountScheduleLayerPreviousResponse
			decision.StickyPreviousHit = true
			decision.SelectedAccountID = selection.Account.ID
			decision.SelectedAccountType = selection.Account.Type
			if req.SessionHash != "" {
				_ = s.service.BindStickySession(ctx, req.GroupID, req.SessionHash, selection.Account.ID)
			}
			return selection, decision, nil
		}
	}

	if !req.StickyWeighted {
		selection, escapedSticky, err := s.selectBySessionHash(ctx, req)
		if err != nil {
			return nil, decision, err
		}
		if selection != nil && selection.Account != nil {
			decision.Layer = openAIAccountScheduleLayerSessionSticky
			decision.StickySessionHit = true
			decision.SelectedAccountID = selection.Account.ID
			decision.SelectedAccountType = selection.Account.Type
			return selection, decision, nil
		}
		if escapedSticky {
			req.PreserveStickyBinding = true
		}
	}

	selection, candidateCount, topK, loadSkew, err := s.selectByLoadBalance(ctx, req)
	decision.Layer = openAIAccountScheduleLayerLoadBalance
	decision.CandidateCount = candidateCount
	decision.TopK = topK
	decision.LoadSkew = loadSkew
	if err != nil {
		return nil, decision, err
	}
	if selection != nil && selection.Account != nil {
		decision.SelectedAccountID = selection.Account.ID
		decision.SelectedAccountType = selection.Account.Type
		if req.StickyWeighted {
			decision.StickyPreviousHit = req.StickyPreviousAccountID > 0 && selection.Account.ID == req.StickyPreviousAccountID
			decision.StickySessionHit = req.StickyAccountID > 0 && selection.Account.ID == req.StickyAccountID
		}
	}
	return selection, decision, nil
}

func (s *defaultOpenAIAccountScheduler) selectBySessionHash(
	ctx context.Context,
	req OpenAIAccountScheduleRequest,
) (*AccountSelectionResult, bool, error) {
	sessionHash := strings.TrimSpace(req.SessionHash)
	if sessionHash == "" || s == nil || s.service == nil || s.service.cache == nil {
		return nil, false, nil
	}

	accountID := req.StickyAccountID
	if accountID <= 0 {
		var err error
		accountID, err = s.service.getStickySessionAccountID(ctx, req.GroupID, sessionHash)
		if err != nil || accountID <= 0 {
			return nil, false, nil
		}
	}
	if accountID <= 0 {
		return nil, false, nil
	}
	if req.ExcludedIDs != nil {
		if _, excluded := req.ExcludedIDs[accountID]; excluded {
			return nil, false, nil
		}
	}

	account, err := s.service.getSchedulableAccount(ctx, accountID)
	if err != nil || account == nil {
		_ = s.service.deleteStickySessionAccountID(ctx, req.GroupID, sessionHash)
		return nil, false, nil
	}
	if shouldClearStickySession(account, req.RequestedModel) || !account.IsOpenAICompatible() || account.Platform != normalizeOpenAICompatiblePlatform(req.Platform) || !account.IsSchedulable() || s.service.isOpenAIAccountRuntimeBlocked(account) {
		_ = s.service.deleteStickySessionAccountID(ctx, req.GroupID, sessionHash)
		return nil, false, nil
	}
	if !s.isAccountRequestCompatible(account, req) {
		return nil, false, nil
	}
	if !s.isAccountTransportCompatible(account, req.RequiredTransport) {
		_ = s.service.deleteStickySessionAccountID(ctx, req.GroupID, sessionHash)
		return nil, false, nil
	}
	account = s.service.recheckSelectedOpenAIAccountFromDBForSchedule(ctx, account, openAIAccountRequestEligibility{
		Platform:               req.Platform,
		RequestedModel:         req.RequestedModel,
		RequireCompact:         req.RequireCompact,
		RequireClaudeGPTBridge: req.RequireClaudeGPTBridge,
	})
	if account == nil || !openAIStickyAccountMatchesGroup(account, req.GroupID) || !s.isAccountTransportCompatible(account, req.RequiredTransport) {
		_ = s.service.deleteStickySessionAccountID(ctx, req.GroupID, sessionHash)
		return nil, false, nil
	}
	escapeCfg := s.service.openAIStickyEscapeConfig()
	if reason, errorRate, ttft, shouldEscape := s.shouldEscapeStickyAccount(accountID, escapeCfg); shouldEscape {
		slog.Info("sticky_escape_triggered", "account_id", accountID, "reason", reason, "error_rate", errorRate, "ttft", ttft)
		return nil, true, nil
	}

	result, acquireErr := s.service.tryAcquireAccountSlot(ctx, accountID, account.Concurrency)
	if acquireErr == nil && result != nil && result.Acquired {
		_ = s.service.refreshStickySessionTTL(ctx, req.GroupID, sessionHash, s.service.openAIWSSessionStickyTTL())
		return &AccountSelectionResult{
			Account:     account,
			Acquired:    true,
			ReleaseFunc: result.ReleaseFunc,
		}, false, nil
	}

	cfg := s.service.schedulingConfig()
	// WaitPlan.MaxConcurrency 使用 Concurrency（非 EffectiveLoadFactor），因为 WaitPlan 控制的是 Redis 实际并发槽位等待。
	if s.service.concurrencyService != nil {
		if escapeCfg.enabled && acquireErr == nil && result != nil && !result.Acquired {
			errorRate, ttft, _ := s.stats.snapshot(accountID)
			slog.Info("sticky_escape_triggered", "account_id", accountID, "reason", "concurrency_full", "error_rate", errorRate, "ttft", ttft)
			return nil, true, nil
		}
		return &AccountSelectionResult{
			Account: account,
			WaitPlan: &AccountWaitPlan{
				AccountID:      accountID,
				MaxConcurrency: account.Concurrency,
				Timeout:        cfg.StickySessionWaitTimeout,
				MaxWaiting:     cfg.StickySessionMaxWaiting,
			},
		}, false, nil
	}
	return nil, false, nil
}

func (s *defaultOpenAIAccountScheduler) shouldEscapeStickyAccount(accountID int64, cfg openAIStickyEscapeConfig) (string, float64, float64, bool) {
	if !cfg.enabled || s == nil || s.stats == nil || accountID <= 0 {
		return "", 0, 0, false
	}
	errorRate, ttft, hasTTFT := s.stats.snapshot(accountID)
	if hasTTFT && ttft > cfg.ttftMs {
		return "ttft", errorRate, ttft, true
	}
	if errorRate > cfg.errorRate {
		return "error_rate", errorRate, ttft, true
	}
	return "", errorRate, ttft, false
}

func openAIStickyAccountMatchesGroup(account *Account, groupID *int64) bool {
	if account == nil {
		return false
	}
	if groupID == nil {
		return len(account.AccountGroups) == 0 && len(account.GroupIDs) == 0
	}
	for _, accountGroupID := range account.GroupIDs {
		if accountGroupID == *groupID {
			return true
		}
	}
	for _, accountGroup := range account.AccountGroups {
		if accountGroup.GroupID == *groupID {
			return true
		}
	}
	return false
}

type openAIAccountCandidateScore struct {
	account   *Account
	loadInfo  *AccountLoadInfo
	score     float64
	errorRate float64
	ttft      float64
	hasTTFT   bool
}

type openAIAccountCandidateHeap []openAIAccountCandidateScore

func partitionOpenAIChatGPTSubscriptionAccounts(candidates []openAIAccountCandidateScore) ([]openAIAccountCandidateScore, []openAIAccountCandidateScore) {
	subscription := make([]openAIAccountCandidateScore, 0, len(candidates))
	regular := make([]openAIAccountCandidateScore, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.account != nil && candidate.account.IsOpenAIChatGPTSubscription() {
			subscription = append(subscription, candidate)
		} else {
			regular = append(regular, candidate)
		}
	}
	return subscription, regular
}

func (h openAIAccountCandidateHeap) Len() int {
	return len(h)
}

func (h openAIAccountCandidateHeap) Less(i, j int) bool {
	// 最小堆根节点保存“最差”候选，便于 O(log k) 维护 topK。
	return isOpenAIAccountCandidateBetter(h[j], h[i])
}

func (h openAIAccountCandidateHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *openAIAccountCandidateHeap) Push(x any) {
	candidate, ok := x.(openAIAccountCandidateScore)
	if !ok {
		panic("openAIAccountCandidateHeap: invalid element type")
	}
	*h = append(*h, candidate)
}

func (h *openAIAccountCandidateHeap) Pop() any {
	old := *h
	n := len(old)
	last := old[n-1]
	*h = old[:n-1]
	return last
}

func isOpenAIAccountCandidateBetter(left openAIAccountCandidateScore, right openAIAccountCandidateScore) bool {
	if left.score != right.score {
		return left.score > right.score
	}
	if left.account.Priority != right.account.Priority {
		return left.account.Priority < right.account.Priority
	}
	if left.loadInfo.LoadRate != right.loadInfo.LoadRate {
		return left.loadInfo.LoadRate < right.loadInfo.LoadRate
	}
	if left.loadInfo.WaitingCount != right.loadInfo.WaitingCount {
		return left.loadInfo.WaitingCount < right.loadInfo.WaitingCount
	}
	return left.account.ID < right.account.ID
}

func selectTopKOpenAICandidates(candidates []openAIAccountCandidateScore, topK int) []openAIAccountCandidateScore {
	if len(candidates) == 0 {
		return nil
	}
	if topK <= 0 {
		topK = 1
	}
	if topK >= len(candidates) {
		ranked := append([]openAIAccountCandidateScore(nil), candidates...)
		sort.Slice(ranked, func(i, j int) bool {
			return isOpenAIAccountCandidateBetter(ranked[i], ranked[j])
		})
		return ranked
	}

	best := make(openAIAccountCandidateHeap, 0, topK)
	for _, candidate := range candidates {
		if len(best) < topK {
			heap.Push(&best, candidate)
			continue
		}
		if isOpenAIAccountCandidateBetter(candidate, best[0]) {
			best[0] = candidate
			heap.Fix(&best, 0)
		}
	}

	ranked := make([]openAIAccountCandidateScore, len(best))
	copy(ranked, best)
	sort.Slice(ranked, func(i, j int) bool {
		return isOpenAIAccountCandidateBetter(ranked[i], ranked[j])
	})
	return ranked
}

type openAISelectionRNG struct {
	state uint64
}

func newOpenAISelectionRNG(seed uint64) openAISelectionRNG {
	if seed == 0 {
		seed = 0x9e3779b97f4a7c15
	}
	return openAISelectionRNG{state: seed}
}

func (r *openAISelectionRNG) nextUint64() uint64 {
	// xorshift64*
	x := r.state
	x ^= x >> 12
	x ^= x << 25
	x ^= x >> 27
	r.state = x
	return x * 2685821657736338717
}

func (r *openAISelectionRNG) nextFloat64() float64 {
	// [0,1)
	return float64(r.nextUint64()>>11) / (1 << 53)
}

func deriveOpenAISelectionSeed(req OpenAIAccountScheduleRequest) uint64 {
	hasher := fnv.New64a()
	writeValue := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		_, _ = hasher.Write([]byte(trimmed))
		_, _ = hasher.Write([]byte{0})
	}

	writeValue(req.SessionHash)
	writeValue(req.PreviousResponseID)
	writeValue(req.RequestedModel)
	if req.GroupID != nil {
		_, _ = hasher.Write([]byte(strconv.FormatInt(*req.GroupID, 10)))
	}

	seed := hasher.Sum64()
	// 对“无会话锚点”的纯负载均衡请求引入时间熵，避免固定命中同一账号。
	if strings.TrimSpace(req.SessionHash) == "" && strings.TrimSpace(req.PreviousResponseID) == "" {
		seed ^= uint64(time.Now().UnixNano())
	}
	if seed == 0 {
		seed = uint64(time.Now().UnixNano()) ^ 0x9e3779b97f4a7c15
	}
	return seed
}

func buildOpenAIWeightedSelectionOrder(
	candidates []openAIAccountCandidateScore,
	req OpenAIAccountScheduleRequest,
) []openAIAccountCandidateScore {
	if len(candidates) <= 1 {
		return append([]openAIAccountCandidateScore(nil), candidates...)
	}

	pool := append([]openAIAccountCandidateScore(nil), candidates...)
	weights := make([]float64, len(pool))
	minScore := pool[0].score
	for i := 1; i < len(pool); i++ {
		if pool[i].score < minScore {
			minScore = pool[i].score
		}
	}
	for i := range pool {
		// 将 top-K 分值平移到正区间，避免“单一最高分账号”长期垄断。
		weight := (pool[i].score - minScore) + 1.0
		if math.IsNaN(weight) || math.IsInf(weight, 0) || weight <= 0 {
			weight = 1.0
		}
		weights[i] = weight
	}

	order := make([]openAIAccountCandidateScore, 0, len(pool))
	rng := newOpenAISelectionRNG(deriveOpenAISelectionSeed(req))
	for len(pool) > 0 {
		total := 0.0
		for _, w := range weights {
			total += w
		}

		selectedIdx := 0
		if total > 0 {
			r := rng.nextFloat64() * total
			acc := 0.0
			for i, w := range weights {
				acc += w
				if r <= acc {
					selectedIdx = i
					break
				}
			}
		} else {
			selectedIdx = int(rng.nextUint64() % uint64(len(pool)))
		}

		order = append(order, pool[selectedIdx])
		pool = append(pool[:selectedIdx], pool[selectedIdx+1:]...)
		weights = append(weights[:selectedIdx], weights[selectedIdx+1:]...)
	}
	return order
}

func (s *defaultOpenAIAccountScheduler) selectByLoadBalance(
	ctx context.Context,
	req OpenAIAccountScheduleRequest,
) (*AccountSelectionResult, int, int, float64, error) {
	accounts, err := s.service.listSchedulableAccounts(ctx, req.GroupID, req.Platform)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	if len(accounts) == 0 {
		return nil, 0, 0, 0, noAvailableOpenAISelectionError(req.RequestedModel, false)
	}

	// require_privacy_set: 获取分组信息
	var schedGroup *Group
	if req.GroupID != nil && s.service.schedulerSnapshot != nil {
		schedGroup, _ = s.service.schedulerSnapshot.GetGroupByID(ctx, *req.GroupID)
	}

	filtered := make([]*Account, 0, len(accounts))
	loadReq := make([]AccountWithConcurrency, 0, len(accounts))
	for i := range accounts {
		account := &accounts[i]
		if req.ExcludedIDs != nil {
			if _, excluded := req.ExcludedIDs[account.ID]; excluded {
				continue
			}
		}
		if !account.IsSchedulable() || !account.IsOpenAICompatible() || account.Platform != normalizeOpenAICompatiblePlatform(req.Platform) || s.service.isOpenAIAccountRuntimeBlocked(account) {
			continue
		}
		// require_privacy_set: 跳过 privacy 未设置的账号并标记异常
		if schedGroup != nil && schedGroup.RequirePrivacySet && !account.IsPrivacySet() {
			_ = s.service.accountRepo.SetError(ctx, account.ID,
				fmt.Sprintf("Privacy not set, required by group [%s]", schedGroup.Name))
			continue
		}
		candidate := account
		if !s.isAccountCandidatePoolCompatible(candidate, req) {
			candidate = s.service.refreshStaleOpenAIScheduleCandidate(ctx, account, openAIAccountRequestEligibility{
				Platform:               req.Platform,
				RequestedModel:         req.RequestedModel,
				RequireClaudeGPTBridge: req.RequireClaudeGPTBridge,
			})
			if candidate == nil {
				continue
			}
		}
		if !s.isAccountTransportCompatible(candidate, req.RequiredTransport) {
			continue
		}
		filtered = append(filtered, candidate)
		loadReq = append(loadReq, AccountWithConcurrency{
			ID:             candidate.ID,
			MaxConcurrency: candidate.EffectiveLoadFactor(),
		})
	}
	if len(filtered) == 0 {
		return nil, 0, 0, 0, noAvailableOpenAISelectionError(req.RequestedModel, false)
	}

	loadMap := map[int64]*AccountLoadInfo{}
	if s.service.concurrencyService != nil {
		if batchLoad, loadErr := s.service.concurrencyService.GetAccountsLoadBatch(ctx, loadReq); loadErr == nil {
			loadMap = batchLoad
		}
	}

	allCandidates := make([]openAIAccountCandidateScore, 0, len(filtered))
	for _, account := range filtered {
		loadInfo := loadMap[account.ID]
		if loadInfo == nil {
			loadInfo = &AccountLoadInfo{AccountID: account.ID}
		}
		errorRate, ttft, hasTTFT := s.stats.snapshot(account.ID)
		allCandidates = append(allCandidates, openAIAccountCandidateScore{
			account:   account,
			loadInfo:  loadInfo,
			errorRate: errorRate,
			ttft:      ttft,
			hasTTFT:   hasTTFT,
		})
	}

	// Compact 模式下把明确不支持 compact 的账号拆出，仅在 schedulerSnapshot 启用
	// 时作为最后兜底（snapshot 可能已陈旧）。
	candidates := allCandidates
	staleSnapshotCompactRetry := make([]openAIAccountCandidateScore, 0, len(allCandidates))
	if req.RequireCompact {
		candidates = make([]openAIAccountCandidateScore, 0, len(allCandidates))
		for _, candidate := range allCandidates {
			if openAICompactSupportTier(candidate.account) == 0 {
				staleSnapshotCompactRetry = append(staleSnapshotCompactRetry, candidate)
				continue
			}
			candidates = append(candidates, candidate)
		}
		if len(candidates) == 0 && len(staleSnapshotCompactRetry) == 0 {
			return nil, 0, 0, 0, ErrNoAvailableCompactAccounts
		}
	}

	candidateCount := len(candidates)
	loadSkew := 0.0
	if len(candidates) > 0 {
		minPriority, maxPriority := candidates[0].account.Priority, candidates[0].account.Priority
		maxWaiting := 1
		loadRateSum := 0.0
		loadRateSumSquares := 0.0
		minTTFT, maxTTFT := 0.0, 0.0
		hasTTFTSample := false
		for _, candidate := range candidates {
			if candidate.account.Priority < minPriority {
				minPriority = candidate.account.Priority
			}
			if candidate.account.Priority > maxPriority {
				maxPriority = candidate.account.Priority
			}
			if candidate.loadInfo.WaitingCount > maxWaiting {
				maxWaiting = candidate.loadInfo.WaitingCount
			}
			if candidate.hasTTFT && candidate.ttft > 0 {
				if !hasTTFTSample {
					minTTFT, maxTTFT = candidate.ttft, candidate.ttft
					hasTTFTSample = true
				} else {
					if candidate.ttft < minTTFT {
						minTTFT = candidate.ttft
					}
					if candidate.ttft > maxTTFT {
						maxTTFT = candidate.ttft
					}
				}
			}
			loadRate := float64(candidate.loadInfo.LoadRate)
			loadRateSum += loadRate
			loadRateSumSquares += loadRate * loadRate
		}
		loadSkew = calcLoadSkewByMoments(loadRateSum, loadRateSumSquares, len(candidates))

		weights := s.service.openAIWSSchedulerWeightsForRequest(ctx)
		minResetRemaining, maxResetRemaining := 0.0, 0.0
		hasResetSample := false
		if weights.Reset > 0 {
			now := time.Now()
			for _, candidate := range candidates {
				if end := candidate.account.SessionWindowEnd; end != nil && now.Before(*end) {
					remaining := end.Sub(now).Seconds()
					if !hasResetSample {
						minResetRemaining, maxResetRemaining, hasResetSample = remaining, remaining, true
					} else {
						if remaining < minResetRemaining {
							minResetRemaining = remaining
						}
						if remaining > maxResetRemaining {
							maxResetRemaining = remaining
						}
					}
				}
			}
		}
		now := time.Now()
		for i := range candidates {
			item := &candidates[i]
			priorityFactor := 1.0
			if maxPriority > minPriority {
				priorityFactor = 1 - float64(item.account.Priority-minPriority)/float64(maxPriority-minPriority)
			}
			loadFactor := 1 - clamp01(float64(item.loadInfo.LoadRate)/100.0)
			queueFactor := 1 - clamp01(float64(item.loadInfo.WaitingCount)/float64(maxWaiting))
			errorFactor := 1 - clamp01(item.errorRate)
			ttftFactor := 0.5
			if item.hasTTFT && hasTTFTSample && maxTTFT > minTTFT {
				ttftFactor = 1 - clamp01((item.ttft-minTTFT)/(maxTTFT-minTTFT))
			}
			quotaHeadroomFactor := 0.0
			if weights.QuotaHeadroom > 0 {
				quotaHeadroomFactor = openAIQuotaHeadroomFactor(item.account, now)
			}
			resetFactor := 0.0
			if weights.Reset > 0 && hasResetSample {
				if end := item.account.SessionWindowEnd; end != nil && now.Before(*end) {
					if maxResetRemaining > minResetRemaining {
						resetFactor = 1 - clamp01((end.Sub(now).Seconds()-minResetRemaining)/(maxResetRemaining-minResetRemaining))
					} else {
						resetFactor = 1
					}
				}
			}

			item.score = weights.Priority*priorityFactor +
				weights.Load*loadFactor +
				weights.Queue*queueFactor +
				weights.ErrorRate*errorFactor +
				weights.TTFT*ttftFactor + weights.Reset*resetFactor +
				weights.QuotaHeadroom*quotaHeadroomFactor
			if req.StickyWeighted {
				if req.StickyPreviousAccountID > 0 && item.account.ID == req.StickyPreviousAccountID {
					item.score += weights.Previous
				}
				if req.StickyAccountID > 0 && item.account.ID == req.StickyAccountID {
					item.score += weights.SessionSticky
				}
			}
		}
	}

	topK := 0
	if len(candidates) > 0 {
		topK = s.service.openAIWSLBTopKForRequest(ctx)
		if topK > len(candidates) {
			topK = len(candidates)
		}
		if topK <= 0 {
			topK = 1
		}
	}

	buildSelectionOrder := func(pool []openAIAccountCandidateScore) []openAIAccountCandidateScore {
		if len(pool) == 0 || topK <= 0 {
			return nil
		}
		groupTopK := topK
		if groupTopK > len(pool) {
			groupTopK = len(pool)
		}
		ranked := selectTopKOpenAICandidates(pool, groupTopK)
		if req.StickyWeighted {
			for _, stickyID := range []int64{req.StickyPreviousAccountID, req.StickyAccountID} {
				if stickyID <= 0 {
					continue
				}
				for i, candidate := range ranked {
					if candidate.account != nil && candidate.account.ID == stickyID {
						ordered := append([]openAIAccountCandidateScore{candidate}, ranked[:i]...)
						return append(ordered, ranked[i+1:]...)
					}
				}
			}
		}
		return buildOpenAIWeightedSelectionOrder(ranked, req)
	}
	sortCompactRetryCandidates := func(pool []openAIAccountCandidateScore) []openAIAccountCandidateScore {
		if len(pool) == 0 {
			return nil
		}
		ordered := append([]openAIAccountCandidateScore(nil), pool...)
		sort.SliceStable(ordered, func(i, j int) bool {
			a, b := ordered[i], ordered[j]
			if a.account.Priority != b.account.Priority {
				return a.account.Priority < b.account.Priority
			}
			if a.loadInfo.LoadRate != b.loadInfo.LoadRate {
				return a.loadInfo.LoadRate < b.loadInfo.LoadRate
			}
			if a.loadInfo.WaitingCount != b.loadInfo.WaitingCount {
				return a.loadInfo.WaitingCount < b.loadInfo.WaitingCount
			}
			switch {
			case a.account.LastUsedAt == nil && b.account.LastUsedAt != nil:
				return true
			case a.account.LastUsedAt != nil && b.account.LastUsedAt == nil:
				return false
			case a.account.LastUsedAt == nil && b.account.LastUsedAt == nil:
				return false
			default:
				return a.account.LastUsedAt.Before(*b.account.LastUsedAt)
			}
		})
		return ordered
	}

	buildPoolOrder := func(pool []openAIAccountCandidateScore) []openAIAccountCandidateScore {
		if !req.RequireCompact {
			return buildSelectionOrder(pool)
		}
		supported := make([]openAIAccountCandidateScore, 0, len(candidates))
		unknown := make([]openAIAccountCandidateScore, 0, len(candidates))
		for _, candidate := range pool {
			switch openAICompactSupportTier(candidate.account) {
			case 2:
				supported = append(supported, candidate)
			case 1:
				unknown = append(unknown, candidate)
			}
		}
		if len(supported) == 0 && len(unknown) == 0 && s.service.schedulerSnapshot == nil {
			return nil
		}
		ordered := append(buildSelectionOrder(supported), buildSelectionOrder(unknown)...)
		return ordered
	}

	selectionOrder := make([]openAIAccountCandidateScore, 0, len(allCandidates))
	waitOrder := make([]openAIAccountCandidateScore, 0, len(allCandidates))
	if req.SubscriptionPriority {
		subscription, regular := partitionOpenAIChatGPTSubscriptionAccounts(candidates)
		subscriptionOrder := buildPoolOrder(subscription)
		regularOrder := buildPoolOrder(regular)
		selectionOrder = append(selectionOrder, subscriptionOrder...)
		selectionOrder = append(selectionOrder, regularOrder...)
		waitOrder = append(waitOrder, regularOrder...)
		waitOrder = append(waitOrder, subscriptionOrder...)
	} else {
		selectionOrder = buildPoolOrder(candidates)
		waitOrder = append(waitOrder, selectionOrder...)
	}
	if req.RequireCompact && len(staleSnapshotCompactRetry) > 0 && s.service.schedulerSnapshot != nil {
		fallbackOrder := sortCompactRetryCandidates(staleSnapshotCompactRetry)
		selectionOrder = append(selectionOrder, fallbackOrder...)
		waitOrder = append(waitOrder, fallbackOrder...)
	}
	if len(selectionOrder) == 0 {
		return nil, candidateCount, topK, loadSkew, noAvailableOpenAISelectionError(req.RequestedModel, req.RequireCompact && len(allCandidates) > 0)
	}

	compactBlocked := false
	for i := 0; i < len(selectionOrder); i++ {
		candidate := selectionOrder[i]
		eligibility := openAIAccountRequestEligibility{
			Platform:               req.Platform,
			RequestedModel:         req.RequestedModel,
			RequireCompact:         req.RequireCompact,
			RequireClaudeGPTBridge: req.RequireClaudeGPTBridge,
		}
		fresh := s.service.resolveFreshSchedulableOpenAIAccountForSchedule(ctx, candidate.account, eligibility)
		if fresh == nil || !s.isAccountTransportCompatible(fresh, req.RequiredTransport) || !s.isAccountRequestCompatible(fresh, req) {
			continue
		}
		fresh = s.service.recheckSelectedOpenAIAccountFromDBForSchedule(ctx, fresh, eligibility)
		if fresh == nil || !s.isAccountTransportCompatible(fresh, req.RequiredTransport) || !s.isAccountRequestCompatible(fresh, req) {
			continue
		}
		if req.RequireCompact && openAICompactSupportTier(fresh) == 0 {
			compactBlocked = true
			continue
		}
		result, acquireErr := s.service.tryAcquireAccountSlot(ctx, fresh.ID, fresh.Concurrency)
		if acquireErr != nil {
			return nil, candidateCount, topK, loadSkew, acquireErr
		}
		if result != nil && result.Acquired {
			if req.SessionHash != "" && !req.PreserveStickyBinding {
				_ = s.service.BindStickySession(ctx, req.GroupID, req.SessionHash, fresh.ID)
			}
			return &AccountSelectionResult{
				Account:     fresh,
				Acquired:    true,
				ReleaseFunc: result.ReleaseFunc,
			}, candidateCount, topK, loadSkew, nil
		}
	}

	if req.StickyWeighted {
		for _, stickyID := range []int64{req.StickyPreviousAccountID, req.StickyAccountID} {
			if stickyID <= 0 {
				continue
			}
			var stickyCandidate *openAIAccountCandidateScore
			for i := range candidates {
				if candidates[i].account != nil && candidates[i].account.ID == stickyID {
					stickyCandidate = &candidates[i]
					break
				}
			}
			if stickyCandidate == nil {
				if stickyID == req.StickyAccountID && strings.TrimSpace(req.SessionHash) != "" {
					_ = s.service.deleteStickySessionAccountID(ctx, req.GroupID, req.SessionHash)
				}
				continue
			}
			fresh := s.service.resolveFreshSchedulableOpenAIAccountForSchedule(ctx, stickyCandidate.account, openAIAccountRequestEligibility{
				Platform:               req.Platform,
				RequestedModel:         req.RequestedModel,
				RequireCompact:         req.RequireCompact,
				RequireClaudeGPTBridge: req.RequireClaudeGPTBridge,
			})
			if fresh == nil || !s.isAccountTransportCompatible(fresh, req.RequiredTransport) || !s.isAccountRequestCompatible(fresh, req) {
				continue
			}
			fresh = s.service.recheckSelectedOpenAIAccountFromDBForSchedule(ctx, fresh, openAIAccountRequestEligibility{
				Platform:               req.Platform,
				RequestedModel:         req.RequestedModel,
				RequireCompact:         req.RequireCompact,
				RequireClaudeGPTBridge: req.RequireClaudeGPTBridge,
			})
			if fresh == nil || !s.isAccountTransportCompatible(fresh, req.RequiredTransport) || !s.isAccountRequestCompatible(fresh, req) {
				continue
			}
			result, acquireErr := s.service.tryAcquireAccountSlot(ctx, fresh.ID, fresh.Concurrency)
			if acquireErr != nil {
				return nil, candidateCount, topK, loadSkew, acquireErr
			}
			if result != nil && result.Acquired {
				return &AccountSelectionResult{Account: fresh, Acquired: true, ReleaseFunc: result.ReleaseFunc}, candidateCount, topK, loadSkew, nil
			}
			if s.service.concurrencyService != nil {
				cfg := s.service.schedulingConfig()
				return &AccountSelectionResult{Account: fresh, WaitPlan: &AccountWaitPlan{
					AccountID: fresh.ID, MaxConcurrency: fresh.Concurrency,
					Timeout: cfg.StickySessionWaitTimeout, MaxWaiting: cfg.StickySessionMaxWaiting,
				}}, candidateCount, topK, loadSkew, nil
			}
		}
	}

	cfg := s.service.schedulingConfig()
	// WaitPlan.MaxConcurrency 使用 Concurrency（非 EffectiveLoadFactor），因为 WaitPlan 控制的是 Redis 实际并发槽位等待。
	for _, candidate := range waitOrder {
		eligibility := openAIAccountRequestEligibility{
			Platform:               req.Platform,
			RequestedModel:         req.RequestedModel,
			RequireCompact:         req.RequireCompact,
			RequireClaudeGPTBridge: req.RequireClaudeGPTBridge,
		}
		fresh := s.service.resolveFreshSchedulableOpenAIAccountForSchedule(ctx, candidate.account, eligibility)
		if fresh == nil || !s.isAccountTransportCompatible(fresh, req.RequiredTransport) || !s.isAccountRequestCompatible(fresh, req) {
			continue
		}
		fresh = s.service.recheckSelectedOpenAIAccountFromDBForSchedule(ctx, fresh, eligibility)
		if fresh == nil || !s.isAccountTransportCompatible(fresh, req.RequiredTransport) || !s.isAccountRequestCompatible(fresh, req) {
			continue
		}
		if req.RequireCompact && openAICompactSupportTier(fresh) == 0 {
			compactBlocked = true
			continue
		}
		return &AccountSelectionResult{
			Account: fresh,
			WaitPlan: &AccountWaitPlan{
				AccountID:      fresh.ID,
				MaxConcurrency: fresh.Concurrency,
				Timeout:        cfg.FallbackWaitTimeout,
				MaxWaiting:     cfg.FallbackMaxWaiting,
			},
		}, candidateCount, topK, loadSkew, nil
	}

	return nil, candidateCount, topK, loadSkew, noAvailableOpenAISelectionError(req.RequestedModel, compactBlocked)
}

func (s *defaultOpenAIAccountScheduler) isAccountTransportCompatible(account *Account, requiredTransport OpenAIUpstreamTransport) bool {
	if requiredTransport == OpenAIUpstreamTransportAny || requiredTransport == OpenAIUpstreamTransportHTTPSSE {
		return true
	}
	if s == nil || s.service == nil {
		return false
	}
	return s.service.isOpenAIAccountTransportCompatible(account, requiredTransport)
}

func (s *defaultOpenAIAccountScheduler) isAccountRequestCompatible(args ...any) bool {
	ctx := context.Background()
	var account *Account
	var req OpenAIAccountScheduleRequest
	switch len(args) {
	case 2:
		account, _ = args[0].(*Account)
		req, _ = args[1].(OpenAIAccountScheduleRequest)
	case 3:
		if candidate, ok := args[0].(context.Context); ok {
			ctx = candidate
		}
		account, _ = args[1].(*Account)
		req, _ = args[2].(OpenAIAccountScheduleRequest)
	default:
		return false
	}
	if account == nil {
		return false
	}
	if paused, _ := shouldAutoPauseOpenAIAccountByQuota(ctx, account); paused {
		return false
	}
	if account.IsShadow() {
		if s == nil || s.service == nil {
			return false
		}
		if !parentHealthyForShadow(account, func(parentID int64) *Account {
			if s.service.schedulerSnapshot != nil {
				if parent, err := s.service.schedulerSnapshot.GetAccount(ctx, parentID); err == nil && parent != nil {
					return parent
				}
			}
			if s.service.accountRepo != nil {
				parent, _ := s.service.accountRepo.GetByID(ctx, parentID)
				return parent
			}
			return nil
		}) {
			return false
		}
	}
	if !isOpenAIAccountEligibleForScheduleRequest(account, openAIAccountRequestEligibility{
		Platform:               req.Platform,
		RequestedModel:         req.RequestedModel,
		RequireCompact:         req.RequireCompact,
		RequireClaudeGPTBridge: req.RequireClaudeGPTBridge,
	}) {
		return false
	}
	return accountSupportsOpenAICapabilities(account, req.RequiredCapability, req.RequiredImageCapability)
}

func (s *defaultOpenAIAccountScheduler) isAccountCandidatePoolCompatible(account *Account, req OpenAIAccountScheduleRequest) bool {
	if account == nil {
		return false
	}
	eligibility := openAIAccountRequestEligibility{
		Platform:               req.Platform,
		RequestedModel:         req.RequestedModel,
		RequireCompact:         req.RequireCompact,
		RequireClaudeGPTBridge: req.RequireClaudeGPTBridge,
	}
	eligibility = openAIAccountCandidatePoolEligibility(eligibility)
	if !isOpenAIAccountEligibleForScheduleRequest(account, eligibility) {
		return false
	}
	return accountSupportsOpenAICapabilities(account, req.RequiredCapability, req.RequiredImageCapability)
}

func (s *defaultOpenAIAccountScheduler) ReportResult(accountID int64, success bool, firstTokenMs *int) {
	if s == nil || s.stats == nil {
		return
	}
	s.stats.report(accountID, success, firstTokenMs)
}

func (s *defaultOpenAIAccountScheduler) ReportSwitch() {
	if s == nil {
		return
	}
	s.metrics.recordSwitch()
}

func (s *defaultOpenAIAccountScheduler) SnapshotMetrics() OpenAIAccountSchedulerMetricsSnapshot {
	if s == nil {
		return OpenAIAccountSchedulerMetricsSnapshot{}
	}

	selectTotal := s.metrics.selectTotal.Load()
	prevHit := s.metrics.stickyPreviousHitTotal.Load()
	sessionHit := s.metrics.stickySessionHitTotal.Load()
	switchTotal := s.metrics.accountSwitchTotal.Load()
	latencyTotal := s.metrics.latencyMsTotal.Load()
	loadSkewTotal := s.metrics.loadSkewMilliTotal.Load()

	snapshot := OpenAIAccountSchedulerMetricsSnapshot{
		SelectTotal:              selectTotal,
		StickyPreviousHitTotal:   prevHit,
		StickySessionHitTotal:    sessionHit,
		LoadBalanceSelectTotal:   s.metrics.loadBalanceSelectTotal.Load(),
		AccountSwitchTotal:       switchTotal,
		SchedulerLatencyMsTotal:  latencyTotal,
		RuntimeStatsAccountCount: s.stats.size(),
	}
	if selectTotal > 0 {
		snapshot.SchedulerLatencyMsAvg = float64(latencyTotal) / float64(selectTotal)
		snapshot.StickyHitRatio = float64(prevHit+sessionHit) / float64(selectTotal)
		snapshot.AccountSwitchRate = float64(switchTotal) / float64(selectTotal)
		snapshot.LoadSkewAvg = float64(loadSkewTotal) / 1000 / float64(selectTotal)
	}
	return snapshot
}

func (s *OpenAIGatewayService) openAIAdvancedSchedulerSettingRepo() SettingRepository {
	if s == nil || s.rateLimitService == nil || s.rateLimitService.settingService == nil {
		return nil
	}
	return s.rateLimitService.settingService.settingRepo
}

func (s *OpenAIGatewayService) openAIAdvancedSchedulerRuntimeSettings(ctx context.Context) openAIAdvancedSchedulerRuntimeSettings {
	if cached, ok := openAIAdvancedSchedulerSettingCache.Load().(*cachedOpenAIAdvancedSchedulerSetting); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return openAIAdvancedSchedulerRuntimeSettings{
				enabled: cached.enabled, stickyWeightedEnabled: cached.stickyWeightedEnabled,
				subscriptionPriorityEnabled: cached.subscriptionPriorityEnabled,
				lbTopKOverride:              cached.lbTopKOverride, weightOverrides: cloneOpenAIAdvancedSchedulerWeightOverrides(cached.weightOverrides),
			}
		}
	}

	result, _, _ := openAIAdvancedSchedulerSettingSF.Do(openAIAdvancedSchedulerSettingKey, func() (any, error) {
		if cached, ok := openAIAdvancedSchedulerSettingCache.Load().(*cachedOpenAIAdvancedSchedulerSetting); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return openAIAdvancedSchedulerRuntimeSettings{
					enabled: cached.enabled, stickyWeightedEnabled: cached.stickyWeightedEnabled,
					subscriptionPriorityEnabled: cached.subscriptionPriorityEnabled,
					lbTopKOverride:              cached.lbTopKOverride, weightOverrides: cloneOpenAIAdvancedSchedulerWeightOverrides(cached.weightOverrides),
				}, nil
			}
		}

		settings := openAIAdvancedSchedulerRuntimeSettings{weightOverrides: map[string]float64{}}
		if repo := s.openAIAdvancedSchedulerSettingRepo(); repo != nil {
			dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAIAdvancedSchedulerSettingDBTimeout)
			defer cancel()
			values, err := repo.GetMultiple(dbCtx, openAIAdvancedSchedulerRuntimeSettingKeys())
			if err != nil {
				slog.Warn("openai_advanced_scheduler_settings_batch_load_failed", "error", err)
				values = make(map[string]string)
				for _, key := range openAIAdvancedSchedulerRuntimeSettingKeys() {
					if value, valueErr := repo.GetValue(dbCtx, key); valueErr == nil {
						values[key] = value
					}
				}
			}
			settings.enabled = strings.EqualFold(strings.TrimSpace(values[openAIAdvancedSchedulerSettingKey]), "true")
			settings.stickyWeightedEnabled = strings.EqualFold(strings.TrimSpace(values[SettingKeyOpenAIAdvancedSchedulerStickyWeightedEnabled]), "true")
			settings.subscriptionPriorityEnabled = strings.EqualFold(strings.TrimSpace(values[SettingKeyOpenAIAdvancedSchedulerSubscriptionPriorityEnabled]), "true")
			settings.lbTopKOverride = parsePositiveIntOverride(values[SettingKeyOpenAIAdvancedSchedulerLBTopK])
			settings.weightOverrides = parseOpenAIAdvancedSchedulerWeightOverrides(values)
		}

		openAIAdvancedSchedulerSettingCache.Store(&cachedOpenAIAdvancedSchedulerSetting{
			enabled:                     settings.enabled,
			stickyWeightedEnabled:       settings.stickyWeightedEnabled,
			subscriptionPriorityEnabled: settings.subscriptionPriorityEnabled,
			lbTopKOverride:              settings.lbTopKOverride,
			weightOverrides:             cloneOpenAIAdvancedSchedulerWeightOverrides(settings.weightOverrides),
			expiresAt:                   time.Now().Add(openAIAdvancedSchedulerSettingCacheTTL).UnixNano(),
		})
		return settings, nil
	})

	settings, _ := result.(openAIAdvancedSchedulerRuntimeSettings)
	return settings
}

func (s *OpenAIGatewayService) isOpenAIAdvancedSchedulerEnabled(ctx context.Context) bool {
	return s.openAIAdvancedSchedulerRuntimeSettings(ctx).enabled
}

func (s *OpenAIGatewayService) isOpenAIAdvancedSchedulerStickyWeightedEnabled(ctx context.Context) bool {
	settings := s.openAIAdvancedSchedulerRuntimeSettings(ctx)
	return settings.enabled && settings.stickyWeightedEnabled
}

func (s *OpenAIGatewayService) isOpenAIAdvancedSchedulerSubscriptionPriorityEnabled(ctx context.Context) bool {
	settings := s.openAIAdvancedSchedulerRuntimeSettings(ctx)
	return settings.enabled && settings.subscriptionPriorityEnabled
}

func openAIAdvancedSchedulerRuntimeSettingKeys() []string {
	keys := []string{openAIAdvancedSchedulerSettingKey, SettingKeyOpenAIAdvancedSchedulerStickyWeightedEnabled, SettingKeyOpenAIAdvancedSchedulerSubscriptionPriorityEnabled, SettingKeyOpenAIAdvancedSchedulerLBTopK}
	for _, spec := range openAIAdvancedSchedulerWeightOverrideSpecs() {
		keys = append(keys, spec.key)
	}
	return keys
}

type openAIAdvancedSchedulerWeightOverrideSpec struct{ key, name string }

func openAIAdvancedSchedulerWeightOverrideSpecs() []openAIAdvancedSchedulerWeightOverrideSpec {
	return []openAIAdvancedSchedulerWeightOverrideSpec{
		{SettingKeyOpenAIAdvancedSchedulerWeightPriority, "priority"}, {SettingKeyOpenAIAdvancedSchedulerWeightLoad, "load"},
		{SettingKeyOpenAIAdvancedSchedulerWeightQueue, "queue"}, {SettingKeyOpenAIAdvancedSchedulerWeightErrorRate, "error_rate"},
		{SettingKeyOpenAIAdvancedSchedulerWeightTTFT, "ttft"}, {SettingKeyOpenAIAdvancedSchedulerWeightReset, "reset"},
		{SettingKeyOpenAIAdvancedSchedulerWeightQuotaHeadroom, "quota_headroom"}, {SettingKeyOpenAIAdvancedSchedulerWeightPreviousResponse, "previous_response"},
		{SettingKeyOpenAIAdvancedSchedulerWeightSessionSticky, "session_sticky"},
	}
}

func parsePositiveIntOverride(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func parseOpenAIAdvancedSchedulerWeightOverrides(values map[string]string) map[string]float64 {
	overrides := map[string]float64{}
	for _, spec := range openAIAdvancedSchedulerWeightOverrideSpecs() {
		raw := strings.TrimSpace(values[spec.key])
		if raw == "" {
			continue
		}
		value, err := strconv.ParseFloat(raw, 64)
		if err == nil && value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0) {
			overrides[spec.name] = value
		}
	}
	return overrides
}

func cloneOpenAIAdvancedSchedulerWeightOverrides(in map[string]float64) map[string]float64 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]float64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (s *OpenAIGatewayService) getOpenAIAccountScheduler(ctx context.Context) OpenAIAccountScheduler {
	if s == nil {
		return nil
	}
	if !s.isOpenAIAdvancedSchedulerEnabled(ctx) {
		return nil
	}
	s.openaiSchedulerOnce.Do(func() {
		if s.openaiAccountStats == nil {
			s.openaiAccountStats = newOpenAIAccountRuntimeStats()
		}
		if s.openaiScheduler == nil {
			s.openaiScheduler = newDefaultOpenAIAccountScheduler(s, s.openaiAccountStats)
		}
	})
	return s.openaiScheduler
}

func resetOpenAIAdvancedSchedulerSettingCacheForTest() {
	openAIAdvancedSchedulerSettingCache = atomic.Value{}
	openAIAdvancedSchedulerSettingSF = singleflight.Group{}
}

func (s *OpenAIGatewayService) SelectAccountWithScheduler(
	ctx context.Context,
	groupID *int64,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requireCompact bool,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	return s.selectAccountWithScheduler(ctx, groupID, previousResponseID, sessionHash, requestedModel, excludedIDs, requiredTransport, "", "", requireCompact, false, false)
}

func (s *OpenAIGatewayService) selectAccountByPreviousResponseIDForCapability(
	ctx context.Context,
	groupID *int64,
	previousResponseID string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredCapability OpenAIEndpointCapability,
	requiredImageCapability OpenAIImagesCapability,
	requireCompact bool,
) (*AccountSelectionResult, error) {
	selection, err := s.SelectAccountByPreviousResponseID(ctx, groupID, previousResponseID, requestedModel, excludedIDs, requireCompact)
	if err != nil || selection == nil || selection.Account == nil {
		return selection, err
	}
	if !accountSupportsOpenAICapabilities(selection.Account, requiredCapability, requiredImageCapability) {
		if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
		return nil, nil
	}
	return selection, nil
}

func (s *OpenAIGatewayService) SelectAccountWithSchedulerForCapability(
	ctx context.Context,
	groupID *int64,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requiredCapability OpenAIEndpointCapability,
	requireCompact bool,
	previousResponseCanMove bool,
	platformOverride ...string,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	return s.selectAccountWithScheduler(ctx, groupID, previousResponseID, sessionHash, requestedModel, excludedIDs, requiredTransport, requiredCapability, "", requireCompact, false, previousResponseCanMove, platformOverride...)
}

func (s *OpenAIGatewayService) SelectAccountWithSchedulerForClaudeGPTBridge(
	ctx context.Context,
	groupID *int64,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	return s.selectAccountWithScheduler(ctx, groupID, "", sessionHash, requestedModel, excludedIDs, requiredTransport, "", "", false, true, false)
}

func (s *OpenAIGatewayService) SelectAccountWithSchedulerForImages(
	ctx context.Context,
	groupID *int64,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredCapability OpenAIImagesCapability,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	selection, decision, err := s.selectAccountWithScheduler(ctx, groupID, "", sessionHash, requestedModel, excludedIDs, OpenAIUpstreamTransportHTTPSSE, "", requiredCapability, false, false, false)
	if err == nil && selection != nil && selection.Account != nil {
		return selection, decision, nil
	}
	// 如果要求 native 能力（如指定了模型）但没有可用的 APIKey 账号，回退到 basic（OAuth 账号）
	if requiredCapability == OpenAIImagesCapabilityNative {
		return s.selectAccountWithScheduler(ctx, groupID, "", sessionHash, requestedModel, excludedIDs, OpenAIUpstreamTransportHTTPSSE, "", OpenAIImagesCapabilityBasic, false, false, false)
	}
	return selection, decision, err
}

func (s *OpenAIGatewayService) selectAccountWithScheduler(
	ctx context.Context,
	groupID *int64,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requiredCapability OpenAIEndpointCapability,
	requiredImageCapability OpenAIImagesCapability,
	requireCompact bool,
	requireClaudeGPTBridge bool,
	previousResponseCanMove bool,
	platformOverride ...string,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	ctx = s.withOpenAIQuotaAutoPauseContext(ctx)
	platform := PlatformOpenAI
	if len(platformOverride) > 0 {
		platform = normalizeOpenAICompatiblePlatform(platformOverride[0])
	}
	decision := OpenAIAccountScheduleDecision{}
	scheduler := s.getOpenAIAccountScheduler(ctx)
	if scheduler == nil {
		decision.Layer = openAIAccountScheduleLayerLoadBalance
		if requiredTransport == OpenAIUpstreamTransportAny || requiredTransport == OpenAIUpstreamTransportHTTPSSE {
			effectiveExcludedIDs := cloneExcludedAccountIDs(excludedIDs)
			for {
				selection, err := s.selectAccountWithLoadAwarenessForSchedule(ctx, groupID, sessionHash, requestedModel, effectiveExcludedIDs, openAIAccountRequestEligibility{
					Platform:               platform,
					RequestedModel:         requestedModel,
					RequireCompact:         requireCompact,
					RequireClaudeGPTBridge: requireClaudeGPTBridge,
				})
				if err != nil {
					return nil, decision, err
				}
				if selection == nil || selection.Account == nil {
					return selection, decision, nil
				}
				if accountSupportsOpenAICapabilities(selection.Account, requiredCapability, requiredImageCapability) {
					return selection, decision, nil
				}
				if selection.ReleaseFunc != nil {
					selection.ReleaseFunc()
				}
				if effectiveExcludedIDs == nil {
					effectiveExcludedIDs = make(map[int64]struct{})
				}
				if _, exists := effectiveExcludedIDs[selection.Account.ID]; exists {
					return nil, decision, ErrNoAvailableAccounts
				}
				effectiveExcludedIDs[selection.Account.ID] = struct{}{}
			}
		}

		effectiveExcludedIDs := cloneExcludedAccountIDs(excludedIDs)
		for {
			selection, err := s.selectAccountWithLoadAwarenessForSchedule(ctx, groupID, sessionHash, requestedModel, effectiveExcludedIDs, openAIAccountRequestEligibility{
				Platform:               platform,
				RequestedModel:         requestedModel,
				RequireCompact:         requireCompact,
				RequireClaudeGPTBridge: requireClaudeGPTBridge,
			})
			if err != nil {
				return nil, decision, err
			}
			if selection == nil || selection.Account == nil {
				return selection, decision, nil
			}
			if s.isOpenAIAccountTransportCompatible(selection.Account, requiredTransport) &&
				accountSupportsOpenAICapabilities(selection.Account, requiredCapability, requiredImageCapability) {
				return selection, decision, nil
			}
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			if effectiveExcludedIDs == nil {
				effectiveExcludedIDs = make(map[int64]struct{})
			}
			if _, exists := effectiveExcludedIDs[selection.Account.ID]; exists {
				return nil, decision, ErrNoAvailableAccounts
			}
			effectiveExcludedIDs[selection.Account.ID] = struct{}{}
		}
	}

	var stickyAccountID int64
	if sessionHash != "" && s.cache != nil {
		if accountID, err := s.getStickySessionAccountID(ctx, groupID, sessionHash); err == nil && accountID > 0 {
			stickyAccountID = accountID
		}
	}
	stickyWeighted := s.isOpenAIAdvancedSchedulerStickyWeightedEnabled(ctx)
	subscriptionPriority := s.isOpenAIAdvancedSchedulerSubscriptionPriorityEnabled(ctx)
	stickyPreviousAccountID := int64(0)
	if stickyWeighted && previousResponseCanMove && strings.TrimSpace(previousResponseID) != "" && platform == PlatformOpenAI {
		if stickySelection, stickyErr := s.selectAccountByPreviousResponseIDForCapability(ctx, groupID, previousResponseID, requestedModel, excludedIDs, requiredCapability, requiredImageCapability, requireCompact); stickyErr == nil && stickySelection != nil && stickySelection.Account != nil {
			stickyPreviousAccountID = stickySelection.Account.ID
			if stickySelection.ReleaseFunc != nil {
				stickySelection.ReleaseFunc()
			}
		}
	}

	return scheduler.Select(ctx, OpenAIAccountScheduleRequest{
		GroupID:                 groupID,
		Platform:                platform,
		SessionHash:             sessionHash,
		StickyAccountID:         stickyAccountID,
		StickyPreviousAccountID: stickyPreviousAccountID,
		StickyWeighted:          stickyWeighted,
		SubscriptionPriority:    subscriptionPriority,
		PreviousResponseID:      previousResponseID,
		PreviousResponseCanMove: previousResponseCanMove,
		RequestedModel:          requestedModel,
		RequiredTransport:       requiredTransport,
		RequiredCapability:      requiredCapability,
		RequiredImageCapability: requiredImageCapability,
		RequireCompact:          requireCompact,
		RequireClaudeGPTBridge:  requireClaudeGPTBridge,
		ExcludedIDs:             excludedIDs,
	})
}

func accountSupportsOpenAICapabilities(account *Account, requiredCapability OpenAIEndpointCapability, requiredImageCapability OpenAIImagesCapability) bool {
	if account == nil {
		return false
	}
	return account.SupportsOpenAIEndpointCapability(requiredCapability) &&
		account.SupportsOpenAIImageCapability(requiredImageCapability)
}

func cloneExcludedAccountIDs(excludedIDs map[int64]struct{}) map[int64]struct{} {
	if len(excludedIDs) == 0 {
		return nil
	}
	cloned := make(map[int64]struct{}, len(excludedIDs))
	for id := range excludedIDs {
		cloned[id] = struct{}{}
	}
	return cloned
}

func (s *OpenAIGatewayService) isOpenAIAccountTransportCompatible(account *Account, requiredTransport OpenAIUpstreamTransport) bool {
	if requiredTransport == OpenAIUpstreamTransportAny || requiredTransport == OpenAIUpstreamTransportHTTPSSE {
		return true
	}
	if s == nil || account == nil {
		return false
	}
	return s.getOpenAIWSProtocolResolver().Resolve(account).Transport == requiredTransport
}

func (s *OpenAIGatewayService) ReportOpenAIAccountScheduleResult(accountID int64, success bool, firstTokenMs *int) {
	if s != nil && s.openaiSchedulerForTest != nil {
		s.openaiSchedulerForTest.ReportResult(accountID, success, firstTokenMs)
		return
	}
	scheduler := s.getOpenAIAccountScheduler(context.Background())
	if scheduler == nil {
		return
	}
	scheduler.ReportResult(accountID, success, firstTokenMs)
}

func (s *OpenAIGatewayService) RecordOpenAIAccountSwitch() {
	scheduler := s.getOpenAIAccountScheduler(context.Background())
	if scheduler == nil {
		return
	}
	scheduler.ReportSwitch()
}

func (s *OpenAIGatewayService) SnapshotOpenAIAccountSchedulerMetrics() OpenAIAccountSchedulerMetricsSnapshot {
	scheduler := s.getOpenAIAccountScheduler(context.Background())
	if scheduler == nil {
		return OpenAIAccountSchedulerMetricsSnapshot{}
	}
	return scheduler.SnapshotMetrics()
}

func (s *OpenAIGatewayService) openAIWSSessionStickyTTL() time.Duration {
	if s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds > 0 {
		return time.Duration(s.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds) * time.Second
	}
	return openaiStickySessionTTL
}

func (s *OpenAIGatewayService) openAIWSLBTopK() int {
	if s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.LBTopK > 0 {
		return s.cfg.Gateway.OpenAIWS.LBTopK
	}
	return 7
}

func (s *OpenAIGatewayService) openAIWSLBTopKForRequest(ctx context.Context) int {
	base := s.openAIWSLBTopK()
	settings := s.openAIAdvancedSchedulerRuntimeSettings(ctx)
	if settings.enabled && settings.lbTopKOverride > 0 {
		return settings.lbTopKOverride
	}
	return base
}

func (s *OpenAIGatewayService) openAIStickyEscapeConfig() openAIStickyEscapeConfig {
	if s != nil && s.cfg != nil {
		cfg := s.cfg.Gateway.OpenAIScheduler
		enabled := cfg.StickyEscapeEnabled
		if !enabled && cfg.StickyEscapeTTFTMs == 0 && cfg.StickyEscapeErrorRate == 0 {
			enabled = true
		}
		ttftMs := float64(cfg.StickyEscapeTTFTMs)
		if ttftMs <= 0 {
			ttftMs = 15000
		}
		errorRate := cfg.StickyEscapeErrorRate
		if errorRate < 0 || errorRate > 1 || (errorRate == 0 && cfg.StickyEscapeTTFTMs == 0) {
			errorRate = 0.5
		}
		return openAIStickyEscapeConfig{enabled: enabled, ttftMs: ttftMs, errorRate: errorRate}
	}
	return openAIStickyEscapeConfig{enabled: true, ttftMs: 15000, errorRate: 0.5}
}

func (s *OpenAIGatewayService) openAIWSSchedulerWeights() GatewayOpenAIWSSchedulerScoreWeightsView {
	if s != nil && s.cfg != nil {
		return GatewayOpenAIWSSchedulerScoreWeightsView{
			Priority:      s.cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority,
			Load:          s.cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load,
			Queue:         s.cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue,
			ErrorRate:     s.cfg.Gateway.OpenAIWS.SchedulerScoreWeights.ErrorRate,
			TTFT:          s.cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT,
			Reset:         s.cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Reset,
			QuotaHeadroom: s.cfg.Gateway.OpenAIWS.SchedulerScoreWeights.QuotaHeadroom,
			Previous:      s.cfg.Gateway.OpenAIWS.SchedulerScoreWeights.PreviousResponse,
			SessionSticky: s.cfg.Gateway.OpenAIWS.SchedulerScoreWeights.SessionSticky,
		}
	}
	return GatewayOpenAIWSSchedulerScoreWeightsView{
		Priority:      1.0,
		Load:          1.0,
		Queue:         0.7,
		ErrorRate:     0.8,
		TTFT:          0.5,
		Reset:         0,
		QuotaHeadroom: 0,
		Previous:      5,
		SessionSticky: 3,
	}
}

func (s *OpenAIGatewayService) openAIWSSchedulerWeightsForRequest(ctx context.Context) GatewayOpenAIWSSchedulerScoreWeightsView {
	weights := s.openAIWSSchedulerWeights()
	settings := s.openAIAdvancedSchedulerRuntimeSettings(ctx)
	if !settings.enabled {
		return weights
	}
	for name, value := range settings.weightOverrides {
		switch name {
		case "priority":
			weights.Priority = value
		case "load":
			weights.Load = value
		case "queue":
			weights.Queue = value
		case "error_rate":
			weights.ErrorRate = value
		case "ttft":
			weights.TTFT = value
		case "reset":
			weights.Reset = value
		case "quota_headroom":
			weights.QuotaHeadroom = value
		case "previous_response":
			weights.Previous = value
		case "session_sticky":
			weights.SessionSticky = value
		}
	}
	return weights
}

type GatewayOpenAIWSSchedulerScoreWeightsView struct {
	Priority      float64
	Load          float64
	Queue         float64
	ErrorRate     float64
	TTFT          float64
	Reset         float64
	QuotaHeadroom float64
	Previous      float64
	SessionSticky float64
}

type OpenAIAccountSchedulerScoreSnapshot struct {
	BaseScore             float64
	StickyScore           float64
	StickyScoreInfinity   bool
	StickyWeightedEnabled bool
}

func (s *RateLimitService) BuildOpenAIAccountSchedulerScoreSnapshot(ctx context.Context, accounts []*Account, loadMap map[int64]*AccountLoadInfo) map[int64]OpenAIAccountSchedulerScoreSnapshot {
	gateway := &OpenAIGatewayService{}
	if s != nil {
		gateway.cfg = s.cfg
		gateway.rateLimitService = s
	}
	return buildOpenAIAccountSchedulerScoreSnapshot(accounts, loadMap, gateway.openAIWSSchedulerWeightsForRequest(ctx), gateway.isOpenAIAdvancedSchedulerStickyWeightedEnabled(ctx))
}

func BuildOpenAIAccountSchedulerScoreSnapshot(accounts []*Account, loadMap map[int64]*AccountLoadInfo) map[int64]OpenAIAccountSchedulerScoreSnapshot {
	gateway := &OpenAIGatewayService{}
	return buildOpenAIAccountSchedulerScoreSnapshot(accounts, loadMap, gateway.openAIWSSchedulerWeights(), false)
}

func buildOpenAIAccountSchedulerScoreSnapshot(accounts []*Account, loadMap map[int64]*AccountLoadInfo, weights GatewayOpenAIWSSchedulerScoreWeightsView, stickyWeighted bool) map[int64]OpenAIAccountSchedulerScoreSnapshot {
	result := make(map[int64]OpenAIAccountSchedulerScoreSnapshot, len(accounts))
	if len(accounts) == 0 {
		return result
	}
	minPriority, maxPriority, maxWaiting := accounts[0].Priority, accounts[0].Priority, 1
	now := time.Now()
	minResetRemaining, maxResetRemaining := 0.0, 0.0
	hasResetSample := false
	for _, account := range accounts {
		if account == nil {
			continue
		}
		if account.Priority < minPriority {
			minPriority = account.Priority
		}
		if account.Priority > maxPriority {
			maxPriority = account.Priority
		}
		if load := loadMap[account.ID]; load != nil && load.WaitingCount > maxWaiting {
			maxWaiting = load.WaitingCount
		}
		if weights.Reset > 0 && account.SessionWindowEnd != nil && now.Before(*account.SessionWindowEnd) {
			remaining := account.SessionWindowEnd.Sub(now).Seconds()
			if !hasResetSample {
				minResetRemaining, maxResetRemaining, hasResetSample = remaining, remaining, true
			} else {
				minResetRemaining = math.Min(minResetRemaining, remaining)
				maxResetRemaining = math.Max(maxResetRemaining, remaining)
			}
		}
	}
	for _, account := range accounts {
		if account == nil {
			continue
		}
		load := loadMap[account.ID]
		if load == nil {
			load = &AccountLoadInfo{AccountID: account.ID}
		}
		priorityFactor := 1.0
		if maxPriority > minPriority {
			priorityFactor = 1 - float64(account.Priority-minPriority)/float64(maxPriority-minPriority)
		}
		base := weights.Priority*priorityFactor + weights.Load*(1-clamp01(float64(load.LoadRate)/100)) + weights.Queue*(1-clamp01(float64(load.WaitingCount)/float64(maxWaiting))) + weights.ErrorRate + weights.TTFT*0.5
		if weights.Reset > 0 && hasResetSample && account.SessionWindowEnd != nil && now.Before(*account.SessionWindowEnd) {
			resetFactor := 1.0
			if maxResetRemaining > minResetRemaining {
				resetFactor = 1 - clamp01((account.SessionWindowEnd.Sub(now).Seconds()-minResetRemaining)/(maxResetRemaining-minResetRemaining))
			}
			base += weights.Reset * resetFactor
		}
		if weights.QuotaHeadroom > 0 {
			base += weights.QuotaHeadroom * openAIQuotaHeadroomFactor(account, now)
		}
		snapshot := OpenAIAccountSchedulerScoreSnapshot{BaseScore: base, StickyWeightedEnabled: stickyWeighted, StickyScoreInfinity: !stickyWeighted}
		if stickyWeighted {
			snapshot.StickyScore = base + weights.Previous + weights.SessionSticky
		}
		result[account.ID] = snapshot
	}
	return result
}

func openAIQuotaHeadroomFactor(account *Account, now time.Time) float64 {
	if account == nil || len(account.Extra) == 0 || openAIQuotaHeadroomSnapshotStale(account.Extra, now) {
		return openAIQuotaHeadroomNeutralFactor
	}
	primaryUsed, ok := firstAccountExtraNumber(account.Extra, "codex_primary_used_percent", "codex_7d_used_percent")
	if !ok {
		return openAIQuotaHeadroomNeutralFactor
	}
	factor := 1 - clamp01(primaryUsed/100)
	if secondaryUsed, ok := firstAccountExtraNumber(account.Extra, "codex_secondary_used_percent", "codex_5h_used_percent"); ok {
		secondaryRemaining := 1 - clamp01(secondaryUsed/100)
		if secondaryRemaining < openAIQuotaHeadroomSecondaryLowRemain {
			factor *= openAIQuotaHeadroomNeutralFactor
		}
	}
	return factor
}

func openAIQuotaHeadroomSnapshotStale(extra map[string]any, now time.Time) bool {
	raw, ok := extra["codex_usage_updated_at"]
	if !ok {
		return true
	}
	updatedAt, err := parseTime(fmt.Sprint(raw))
	if err != nil || updatedAt.After(now) {
		return true
	}
	return now.Sub(updatedAt) >= openAIQuotaHeadroomSnapshotStaleAfter
}

func firstAccountExtraNumber(extra map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		raw, ok := extra[key]
		if !ok || raw == nil {
			continue
		}
		value := parseExtraFloat64(raw)
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		return value, true
	}
	return 0, false
}

func clamp01(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}

func calcLoadSkewByMoments(sum float64, sumSquares float64, count int) float64 {
	if count <= 1 {
		return 0
	}
	mean := sum / float64(count)
	variance := sumSquares/float64(count) - mean*mean
	if variance < 0 {
		variance = 0
	}
	return math.Sqrt(variance)
}
