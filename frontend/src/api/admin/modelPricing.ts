/**
 * Admin Model Pricing API endpoints
 * Handles global model pricing management for administrators
 */

import { apiClient } from '../client'

// --- Types ---

export interface LiteLLMPrices {
  input_price: number
  output_price: number
  cache_write_price: number
  cache_write_1h_price: number
  cache_read_price: number
  image_output_price: number
}

export interface GlobalOverride {
  id: number
  model: string
  provider: string
  billing_mode: string
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price: number | null
  cache_read_price: number | null
  image_output_price: number | null
  per_request_price: number | null
  image_price_1k: number | null
  image_price_2k: number | null
  image_price_4k: number | null
  image_billing_strategy: 'tier' | 'megapixel'
  image_megapixel_price: number | null
  image_quality_prices?: Record<string, number> | null
  image_quality_multipliers?: Record<string, number> | null
  image_tier_rules?: ImageTierRule[] | null
  enabled: boolean
  notes: string
  display_input_price: number | null
  display_output_price: number | null
  display_cache_read_price: number | null
  display_cache_creation_price: number | null
  display_cache_creation_1h_price: number | null
  display_rate_multiplier: number | null
  show_on_pricing_page: boolean
}

export interface ImageTierRule {
  tier_label: string
  max_pixels?: number | null
  price?: number | null
}

export interface BillingBasisHint {
  /**
   * 模型名在某个平台级模型映射里扮演的角色。一个模型可以同时是映射键
   * （mapping_target 非空）和其他键的映射目标（mapped_from 非空）。
   * type 是给旧展示的汇总标签：
   * - requested_equals_upstream: 自身有同名映射条目（请求名 == 上游名）
   * - requested_only: 自身有映射条目且指向别的上游名
   * - upstream_only: 自身没有映射条目，只作为其他请求名的映射目标出现
   */
  platform?: string
  type: 'requested_equals_upstream' | 'upstream_only' | 'requested_only'
  related_models?: string[]
  mapping_key?: string
  /** 自身作为映射键时的映射目标（可能等于自身，即同名映射） */
  mapping_target?: string
  /** 映射到此模型的其他请求名（不含自身） */
  mapped_from?: string[]
  mapping_billing_objects?: Record<string, 'requested' | 'mapped'>
  billing_object?: 'requested' | 'mapped'
  billing_object_editable?: boolean
  mapping_editable?: boolean
}

export interface ModelPricingItem {
  model: string
  provider: string
  litellm_prices: LiteLLMPrices | null
  global_override: GlobalOverride | null
  channel_override_count: number
  effective_source: 'channel' | 'global' | 'litellm' | 'fallback'
  /** 主 hint（provider 匹配或平台顺序第一个），兼容旧逻辑 */
  billing_basis_hint?: BillingBasisHint | null
  /** 此模型在每个平台默认映射中的完整角色（provider 筛选时只有该平台一条） */
  billing_basis_hints?: BillingBasisHint[] | null
}

export interface ModelPricingStats {
  total_models: number
  global_override_count: number
  channel_override_count: number
}

export interface ModelPricingListResult {
  items: ModelPricingItem[]
  pagination: {
    total: number
    page: number
    page_size: number
    pages: number
  }
  stats: ModelPricingStats
}

export interface ModelPricingDetail {
  model: string
  provider: string
  litellm_prices: LiteLLMPrices | null
  global_override: GlobalOverride | null
  channel_overrides: ChannelOverrideSummary[]
  /** 建议价：当 LiteLLM 和 global_override 都不存在时，后端按命名近似推断的参考 */
  suggested_prices?: LiteLLMPrices | null
  /** 建议价来源模型名（用于前端展示"来自 xxx"） */
  suggested_from?: string
}

export interface ChannelOverrideSummary {
  channel_id: number
  channel_name: string
  platform: string
  billing_mode: string
  input_price: number | null
  output_price: number | null
}

export interface CreateOverrideRequest {
  model: string
  provider?: string
  billing_mode?: string
  input_price?: number | null
  output_price?: number | null
  cache_write_price?: number | null
  cache_write_1h_price?: number | null
  cache_read_price?: number | null
  image_output_price?: number | null
  per_request_price?: number | null
  image_price_1k?: number | null
  image_price_2k?: number | null
  image_price_4k?: number | null
  image_billing_strategy?: 'tier' | 'megapixel'
  image_megapixel_price?: number | null
  image_quality_prices?: Record<string, number> | null
  image_quality_multipliers?: Record<string, number> | null
  image_tier_rules?: ImageTierRule[] | null
  enabled?: boolean
  notes?: string
  display_input_price?: number | null
  display_output_price?: number | null
  display_cache_read_price?: number | null
  display_cache_creation_price?: number | null
  display_cache_creation_1h_price?: number | null
  display_rate_multiplier?: number | null
  show_on_pricing_page?: boolean
}

