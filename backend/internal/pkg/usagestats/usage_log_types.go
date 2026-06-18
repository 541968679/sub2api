// Package usagestats provides types for usage statistics and reporting.
package usagestats

import "time"

const (
	ModelSourceRequested = "requested"
	ModelSourceUpstream  = "upstream"
	ModelSourceMapping   = "mapping"
)

func IsValidModelSource(source string) bool {
	switch source {
	case ModelSourceRequested, ModelSourceUpstream, ModelSourceMapping:
		return true
	default:
		return false
	}
}

func NormalizeModelSource(source string) string {
	if IsValidModelSource(source) {
		return source
	}
	return ModelSourceRequested
}

// DashboardStats 仪表盘统计
type DashboardStats struct {
	// 用户统计
	TotalUsers    int64 `json:"total_users"`
	TodayNewUsers int64 `json:"today_new_users"` // 今日新增用户数
	ActiveUsers   int64 `json:"active_users"`    // 今日有请求的用户数
	// 小时活跃用户数（UTC 当前小时）
	HourlyActiveUsers int64 `json:"hourly_active_users"`

	// 预聚合新鲜度
	StatsUpdatedAt string `json:"stats_updated_at"`
	StatsStale     bool   `json:"stats_stale"`

	// API Key 统计
	TotalAPIKeys  int64 `json:"total_api_keys"`
	ActiveAPIKeys int64 `json:"active_api_keys"` // 状态为 active 的 API Key 数

	// 账户统计
	TotalAccounts     int64 `json:"total_accounts"`
	NormalAccounts    int64 `json:"normal_accounts"`    // 正常账户数 (schedulable=true, status=active)
	ErrorAccounts     int64 `json:"error_accounts"`     // 异常账户数 (status=error)
	RateLimitAccounts int64 `json:"ratelimit_accounts"` // 限流账户数
	OverloadAccounts  int64 `json:"overload_accounts"`  // 过载账户数

	// 累计 Token 使用统计
	TotalRequests            int64   `json:"total_requests"`
	TotalInputTokens         int64   `json:"total_input_tokens"`
	TotalOutputTokens        int64   `json:"total_output_tokens"`
	TotalCacheCreationTokens int64   `json:"total_cache_creation_tokens"`
	TotalCacheReadTokens     int64   `json:"total_cache_read_tokens"`
	TotalTokens              int64   `json:"total_tokens"`
	TotalCost                float64 `json:"total_cost"`         // 累计标准计费
	TotalActualCost          float64 `json:"total_actual_cost"`  // 累计实际扣除
	TotalAccountCost         float64 `json:"total_account_cost"` // 累计账号成本

	// 今日 Token 使用统计
	TodayRequests            int64   `json:"today_requests"`
	TodayInputTokens         int64   `json:"today_input_tokens"`
	TodayOutputTokens        int64   `json:"today_output_tokens"`
	TodayCacheCreationTokens int64   `json:"today_cache_creation_tokens"`
	TodayCacheReadTokens     int64   `json:"today_cache_read_tokens"`
	TodayTokens              int64   `json:"today_tokens"`
	TodayCost                float64 `json:"today_cost"`         // 今日标准计费
	TodayActualCost          float64 `json:"today_actual_cost"`  // 今日实际扣除
	TodayAccountCost         float64 `json:"today_account_cost"` // 今日账号成本

	// 系统运行统计
	AverageDurationMs float64 `json:"average_duration_ms"` // 平均响应时间

	// 性能指标
	Rpm int64 `json:"rpm"` // 近5分钟平均每分钟请求数
	Tpm int64 `json:"tpm"` // 近5分钟平均每分钟Token数
}

// TrendDataPoint represents a single point in trend data
type TrendDataPoint struct {
	Date                string  `json:"date"`
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	Cost                float64 `json:"cost"`        // 标准计费
	ActualCost          float64 `json:"actual_cost"` // 实际扣除
}

