package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// GlobalModelPricing 全局模型定价覆盖实体
// 管理员可通过此配置为特定模型设定平台级自定义价格，覆盖 LiteLLM 默认值。
//
// 应用时机：ModelPricingResolver.resolveBasePricing 在 LiteLLM/fallback 基础
// 定价上叠加此覆盖（仅替换非 nil 字段），以保留 Priority tier、长上下文倍率、
// 缓存 5m/1h 分级等关键字段。实现见 model_pricing_resolver.go:applyGlobalPricingOverride。
type GlobalModelPricing struct {
	ID               int64
	Model            string      // 模型名称（唯一，大小写不敏感）
	Provider         string      // 平台标识（anthropic/openai/gemini/antigravity）
	BillingMode      BillingMode // 计费模式
	InputPrice       *float64    // 每 token 输入价格（USD）
	OutputPrice      *float64    // 每 token 输出价格（USD）
	CacheWritePrice  *float64    // 缓存写入价格
	CacheReadPrice   *float64    // 缓存读取价格
	ImageOutputPrice *float64    // 图片输出价格
	PerRequestPrice  *float64    // 按次计费价格
	Enabled          bool        // 是否启用
	Notes            string      // 管理员备注

	// Display overrides — only affect user-facing usage log display, not actual billing.
	DisplayInputPrice     *float64 // 展示给用户的输入单价
	DisplayOutputPrice    *float64 // 展示给用户的输出单价
	DisplayCacheReadPrice *float64 // 展示给用户的缓存读取单价
	DisplayRateMultiplier *float64 // 展示给用户的倍率
	CacheTransferRatio    *float64 // 缓存 token 转移到输入 token 的比例 (0~1)

	CreatedAt time.Time
	UpdatedAt time.Time
}

// GlobalModelPricingRepository 全局模型定价数据访问接口
type GlobalModelPricingRepository interface {
	List(ctx context.Context, params pagination.PaginationParams, search, provider string) ([]GlobalModelPricing, *pagination.PaginationResult, error)
	GetByID(ctx context.Context, id int64) (*GlobalModelPricing, error)
	GetByModel(ctx context.Context, model string) (*GlobalModelPricing, error)
	Create(ctx context.Context, pricing *GlobalModelPricing) error
	Update(ctx context.Context, pricing *GlobalModelPricing) error
	Delete(ctx context.Context, id int64) error
	GetAllEnabled(ctx context.Context) ([]GlobalModelPricing, error)
}
