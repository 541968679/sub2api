package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

type OpenAIMessagesDispatchModelConfig = domain.OpenAIMessagesDispatchModelConfig
type GroupModelsListConfig = domain.GroupModelsListConfig

type Group struct {
	ID             int64
	Name           string
	Description    string
	Platform       string
	RateMultiplier float64
	// 高峰时段倍率：peak_rate_enabled 为 true 且当前时刻处于 [PeakStart, PeakEnd) 时，
	// token 计费倍率额外乘以 PeakRateMultiplier。详见 PeakMultiplierAt。
	PeakRateEnabled    bool
	PeakStart          string
	PeakEnd            string
	PeakRateMultiplier float64
	IsExclusive        bool
	Status         string
	Hydrated       bool

	SubscriptionType    string
	DailyLimitUSD       *float64
	WeeklyLimitUSD      *float64
	MonthlyLimitUSD     *float64
	DefaultValidityDays int

	ImagePrice1K *float64
	ImagePrice2K *float64
	ImagePrice4K *float64

	AllowImageGeneration         bool
	AllowBatchImageGeneration    bool
	ImageRateIndependent         bool
	ImageRateMultiplier          float64
	BatchImageDiscountMultiplier float64
	BatchImageHoldMultiplier     float64
	VideoRateIndependent         bool
	VideoRateMultiplier          float64
	VideoPrice480P               *float64
	VideoPrice720P               *float64
	VideoPrice1080P              *float64

	ClaudeCodeOnly                  bool
	FallbackGroupID                 *int64
	FallbackGroupIDOnInvalidRequest *int64

	ModelRouting         map[string][]int64
	ModelRoutingEnabled  bool
	MCPXMLInject         bool
	SupportedModelScopes []string

	BlockedModels []string
	AllowedModels []string

	SortOrder int

	AllowMessagesDispatch       bool
	RequireOAuthOnly            bool
	RequirePrivacySet           bool
	DefaultMappedModel          string
	MessagesDispatchModelConfig OpenAIMessagesDispatchModelConfig
	ModelsListConfig            GroupModelsListConfig

	RPMLimit int

	CreatedAt time.Time
	UpdatedAt time.Time

	AccountGroups           []AccountGroup
	AccountCount            int64
	ActiveAccountCount      int64
	RateLimitedAccountCount int64
}

func (g *Group) IsActive() bool {
	return g.Status == StatusActive
}

func (g *Group) IsSubscriptionType() bool {
	return g.SubscriptionType == SubscriptionTypeSubscription
}

func (g *Group) HasDailyLimit() bool {
	return g.DailyLimitUSD != nil && *g.DailyLimitUSD > 0
}

func (g *Group) HasWeeklyLimit() bool {
	return g.WeeklyLimitUSD != nil && *g.WeeklyLimitUSD > 0
}

func (g *Group) HasMonthlyLimit() bool {
	return g.MonthlyLimitUSD != nil && *g.MonthlyLimitUSD > 0
}

// IsModelAllowed reports whether the requested model is allowed for this group.
func (g *Group) IsModelAllowed(requestedModel string) bool {
	if g == nil {
		return true
	}
	model := normalizeModelAccessValue(requestedModel)
	if model == "" {
		return true
	}
	for _, pattern := range g.BlockedModels {
		if matchModelAccessPattern(pattern, model) {
			return false
		}
	}
	if len(g.AllowedModels) > 0 {
		for _, pattern := range g.AllowedModels {
			if matchModelAccessPattern(pattern, model) {
				return true
			}
		}
		return false
	}
	return true
}

// GetImagePrice returns the configured image price for the requested size.
func (g *Group) GetImagePrice(imageSize string) *float64 {
	switch imageSize {
	case "1K":
		return g.ImagePrice1K
	case "2K":
		return g.ImagePrice2K
	case "4K":
		return g.ImagePrice4K
	default:
		return g.ImagePrice2K
	}
}

// IsGroupContextValid reports whether a group from context has the fields required for routing decisions.
func IsGroupContextValid(group *Group) bool {
	if group == nil {
		return false
	}
	if group.ID <= 0 {
		return false
	}
	if !group.Hydrated {
		return false
	}
	if group.Platform == "" || group.Status == "" {
		return false
	}
	return true
}

// GetRoutingAccountIDs returns account IDs for a request model.
func (g *Group) GetRoutingAccountIDs(requestedModel string) []int64 {
	if !g.ModelRoutingEnabled || len(g.ModelRouting) == 0 || requestedModel == "" {
		return nil
	}
	if accountIDs, ok := g.ModelRouting[requestedModel]; ok && len(accountIDs) > 0 {
		return accountIDs
	}
	for pattern, accountIDs := range g.ModelRouting {
		if matchModelPattern(pattern, requestedModel) && len(accountIDs) > 0 {
			return accountIDs
		}
	}
	return nil
}