// ModelStat represents usage statistics for a single model
type ModelStat struct {
	Model               string  `json:"model"`
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	Cost                float64 `json:"cost"`         // 标准计费
	ActualCost          float64 `json:"actual_cost"`  // 实际扣除
	AccountCost         float64 `json:"account_cost"` // 账号成本
}

// CacheStatusSummary describes prompt-cache effectiveness for a time window.
type CacheStatusSummary struct {
	Requests            int64   `json:"requests"`
	CacheHitRequests    int64   `json:"cache_hit_requests"`
	InputTokens         int64   `json:"input_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	PromptTotalTokens   int64   `json:"prompt_total_tokens"`
	CacheReadRate       float64 `json:"cache_read_rate"`
	CacheCreationRate   float64 `json:"cache_creation_rate"`
	RequestHitRate      float64 `json:"request_hit_rate"`
	Status              string  `json:"status"`
}

// CacheStatusTrendPoint describes cache effectiveness in one trend bucket.
type CacheStatusTrendPoint struct {
	Bucket              string  `json:"bucket"`
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	PromptTotalTokens   int64   `json:"prompt_total_tokens"`
	CacheReadRate       float64 `json:"cache_read_rate"`
	CacheCreationRate   float64 `json:"cache_creation_rate"`
}

// CacheStatusModelStat describes cache effectiveness grouped by requested and upstream model.
type CacheStatusModelStat struct {
	RequestedModel      string  `json:"requested_model"`
	UpstreamModel       string  `json:"upstream_model"`
	Requests            int64   `json:"requests"`
	CacheHitRequests    int64   `json:"cache_hit_requests"`
	InputTokens         int64   `json:"input_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	PromptTotalTokens   int64   `json:"prompt_total_tokens"`
	CacheReadRate       float64 `json:"cache_read_rate"`
	CacheCreationRate   float64 `json:"cache_creation_rate"`
	RequestHitRate      float64 `json:"request_hit_rate"`
	Status              string  `json:"status"`
}

// CacheStatusResponse is the admin dashboard cache status payload.
type CacheStatusResponse struct {
	Summary     CacheStatusSummary      `json:"summary"`
	Trend       []CacheStatusTrendPoint `json:"trend"`
	Models      []CacheStatusModelStat  `json:"models"`
	Window      string                  `json:"window"`
	Platform    string                  `json:"platform"`
	GeneratedAt string                  `json:"generated_at"`
}

// EndpointStat represents usage statistics for a single request endpoint.
type EndpointStat struct {
	Endpoint    string  `json:"endpoint"`
	Requests    int64   `json:"requests"`
	TotalTokens int64   `json:"total_tokens"`
	Cost        float64 `json:"cost"`        // 标准计费
	ActualCost  float64 `json:"actual_cost"` // 实际扣除
}

// GroupUsageSummary represents today's and cumulative cost for a single group.
type GroupUsageSummary struct {
	GroupID   int64   `json:"group_id"`
	TodayCost float64 `json:"today_cost"`
	TotalCost float64 `json:"total_cost"`
}

// GroupStat represents usage statistics for a single group
type GroupStat struct {
	GroupID     int64   `json:"group_id"`
	GroupName   string  `json:"group_name"`
	Requests    int64   `json:"requests"`
	TotalTokens int64   `json:"total_tokens"`
	Cost        float64 `json:"cost"`         // 标准计费
	ActualCost  float64 `json:"actual_cost"`  // 实际扣除
	AccountCost float64 `json:"account_cost"` // 账号成本
}

// UserUsageTrendPoint represents user usage trend data point
type UserUsageTrendPoint struct {
	Date       string  `json:"date"`
	UserID     int64   `json:"user_id"`
	Email      string  `json:"email"`
	Username   string  `json:"username"`
	Requests   int64   `json:"requests"`
	Tokens     int64   `json:"tokens"`
	Cost       float64 `json:"cost"`        // 标准计费
	ActualCost float64 `json:"actual_cost"` // 实际扣除
}

