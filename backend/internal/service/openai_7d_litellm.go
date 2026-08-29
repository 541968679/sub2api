package service

import (
	"context"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"
)

const (
	openAI7dDefaultWindow   = 7 * 24 * time.Hour
	openAI7dResetSlop       = time.Hour
	openAI7dLiteLLMCacheTTL = time.Minute
	codex7dResetAtKey       = "codex_7d_reset_at"
	codex7dUsedPercentKey   = "codex_7d_used_percent"
	codex7dWindowSecondsKey = "codex_7d_window_seconds"
	codex7dWindowMinutesKey = "codex_7d_window_minutes"
)

// OpenAI7dCycle is one closed Codex 7-day window snapshot.
type OpenAI7dCycle struct {
	AccountID   int64
	WindowStart time.Time
	WindowEnd   time.Time
	LiteLLMCost float64
	UsedPercent float64
	ClosedAt    time.Time
}

// OpenAI7dPreviousCycle is the latest closed cycle exposed on usage JSON.
type OpenAI7dPreviousCycle struct {
	LiteLLMCost float64   `json:"litellm_cost"`
	UsedPercent float64   `json:"used_percent"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
}

// AccountModelTokenTotals is per-price-model token mass in a time window.
type AccountModelTokenTotals struct {
	Model                 string
	InputTokens           int64
	OutputTokens          int64
	CacheCreationTokens   int64
	CacheCreation5mTokens int64
	CacheCreation1hTokens int64
	CacheReadTokens       int64
	ImageOutputTokens     int64
	ImageCount            int64
}

const openAI7dHistoryLimit = 36

// OpenAI7dWindowAccountStats is the A$/U$ token mass for one Codex window.
// AccountCost must use the same formula as GetAccountWindowStats.
type OpenAI7dWindowAccountStats struct {
	Requests    int64
	Tokens      int64
	AccountCost float64
	UserCost    float64
}

// OpenAI7dCycleHistoryItem is one current or closed cycle for the admin dialog.
type OpenAI7dCycleHistoryItem struct {
	Current     bool      `json:"current"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	LiteLLMCost float64   `json:"litellm_cost"`
	AccountCost float64   `json:"account_cost"`
	UserCost    float64   `json:"user_cost"`
	UsedPercent float64   `json:"used_percent"`
	Requests    int64     `json:"requests"`
	Tokens      int64     `json:"tokens"`
}

// OpenAI7dCycleHistory is the admin dialog payload.
type OpenAI7dCycleHistory struct {
	Items []OpenAI7dCycleHistoryItem `json:"items"`
}

// OpenAI7dCycleRepository persists closed cycles and aggregates usage_logs tokens.
type OpenAI7dCycleRepository interface {
	GetAccountModelTokenTotals(ctx context.Context, accountID int64, start, end time.Time) ([]AccountModelTokenTotals, error)
	InsertCycle(ctx context.Context, cycle OpenAI7dCycle) error
	GetLatestCycle(ctx context.Context, accountID int64) (*OpenAI7dCycle, error)
	ListCycles(ctx context.Context, accountID int64, limit int) ([]OpenAI7dCycle, error)
	GetWindowAccountStats(ctx context.Context, accountID int64, start, end time.Time) (*OpenAI7dWindowAccountStats, error)
}

// OpenAI7dLiteLLMCycleService computes on-the-fly LiteLLM cost and closes cycles
// when Codex 7d reset_at jumps forward.
type OpenAI7dLiteLLMCycleService struct {
	repo    OpenAI7dCycleRepository
	pricing *PricingService
	cache   sync.Map // accountID -> *openAI7dLiteLLMCache
}

type openAI7dLiteLLMCache struct {
	windowStart time.Time
	cost        *float64
	timestamp   time.Time
}

func NewOpenAI7dLiteLLMCycleService(repo OpenAI7dCycleRepository, pricing *PricingService) *OpenAI7dLiteLLMCycleService {
	return &OpenAI7dLiteLLMCycleService{repo: repo, pricing: pricing}
}

