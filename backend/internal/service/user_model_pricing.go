package service

import (
	"context"
	"time"
)

// UserModelPricingOverride 用户级模型定价覆盖实体。
// 管理员可通过此配置为特定用户的特定模型设定专属价格，覆盖全局和渠道定价。
//
// 叠加优先级：User > Channel > Global > LiteLLM/Fallback
// 应用时机：ModelPricingResolver.Resolve 在 Channel 覆盖之后叠加此覆盖。
type UserModelPricingOverride struct {
	ID     int64
	UserID int64
	Model  string

	// 真实计费覆盖（非 nil 字段替换上层链路值）
	InputPrice      *float64
	OutputPrice     *float64
	CacheWritePrice *float64
	CacheReadPrice  *float64

	// 展示覆盖（仅影响用户看到的 usage log，不影响真实计费）
	DisplayInputPrice     *float64
	DisplayOutputPrice    *float64
	DisplayRateMultiplier *float64
	CacheTransferRatio    *float64

	Enabled bool
	Notes   string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserModelPricingRepository 用户模型定价覆盖数据访问接口
type UserModelPricingRepository interface {
	GetByUserID(ctx context.Context, userID int64) ([]UserModelPricingOverride, error)
	GetEnabledByUserID(ctx context.Context, userID int64) ([]UserModelPricingOverride, error)
	GetByUserAndModel(ctx context.Context, userID int64, model string) (*UserModelPricingOverride, error)
	GetByID(ctx context.Context, id int64) (*UserModelPricingOverride, error)
	Create(ctx context.Context, o *UserModelPricingOverride) error
	Update(ctx context.Context, o *UserModelPricingOverride) error
	Delete(ctx context.Context, id int64) error
	DeleteByUserID(ctx context.Context, userID int64) error
	BatchUpsert(ctx context.Context, userID int64, overrides []UserModelPricingOverride) error

	// 聚合查询（供模型定价列表展示用）
	GetEnabledCountByModel(ctx context.Context) (map[string]int, error)
	GetByModel(ctx context.Context, model string) ([]UserModelPricingOverride, error)
}