// UserSpendingRankingItem represents a user spending ranking row.
type UserSpendingRankingItem struct {
	UserID     int64   `json:"user_id"`
	Email      string  `json:"email"`
	ActualCost float64 `json:"actual_cost"` // 实际扣除
	Requests   int64   `json:"requests"`
	Tokens     int64   `json:"tokens"`
}

// UserSpendingRankingResponse represents ranking rows plus total spend for the time range.
type UserSpendingRankingResponse struct {
	Ranking         []UserSpendingRankingItem `json:"ranking"`
	TotalActualCost float64                   `json:"total_actual_cost"`
	TotalRequests   int64                     `json:"total_requests"`
	TotalTokens     int64                     `json:"total_tokens"`
}

// UserBreakdownItem represents per-user usage breakdown within a dimension (group, model, endpoint).
type UserBreakdownItem struct {
	UserID      int64   `json:"user_id"`
	Email       string  `json:"email"`
	Requests    int64   `json:"requests"`
	TotalTokens int64   `json:"total_tokens"`
	Cost        float64 `json:"cost"`         // 标准计费
	ActualCost  float64 `json:"actual_cost"`  // 实际扣除
	AccountCost float64 `json:"account_cost"` // 账号成本
}

// UserBreakdownDimension specifies the dimension to filter for user breakdown.
type UserBreakdownDimension struct {
	GroupID      int64  // filter by group_id (>0 to enable)
	Model        string // filter by model name (non-empty to enable)
	ModelType    string // "requested", "upstream", or "mapping"
	Endpoint     string // filter by endpoint value (non-empty to enable)
	EndpointType string // "inbound", "upstream", or "path"
	// Additional filter conditions
	UserID      int64  // filter by user_id (>0 to enable)
	APIKeyID    int64  // filter by api_key_id (>0 to enable)
	AccountID   int64  // filter by account_id (>0 to enable)
	RequestType *int16 // filter by request_type (non-nil to enable)
	Stream      *bool  // filter by stream flag (non-nil to enable)
	BillingType *int8  // filter by billing_type (non-nil to enable)
}

// APIKeyUsageTrendPoint represents API key usage trend data point
type APIKeyUsageTrendPoint struct {
	Date     string `json:"date"`
	APIKeyID int64  `json:"api_key_id"`
	KeyName  string `json:"key_name"`
	Requests int64  `json:"requests"`
	Tokens   int64  `json:"tokens"`
}

// UserDashboardStats 用户仪表盘统计
type UserDashboardStats struct {
	// API Key 统计
	TotalAPIKeys  int64 `json:"total_api_keys"`
	ActiveAPIKeys int64 `json:"active_api_keys"`

	// 累计 Token 使用统计
	TotalRequests            int64   `json:"total_requests"`
	TotalInputTokens         int64   `json:"total_input_tokens"`
	TotalOutputTokens        int64   `json:"total_output_tokens"`
	TotalCacheCreationTokens int64   `json:"total_cache_creation_tokens"`
	TotalCacheReadTokens     int64   `json:"total_cache_read_tokens"`
	TotalTokens              int64   `json:"total_tokens"`
	TotalCost                float64 `json:"total_cost"`        // 累计标准计费
	TotalActualCost          float64 `json:"total_actual_cost"` // 累计实际扣除

	// 今日 Token 使用统计
	TodayRequests            int64   `json:"today_requests"`
	TodayInputTokens         int64   `json:"today_input_tokens"`
	TodayOutputTokens        int64   `json:"today_output_tokens"`
	TodayCacheCreationTokens int64   `json:"today_cache_creation_tokens"`
	TodayCacheReadTokens     int64   `json:"today_cache_read_tokens"`
	TodayTokens              int64   `json:"today_tokens"`
	TodayCost                float64 `json:"today_cost"`        // 今日标准计费
	TodayActualCost          float64 `json:"today_actual_cost"` // 今日实际扣除

	// 性能统计
	AverageDurationMs float64 `json:"average_duration_ms"`

	// 性能指标
	Rpm int64 `json:"rpm"` // 近5分钟平均每分钟请求数
	Tpm int64 `json:"tpm"` // 近5分钟平均每分钟Token数
}