func openAI7dLiteLLMPriceModel(upstreamModel, requestedModel, model string) string {
	if m := strings.TrimSpace(upstreamModel); m != "" {
		return m
	}
	if m := strings.TrimSpace(requestedModel); m != "" {
		return m
	}
	return strings.TrimSpace(model)
}

func computeLiteLLMStandardCost(pricing *LiteLLMModelPricing, tokens AccountModelTokenTotals) float64 {
	if pricing == nil {
		return 0
	}
	write5m := tokens.CacheCreation5mTokens
	write1h := tokens.CacheCreation1hTokens
	if write5m == 0 && write1h == 0 {
		write5m = tokens.CacheCreationTokens
	}
	write1hPrice := pricing.CacheCreationInputTokenCostAbove1hr
	if write1hPrice <= 0 {
		write1hPrice = pricing.CacheCreationInputTokenCost
	}
	cost := float64(tokens.InputTokens)*pricing.InputCostPerToken +
		float64(tokens.OutputTokens)*pricing.OutputCostPerToken +
		float64(write5m)*pricing.CacheCreationInputTokenCost +
		float64(write1h)*write1hPrice +
		float64(tokens.CacheReadTokens)*pricing.CacheReadInputTokenCost +
		float64(tokens.ImageOutputTokens)*pricing.OutputCostPerImageToken
	if tokens.ImageCount > 0 && pricing.OutputCostPerImage > 0 {
		cost += float64(tokens.ImageCount) * pricing.OutputCostPerImage
	}
	if !isFiniteTrueCost(cost) {
		return 0
	}
	return cost
}