func matchModelPattern(pattern, model string) bool {
	if pattern == model {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(model, prefix)
	}
	return false
}

func normalizeModelAccessValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeModelAccessPatterns(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		pattern := normalizeModelAccessValue(value)
		if pattern == "" {
			continue
		}
		if _, ok := seen[pattern]; ok {
			continue
		}
		seen[pattern] = struct{}{}
		out = append(out, pattern)
	}
	return out
}

func matchModelAccessPattern(pattern, model string) bool {
	pattern = normalizeModelAccessValue(pattern)
	if pattern == "" {
		return false
	}
	if pattern == model {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(model, prefix)
	}
	return false
}

// parseMinutes 把 "HH:MM" 解析为当日分钟数（0..1439）。格式非法返回 (0,false)。
func parseMinutes(hhmm string) (int, bool) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

// PeakMultiplierAt 返回指定时刻 now 的高峰因子。
//   - 未启用 / 未配置 / 配置非法（start>=end 或格式错误） / 非高峰时段 → 返回 1.0（安全降级）
//   - 区间为左闭右开 [PeakStart, PeakEnd)，仅支持当日区间，不支持跨天（如 22:00-次日02:00）
//   - 时刻基于全局系统时区（timezone.Location）判定
// 该方法是纯函数，不读取任何外部状态，便于单测。
func (g *Group) PeakMultiplierAt(now time.Time) float64 {
	if g == nil || !g.IsSubscriptionType() || !g.PeakRateEnabled || g.PeakStart == "" || g.PeakEnd == "" {
		return 1.0
	}
	start, ok1 := parseMinutes(g.PeakStart)
	end, ok2 := parseMinutes(g.PeakEnd)
	if !ok1 || !ok2 || start >= end {
		return 1.0
	}
	t := now.In(timezone.Location())
	cur := t.Hour()*60 + t.Minute()
	if cur >= start && cur < end {
		return g.PeakRateMultiplier
	}
	return 1.0
}

// ValidatePeakRateConfig 是高峰倍率配置的唯一校验来源，供 handler 与 service 层共用。
// enabled=true 时仅允许订阅类型分组；并要求 start/end 合法且 end>start（不支持跨天），multiplier>=0。
// multiplier=0 是允许的，表示高峰 token 请求按 0 倍计费，可用于折扣/免费策略。
// enabled=false 时放行（不关心类型）。subscriptionType 为空按 standard 处理。
func ValidatePeakRateConfig(subscriptionType string, enabled bool, start, end string, multiplier float64) error {
	if !enabled {
		return nil
	}
	if subscriptionType != SubscriptionTypeSubscription {
		return errors.New("高峰时段倍率仅支持订阅类型分组")
	}
	if start == "" || end == "" {
		return errors.New("peak_rate_enabled 为 true 时 peak_start 与 peak_end 必填")
	}
	st, err1 := time.Parse("15:04", start)
	if err1 != nil {
		return fmt.Errorf("peak_start 格式应为 HH:MM，got %q", start)
	}
	en, err2 := time.Parse("15:04", end)
	if err2 != nil {
		return fmt.Errorf("peak_end 格式应为 HH:MM，got %q", end)
	}
	if st.Hour()*60+st.Minute() >= en.Hour()*60+en.Minute() {
		return errors.New("peak_end 必须大于 peak_start（不支持跨天区间，如 22:00-02:00）")
	}
	if multiplier < 0 {
		return errors.New("peak_rate_multiplier 不能为负")
	}
	return nil
}

// computePeakAwareMultipliers 把"基础 token 倍率 base"（已含系统/分组/用户级倍率，但不含高峰）
// 拆分为最终 token 倍率与图片按次倍率：图片按次倍率基于 base 现算、不受高峰影响；token 倍率在 base 上叠加高峰因子。
// gateway_service.recordUsageCore 与 openai_gateway_service.RecordUsage 共用此函数，
// 锁死"高峰因子只乘入 token 倍率、图片按次倍率不受影响"这一叠加顺序——任何调换都会被 group_peak_rate_test 覆盖。
func computePeakAwareMultipliers(apiKey *APIKey, base float64, now time.Time) (text, image float64) {
	image = resolveImageRateMultiplier(apiKey, base)
	peak := 1.0
	if apiKey != nil && apiKey.Group != nil {
		peak = apiKey.Group.PeakMultiplierAt(now)
	}
	text = base * peak
	return
}