// UsageLogFilters represents filters for usage log queries
type UsageLogFilters struct {
	UserID      int64
	APIKeyID    int64
	AccountID   int64
	GroupID     int64
	Model       string
	RequestType *int16
	Stream      *bool
	BillingType *int8
	BillingMode string
	StartTime   *time.Time
	EndTime     *time.Time
	// ExactTotal requests exact COUNT(*) for pagination. Default false for fast large-table paging.
	ExactTotal bool
}

// UsageStats represents usage statistics
type UsageStats struct {
	TotalRequests     int64    `json:"total_requests"`
	TotalInputTokens  int64    `json:"total_input_tokens"`
	TotalOutputTokens int64    `json:"total_output_tokens"`
	TotalCacheTokens  int64    `json:"total_cache_tokens"`
	TotalTokens       int64    `json:"total_tokens"`
	TotalCost         float64  `json:"total_cost"`
	TotalActualCost   float64  `json:"total_actual_cost"`
	TotalAccountCost  *float64 `json:"total_account_cost,omitempty"`
	AverageDurationMs float64  `json:"average_duration_ms"`
	// 缓存命中统计：口径与 CacheStatusSummary 一致（读取率/创建率分母为 input+cache_read+cache_creation）。
	TotalCacheReadTokens     int64          `json:"total_cache_read_tokens"`
	TotalCacheCreationTokens int64          `json:"total_cache_creation_tokens"`
	CacheHitRequests         int64          `json:"cache_hit_requests"`
	CacheReadRate            float64        `json:"cache_read_rate"`
	CacheCreationRate        float64        `json:"cache_creation_rate"`
	RequestHitRate           float64        `json:"request_hit_rate"`
	Endpoints                []EndpointStat `json:"endpoints,omitempty"`
	UpstreamEndpoints        []EndpointStat `json:"upstream_endpoints,omitempty"`
	EndpointPaths            []EndpointStat `json:"endpoint_paths,omitempty"`
}

// DisplayAggregateGroup holds raw aggregate sums for one display-transform-invariant
// group of usage_logs rows. Rows are grouped by every field the user-facing display
// transform branches on (model, group, rate multiplier, long-context snapshot), so the
// transform can be applied once per group and summed — equivalent to transforming every
// row and summing, but at O(groups) instead of O(rows). Used to compute display-value
// statistics for unbounded ranges (e.g. the all-time dashboard totals) without loading
// every row into memory.
type DisplayAggregateGroup struct {
	Model                       string
	GroupID                     *int64
	RateMultiplier              float64
	LongContextApplied          bool
	LongContextInputMultiplier  *float64
	LongContextOutputMultiplier *float64

	Requests            int64
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	InputCost           float64
	OutputCost          float64
	CacheCreationCost   float64
	CacheReadCost       float64
	TotalCost           float64
	ActualCost          float64
	// DurationSum is SUM(COALESCE(duration_ms,0)); average is DurationSum/Requests.
	DurationSum int64
}

// BatchUserUsageStats represents usage stats for a single user
type BatchUserUsageStats struct {
	UserID          int64   `json:"user_id"`
	TodayActualCost float64 `json:"today_actual_cost"`
	TotalActualCost float64 `json:"total_actual_cost"`
}

// BatchAPIKeyUsageStats represents usage stats for a single API key
type BatchAPIKeyUsageStats struct {
	APIKeyID        int64   `json:"api_key_id"`
	TodayActualCost float64 `json:"today_actual_cost"`
	TotalActualCost float64 `json:"total_actual_cost"`
}