export interface UpdateOverrideRequest {
  model?: string
  provider?: string
  billing_mode?: string
  input_price?: number | null
  output_price?: number | null
  cache_write_price?: number | null
  cache_write_1h_price?: number | null
  cache_read_price?: number | null
  image_output_price?: number | null
  per_request_price?: number | null
  image_price_1k?: number | null
  image_price_2k?: number | null
  image_price_4k?: number | null
  image_billing_strategy?: 'tier' | 'megapixel'
  image_megapixel_price?: number | null
  image_quality_prices?: Record<string, number> | null
  image_quality_multipliers?: Record<string, number> | null
  image_tier_rules?: ImageTierRule[] | null
  enabled?: boolean
  notes?: string
  display_input_price?: number | null
  display_output_price?: number | null
  display_cache_read_price?: number | null
  display_cache_creation_price?: number | null
  display_cache_creation_1h_price?: number | null
  display_rate_multiplier?: number | null
  show_on_pricing_page?: boolean
}

export interface RateMultiplierSummary {
  group_id: number
  group_name: string
  rate_multiplier: number
}

// --- API functions ---

/**
 * List all models with pricing info (merged LiteLLM + global overrides)
 */
export async function list(
  page: number = 1,
  pageSize: number = 50,
  filters?: {
    search?: string
    provider?: string
    source?: string
  },
  options?: { signal?: AbortSignal }
): Promise<ModelPricingListResult> {
  const { data } = await apiClient.get<ModelPricingListResult>('/admin/model-pricing', {
    params: {
      page,
      page_size: pageSize,
      ...filters,
    },
    signal: options?.signal,
  })
  return data
}

/**
 * Get single model pricing detail (all sources)
 */
export async function getDetail(model: string): Promise<ModelPricingDetail> {
  const { data } = await apiClient.get<ModelPricingDetail>(
    `/admin/model-pricing/${encodeURIComponent(model)}`
  )
  return data
}

/**
 * Create a global pricing override
 */
export async function createOverride(req: CreateOverrideRequest): Promise<GlobalOverride> {
  const { data } = await apiClient.post<GlobalOverride>('/admin/model-pricing', req)
  return data
}

/**
 * Update a global pricing override
 */
export async function updateOverride(id: number, req: UpdateOverrideRequest): Promise<GlobalOverride> {
  const { data } = await apiClient.put<GlobalOverride>(`/admin/model-pricing/${id}`, req)
  return data
}

/**
 * Delete a global pricing override
 */
export async function deleteOverride(id: number): Promise<void> {
  await apiClient.delete(`/admin/model-pricing/${id}`)
}

/**
 * Get channels that override a specific model
 */
export async function getChannelOverrides(model: string): Promise<ChannelOverrideSummary[]> {
  const { data } = await apiClient.get<ChannelOverrideSummary[]>(
    `/admin/model-pricing/${encodeURIComponent(model)}/channels`
  )
  return data
}

/**
 * Get rate multiplier overview for all active groups
 */
export async function getRateMultiplierOverview(): Promise<RateMultiplierSummary[]> {
  const { data } = await apiClient.get<RateMultiplierSummary[]>('/admin/model-pricing/rate-multipliers')
  return data
}

/**
 * 获取模型配置页删除（隐藏）的模型列表（小写模型名）
 */
export async function getHiddenModels(): Promise<string[]> {
  const { data } = await apiClient.get<string[]>('/admin/model-pricing/hidden-models')
  return data
}

/**
 * 覆盖保存隐藏模型列表，返回清洗后的结果
 */
export async function updateHiddenModels(models: string[]): Promise<string[]> {
  const { data } = await apiClient.put<string[]>('/admin/model-pricing/hidden-models', models)
  return data
}

const modelPricingAPI = {
  list,
  getDetail,
  createOverride,
  updateOverride,
  deleteOverride,
  getChannelOverrides,
  getRateMultiplierOverview,
  getHiddenModels,
  updateHiddenModels,
}

export default modelPricingAPI