func parseCodexExtraTimeLoose(extra map[string]any, key string) (time.Time, bool) {
	if extra == nil {
		return time.Time{}, false
	}
	raw, ok := extra[key]
	if !ok || raw == nil {
		return time.Time{}, false
	}
	if t, ok := raw.(time.Time); ok && !t.IsZero() {
		return t.UTC(), true
	}
	s, ok := raw.(string)
	if !ok {
		return time.Time{}, false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	t, err := parseTime(s)
	if err != nil || t.IsZero() {
		return time.Time{}, false
	}
	return t.UTC(), true
}

func openAI7dWindowDuration(extra map[string]any) time.Duration {
	if extra == nil {
		return openAI7dDefaultWindow
	}
	if secs := parseExtraInt(extra[codex7dWindowSecondsKey]); secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if mins := parseExtraInt(extra[codex7dWindowMinutesKey]); mins > 0 {
		return time.Duration(mins) * time.Minute
	}
	return openAI7dDefaultWindow
}

func shouldCloseOpenAI7dCycle(oldResetAt, newResetAt time.Time) bool {
	if oldResetAt.IsZero() || newResetAt.IsZero() {
		return false
	}
	return newResetAt.Sub(oldResetAt) >= openAI7dResetSlop
}

func resolveOpenAI7dWindow(extra map[string]any, last *OpenAI7dCycle, now time.Time) (start, end time.Time, ok bool) {
	end, ok = parseCodexExtraTimeLoose(extra, codex7dResetAtKey)
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	if !now.Before(end) {
		return time.Time{}, end, false
	}
	start = end.Add(-openAI7dWindowDuration(extra))
	if last != nil && !last.WindowEnd.IsZero() && last.WindowEnd.After(start) && last.WindowEnd.Before(end) {
		start = last.WindowEnd
	}
	return start, end, true
}

func (s *OpenAI7dLiteLLMCycleService) sumLiteLLMCost(ctx context.Context, accountID int64, start, end time.Time) float64 {
	if s == nil || s.repo == nil || accountID <= 0 || !end.After(start) {
		return 0
	}
	rows, err := s.repo.GetAccountModelTokenTotals(ctx, accountID, start, end)
	if err != nil {
		slog.Warn("openai_7d_litellm_aggregate_failed", "account_id", accountID, "error", err)
		return 0
	}
	var total float64
	for _, row := range rows {
		var pricing *LiteLLMModelPricing
		if s.pricing != nil && row.Model != "" {
			pricing = s.pricing.GetModelPricing(row.Model)
		}
		total += computeLiteLLMStandardCost(pricing, row)
	}
	if !isFiniteTrueCost(total) {
		return 0
	}
	return total
}

func (s *OpenAI7dLiteLLMCycleService) cachedCurrentCost(ctx context.Context, accountID int64, start, end, now time.Time) *float64 {
	if s == nil {
		return nil
	}
	if cached, ok := s.cache.Load(accountID); ok {
		if entry, ok := cached.(*openAI7dLiteLLMCache); ok &&
			entry != nil &&
			entry.windowStart.Equal(start) &&
			now.Sub(entry.timestamp) < openAI7dLiteLLMCacheTTL {
			return entry.cost
		}
	}
	queryEnd := end
	if now.Before(end) {
		queryEnd = now
	}
	cost := s.sumLiteLLMCost(ctx, accountID, start, queryEnd)
	if math.IsNaN(cost) || math.IsInf(cost, 0) {
		cost = 0
	}
	s.cache.Store(accountID, &openAI7dLiteLLMCache{
		windowStart: start,
		cost:        &cost,
		timestamp:   now,
	})
	return &cost
}

// CloseIfReset persists the previous window when new Codex 7d reset_at jumps forward.
// Must run before extra is merged/updated so used_percent and old reset_at are still the old values.
func (s *OpenAI7dLiteLLMCycleService) CloseIfReset(ctx context.Context, account *Account, newUpdates map[string]any) {
	if s == nil || s.repo == nil || account == nil || account.Platform != PlatformOpenAI || !account.IsOAuth() {
		return
	}
	if len(newUpdates) == 0 {
		return
	}
	newResetAt, ok := parseCodexExtraTimeLoose(newUpdates, codex7dResetAtKey)
	if !ok {
		return
	}
	oldResetAt, ok := parseCodexExtraTimeLoose(account.Extra, codex7dResetAtKey)
	if !ok || !shouldCloseOpenAI7dCycle(oldResetAt, newResetAt) {
		return
	}
	now := time.Now().UTC()
	windowEnd := oldResetAt
	windowStart := windowEnd.Add(-openAI7dWindowDuration(account.Extra))
	queryEnd := windowEnd
	if now.Before(windowEnd) {
		queryEnd = now
	}
	cycle := OpenAI7dCycle{
		AccountID:   account.ID,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		LiteLLMCost: s.sumLiteLLMCost(ctx, account.ID, windowStart, queryEnd),
		UsedPercent: parseExtraFloat64(account.Extra[codex7dUsedPercentKey]),
		ClosedAt:    now,
	}
	if err := s.repo.InsertCycle(ctx, cycle); err != nil {
		slog.Warn("openai_7d_litellm_cycle_insert_failed", "account_id", account.ID, "error", err)
		return
	}
	s.cache.Delete(account.ID)
}

// AttachCurrent fills seven_day.litellm_cost and previous_cycle. Failures leave them unset.
func (s *OpenAI7dLiteLLMCycleService) AttachCurrent(ctx context.Context, account *Account, usage *UsageInfo) {
	if s == nil || account == nil || usage == nil || usage.SevenDay == nil {
		return
	}
	if account.Platform != PlatformOpenAI || !account.IsOAuth() {
		return
	}
	now := time.Now().UTC()
	var last *OpenAI7dCycle
	if s.repo != nil {
		cycle, err := s.repo.GetLatestCycle(ctx, account.ID)
		if err != nil {
			slog.Warn("openai_7d_litellm_latest_failed", "account_id", account.ID, "error", err)
		} else {
			last = cycle
		}
	}
	if last != nil {
		usage.SevenDay.PreviousCycle = &OpenAI7dPreviousCycle{
			LiteLLMCost: last.LiteLLMCost,
			UsedPercent: last.UsedPercent,
			WindowStart: last.WindowStart,
			WindowEnd:   last.WindowEnd,
		}
	}
	if _, hasEnd := parseCodexExtraTimeLoose(account.Extra, codex7dResetAtKey); !hasEnd {
		return
	}
	start, end, ok := resolveOpenAI7dWindow(account.Extra, last, now)
	if !ok {
		zero := 0.0
		usage.SevenDay.LiteLLMCost = &zero
		return
	}
	usage.SevenDay.LiteLLMCost = s.cachedCurrentCost(ctx, account.ID, start, end, now)
}

func (s *OpenAI7dLiteLLMCycleService) windowAccountStats(ctx context.Context, accountID int64, start, end time.Time) OpenAI7dWindowAccountStats {
	if s == nil || s.repo == nil {
		return OpenAI7dWindowAccountStats{}
	}
	stats, err := s.repo.GetWindowAccountStats(ctx, accountID, start, end)
	if err != nil {
		slog.Warn("openai_7d_window_account_stats_failed", "account_id", accountID, "error", err)
		return OpenAI7dWindowAccountStats{}
	}
	if stats == nil {
		return OpenAI7dWindowAccountStats{}
	}
	return *stats
}

func (s *OpenAI7dLiteLLMCycleService) historyItem(ctx context.Context, accountID int64, start, end time.Time, litellmCost, usedPercent float64, current bool) OpenAI7dCycleHistoryItem {
	queryEnd := end
	if current {
		now := time.Now().UTC()
		if now.Before(end) {
			queryEnd = now
		}
	}
	stats := s.windowAccountStats(ctx, accountID, start, queryEnd)
	return OpenAI7dCycleHistoryItem{
		Current:     current,
		WindowStart: start,
		WindowEnd:   end,
		LiteLLMCost: litellmCost,
		AccountCost: stats.AccountCost,
		UserCost:    stats.UserCost,
		UsedPercent: usedPercent,
		Requests:    stats.Requests,
		Tokens:      stats.Tokens,
	}
}

// ListHistory returns the current Codex 7d window plus recent closed cycles.
func (s *OpenAI7dLiteLLMCycleService) ListHistory(ctx context.Context, account *Account) *OpenAI7dCycleHistory {
	out := &OpenAI7dCycleHistory{Items: []OpenAI7dCycleHistoryItem{}}
	if s == nil || account == nil || account.Platform != PlatformOpenAI || !account.IsOAuth() {
		return out
	}
	now := time.Now().UTC()
	var last *OpenAI7dCycle
	if s.repo != nil {
		if cycles, err := s.repo.ListCycles(ctx, account.ID, openAI7dHistoryLimit); err != nil {
			slog.Warn("openai_7d_litellm_list_failed", "account_id", account.ID, "error", err)
		} else {
			for _, cycle := range cycles {
				c := cycle
				if last == nil {
					last = &c
				}
				out.Items = append(out.Items, s.historyItem(
					ctx, account.ID, cycle.WindowStart, cycle.WindowEnd,
					cycle.LiteLLMCost, cycle.UsedPercent, false,
				))
			}
		}
	}
	if start, end, ok := resolveOpenAI7dWindow(account.Extra, last, now); ok {
		var cost float64
		if ptr := s.cachedCurrentCost(ctx, account.ID, start, end, now); ptr != nil {
			cost = *ptr
		}
		current := s.historyItem(
			ctx, account.ID, start, end, cost,
			parseExtraFloat64(account.Extra[codex7dUsedPercentKey]), true,
		)
		out.Items = append([]OpenAI7dCycleHistoryItem{current}, out.Items...)
		return out
	}
	// List L$ still renders $0 when reset_at is in the past. Keep that chip
	// behavior, but the dialog should still show the last known window.
	if end, hasEnd := parseCodexExtraTimeLoose(account.Extra, codex7dResetAtKey); hasEnd {
		start := end.Add(-openAI7dWindowDuration(account.Extra))
		if last != nil && !last.WindowEnd.IsZero() && last.WindowEnd.After(start) && last.WindowEnd.Before(end) {
			start = last.WindowEnd
		}
		if end.After(start) {
			current := s.historyItem(
				ctx, account.ID, start, end,
				s.sumLiteLLMCost(ctx, account.ID, start, end),
				parseExtraFloat64(account.Extra[codex7dUsedPercentKey]), true,
			)
			out.Items = append([]OpenAI7dCycleHistoryItem{current}, out.Items...)
		}
	}
	return out
}