// AccountUsageHistory represents daily usage history for an account
type AccountUsageHistory struct {
	Date       string  `json:"date"`
	Label      string  `json:"label"`
	Requests   int64   `json:"requests"`
	Tokens     int64   `json:"tokens"`
	Cost       float64 `json:"cost"`        // 标准计费（total_cost）
	ActualCost float64 `json:"actual_cost"` // 账号口径费用（total_cost * account_rate_multiplier）
	UserCost   float64 `json:"user_cost"`   // 用户口径费用（actual_cost，受分组倍率影响）
}

// AccountUsageSummary represents summary statistics for an account
type AccountUsageSummary struct {
	Days              int     `json:"days"`
	ActualDaysUsed    int     `json:"actual_days_used"`
	TotalCost         float64 `json:"total_cost"`      // 账号口径费用
	TotalUserCost     float64 `json:"total_user_cost"` // 用户口径费用
	TotalStandardCost float64 `json:"total_standard_cost"`
	TotalRequests     int64   `json:"total_requests"`
	TotalTokens       int64   `json:"total_tokens"`
	AvgDailyCost      float64 `json:"avg_daily_cost"` // 账号口径日均
	AvgDailyUserCost  float64 `json:"avg_daily_user_cost"`
	AvgDailyRequests  float64 `json:"avg_daily_requests"`
	AvgDailyTokens    float64 `json:"avg_daily_tokens"`
	AvgDurationMs     float64 `json:"avg_duration_ms"`
	Today             *struct {
		Date     string  `json:"date"`
		Cost     float64 `json:"cost"`
		UserCost float64 `json:"user_cost"`
		Requests int64   `json:"requests"`
		Tokens   int64   `json:"tokens"`
	} `json:"today"`
	HighestCostDay *struct {
		Date     string  `json:"date"`
		Label    string  `json:"label"`
		Cost     float64 `json:"cost"`
		UserCost float64 `json:"user_cost"`
		Requests int64   `json:"requests"`
	} `json:"highest_cost_day"`
	HighestRequestDay *struct {
		Date     string  `json:"date"`
		Label    string  `json:"label"`
		Requests int64   `json:"requests"`
		Cost     float64 `json:"cost"`
		UserCost float64 `json:"user_cost"`
	} `json:"highest_request_day"`
}

// AccountUsageStatsResponse represents the full usage statistics response for an account
type AccountUsageStatsResponse struct {
	History           []AccountUsageHistory `json:"history"`
	Summary           AccountUsageSummary   `json:"summary"`
	Models            []ModelStat           `json:"models"`
	Endpoints         []EndpointStat        `json:"endpoints"`
	UpstreamEndpoints []EndpointStat        `json:"upstream_endpoints"`
}

// ============================================================================
// 成本分析：包月/日限订阅的成本与利润统计
// ============================================================================

// SubscriptionProfitRaw 是仓库层聚合出的单个订阅原始行（不含派生计算）。
// 进货成本与倍数等派生指标由 service 层根据进货单价计算。
type SubscriptionProfitRaw struct {
	SubscriptionID      int64
	UserID              int64
	UserEmail           string
	GroupID             int64
	GroupName           string
	PlanID              int64
	PlanName            string
	PlanPrice           float64 // 套餐标价（收入，人民币）
	HasPaidOrder        bool    // 是否找到可归因的已支付订阅订单
	AssignedBy          *int64
	Notes               string
	Status              string
	StartsAt            time.Time
	ExpiresAt           time.Time
	DailyLimitUSD       float64 // 日额度（刀，按 7/40/2 计）
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	RequestCount        int64
	ConsumedUSD         float64 // 消耗刀数（Σ actual_cost，按客户计价 7/40/2）
}

