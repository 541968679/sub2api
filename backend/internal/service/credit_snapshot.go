package service

import (
	"context"
	"time"
)

// CreditSnapshot 是 Antigravity AI Credits 的一个历史余额采样点。
type CreditSnapshot struct {
	ID         int64
	Email      string
	CreditType string
	Amount     float64
	CapturedAt time.Time
}

// CreditSnapshotRepository 抽象快照表的存取。实现位于 repository 包。
type CreditSnapshotRepository interface {
	Insert(ctx context.Context, snap *CreditSnapshot) error
	// ListInRange 返回 [start, end] 内的全部快照，按 email 分组、时间升序。
	ListInRange(ctx context.Context, start, end time.Time) (map[string][]CreditSnapshot, error)
	// GetLatestBefore 返回 email 在 t 之前最近一条快照，没有则返回 nil。
	GetLatestBefore(ctx context.Context, email string, t time.Time) (*CreditSnapshot, error)
}

// AntigravityUsageAggregator 聚合 usage_logs 中 antigravity 平台账号在时间窗内的
// 调用次数和总费用。定义为独立小接口以避免改动 UsageLogRepository 的多处 stub。
type AntigravityUsageAggregator interface {
	AggregateUsage(ctx context.Context, accountIDs []int64, start, end time.Time) (callCount int64, totalCost float64, err error)
}

// AntigravityUsageRatio 是"每 credit 对应多少额度/调用次数"的聚合结果。
type AntigravityUsageRatio struct {
	Start                  time.Time          `json:"start"`
	End                    time.Time          `json:"end"`
	CreditsConsumed        float64            `json:"credits_consumed"`
	CreditsByType          map[string]float64 `json:"credits_by_type"`
	QuotaUsedUSD           float64            `json:"quota_used_usd"`
	CallCount              int64              `json:"call_count"`
	QuotaPerCredit         *float64           `json:"quota_per_credit,omitempty"`
	CallsPerCredit         *float64           `json:"calls_per_credit,omitempty"`
	SnapshotCount          int                `json:"snapshot_count"`
	EmailsSampled          int                `json:"emails_sampled"`
	ManualRefreshThrottled bool               `json:"manual_refresh_throttled,omitempty"`
}
