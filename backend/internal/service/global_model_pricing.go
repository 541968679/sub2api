package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// GlobalModelPricing 全局模型定价覆盖实体
// 管理员可通过此配置为特定模型设定平台级自定义价格，覆盖 LiteLLM 默认值。
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
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ToModelPricing 将全局覆盖转换为计费系统使用的 ModelPricing 结构。
// 仅设置非 nil 的价格字段。
func (g *GlobalModelPricing) ToModelPricing() *ModelPricing {
	mp := &ModelPricing{}
	if g.InputPrice != nil {
		mp.InputPricePerToken = *g.InputPrice
	}
	if g.OutputPrice != nil {
		mp.OutputPricePerToken = *g.OutputPrice
	}
	if g.CacheWritePrice != nil {
		mp.CacheCreationPricePerToken = *g.CacheWritePrice
	}
	if g.CacheReadPrice != nil {
		mp.CacheReadPricePerToken = *g.CacheReadPrice
	}
	if g.ImageOutputPrice != nil {
		mp.ImageOutputPricePerToken = *g.ImageOutputPrice
	}
	return mp
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