// SubscriptionProfitRow 是单个包月订阅的成本/利润统计行（含派生指标）。
type SubscriptionProfitRow struct {
	SubscriptionID int64   `json:"subscription_id"`
	UserID         int64   `json:"user_id"`
	UserEmail      string  `json:"user_email"`
	GroupID        int64   `json:"group_id"`
	GroupName      string  `json:"group_name"`
	PlanID         int64   `json:"plan_id"`
	PlanName       string  `json:"plan_name"`
	PlanPrice      float64 `json:"plan_price"` // 套餐标价（收入，人民币）
	Source         string  `json:"source"`     // paid | redeem | admin | default | system
	HasPaidOrder   bool    `json:"has_paid_order"`
	Status         string  `json:"status"`
	StartsAt       string  `json:"starts_at"`
	ExpiresAt      string  `json:"expires_at"`
	DailyLimitUSD  float64 `json:"daily_limit_usd"` // 日额度（刀）

	// 原始用量
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	RequestCount        int64   `json:"request_count"`
	ConsumedUSD         float64 `json:"consumed_usd"` // 消耗刀数（Σ actual_cost）

	// 派生指标
	CacheRate          float64 `json:"cache_rate"`           // 缓存 token 占比
	RealCostRMB        float64 `json:"real_cost_rmb"`        // 真实成本 = 总 token × 进货单价
	AvgPricePerDollar  float64 `json:"avg_price_per_dollar"` // 平均单价 元/刀 = 标价 ÷ 消耗刀数
	RealCostPerDollar  float64 `json:"real_cost_per_dollar"` // 真实成本 元/刀
	GrossProfitRMB     float64 `json:"gross_profit_rmb"`     // 毛利 = 标价 − 真实成本
	ProfitMultiple     float64 `json:"profit_multiple"`      // 倍数 = 标价 ÷ 真实成本（真实成本为 0 时为 0）
	EquivalentFullDays float64 `json:"equivalent_full_days"` // 等效满额天数 = 消耗刀数 ÷ 日额度
}

// SubscriptionProfitSummary 是全部订阅的汇总指标。
type SubscriptionProfitSummary struct {
	SubscriptionCount   int64   `json:"subscription_count"`
	TotalRevenueRMB     float64 `json:"total_revenue_rmb"`
	TotalRealCostRMB    float64 `json:"total_real_cost_rmb"`
	TotalGrossProfitRMB float64 `json:"total_gross_profit_rmb"`
	TotalConsumedUSD    float64 `json:"total_consumed_usd"`
	AvgProfitMultiple   float64 `json:"avg_profit_multiple"` // 整体 = 总收入 ÷ 总成本
	LossCount           int64   `json:"loss_count"`          // 倍数 < 1（亏损）
	BelowTwoCount       int64   `json:"below_two_count"`     // 倍数 < 2（薄利）
	CostMode            string  `json:"cost_mode"`           // per_mtok（元/百万token）| per_dollar（元/刀）
	PurchasePrice       float64 `json:"purchase_price"`      // 回显进货单价（单位随 cost_mode）
}

// SubscriptionPlanProfit 是按套餐分组的汇总。
type SubscriptionPlanProfit struct {
	PlanID                int64   `json:"plan_id"`
	PlanName              string  `json:"plan_name"`
	PlanPrice             float64 `json:"plan_price"`
	Count                 int64   `json:"count"`
	TotalRevenueRMB       float64 `json:"total_revenue_rmb"`
	TotalRealCostRMB      float64 `json:"total_real_cost_rmb"`
	AvgProfitMultiple     float64 `json:"avg_profit_multiple"`
	AvgEquivalentFullDays float64 `json:"avg_equivalent_full_days"`
	AvgCacheRate          float64 `json:"avg_cache_rate"`
}

// SubscriptionProfitResponse 是包月成本/利润接口的完整响应。
type SubscriptionProfitResponse struct {
	Summary SubscriptionProfitSummary `json:"summary"`
	ByPlan  []SubscriptionPlanProfit  `json:"by_plan"`
	Rows    []SubscriptionProfitRow   `json:"rows"`
}
