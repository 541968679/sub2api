package service

import (
	"context"
	"log/slog"
	"strings"
	"sync"
)

// GlobalPricingCache 为 GlobalModelPricingRepository 提供内存缓存。
// 惰性加载：首次访问时一次性读取所有启用的覆盖条目到内存，后续访问 O(1)。
// 管理后台在 Create/Update/Delete 全局覆盖后必须调用 Invalidate。
//
// 之所以整表缓存而不是 per-model LRU：全局覆盖是管理员手动配置的，
// 数量不多（远小于 channel 定价表），且热路径每个 AI 请求都会查询，
// 用整表 map 查询是最省事且 O(1) 的方案。
type GlobalPricingCache struct {
	repo GlobalModelPricingRepository

	mu      sync.RWMutex
	loaded  bool
	byModel map[string]*GlobalModelPricing
}

// NewGlobalPricingCache 创建缓存实例（惰性加载，构造时不访问数据库）
func NewGlobalPricingCache(repo GlobalModelPricingRepository) *GlobalPricingCache {
	return &GlobalPricingCache{
		repo:    repo,
		byModel: make(map[string]*GlobalModelPricing),
	}
}

// Get 返回指定模型的已启用全局覆盖，未配置返回 nil。
// 模型名大小写不敏感匹配。返回指针指向缓存中的副本——调用方不得修改。
// 对 nil 接收者安全（便于测试用 nil 缓存禁用该功能）。
func (c *GlobalPricingCache) Get(model string) *GlobalModelPricing {
	if c == nil {
		return nil
	}
	key := strings.ToLower(model)

	c.mu.RLock()
	if c.loaded {
		p := c.byModel[key]
		c.mu.RUnlock()
		return p
	}
	c.mu.RUnlock()

	return c.loadAndGet(key)
}

// loadAndGet 在缓存未加载时触发一次全量加载，双检锁避免重复加载。
func (c *GlobalPricingCache) loadAndGet(key string) *GlobalModelPricing {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded {
		return c.byModel[key]
	}

	list, err := c.repo.GetAllEnabled(context.Background())
	if err != nil {
		// 加载失败：不标记 loaded，下次再试；返回 nil 让调用方 fallback 到 LiteLLM
		slog.Warn("global pricing cache load failed", "error", err)
		return nil
	}

	byModel := make(map[string]*GlobalModelPricing, len(list))
	for i := range list {
		entry := list[i]
		byModel[strings.ToLower(entry.Model)] = &entry
	}
	c.byModel = byModel
	c.loaded = true
	return c.byModel[key]
}

// Invalidate 清空缓存，使下次 Get 触发重新加载。
// GlobalModelPricingService 的 Create/Update/Delete 必须调用此方法。
func (c *GlobalPricingCache) Invalidate() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.loaded = false
	c.byModel = make(map[string]*GlobalModelPricing)
	c.mu.Unlock()
}
