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
  cache_read_price: number | null
  image_output_price: number | null
  per_request_price: number | null
  enabled: boolean
  notes: string
  display_input_price: number | null
  display_output_price: number | null
  display_cache_read_price: number | null
  display_rate_multiplier: number | null
  cache_transfer_ratio: number | null
}

export interface BillingBasisHint {
  /**
   * 模型名在平台级模型映射里扮演的角色：
   * - requested_equals_upstream: 同名映射（请求名 == 上游名）
   * - upstream_only: 只是上游名，related_models 列出所有映射源请求名（可能多对一）
   * - requested_only: 只是请求名，related_models 单元素为映射目标上游名
   */
  type: 'requested_equals_upstream' | 'upstream_only' | 'requested_only'
  related_models?: string[]
}

export interface ModelPricingItem {
  model: string
  provider: string
  litellm_prices: LiteLLMPrices | null
  global_override: GlobalOverride | null
  channel_override_count: number
  effective_source: 'channel' | 'global' | 'litellm' | 'fallback'
  billing_basis_hint?: BillingBasisHint | null
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
  cache_read_price?: number | null
  image_output_price?: number | null
  per_request_price?: number | null
  enabled?: boolean
  notes?: string
  display_input_price?: number | null
  display_output_price?: number | null
  display_cache_read_price?: number | null
  display_rate_multiplier?: number | null
  cache_transfer_ratio?: number | null
}

export interface UpdateOverrideRequest {
  model?: string
  provider?: string
  billing_mode?: string
  input_price?: number | null
  output_price?: number | null
  cache_write_price?: number | null
  cache_read_price?: number | null
  image_output_price?: number | null
  per_request_price?: number | null
  enabled?: boolean
  notes?: string
  display_input_price?: number | null
  display_output_price?: number | null
  display_cache_read_price?: number | null
  display_rate_multiplier?: number | null
  cache_transfer_ratio?: number | null
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

const modelPricingAPI = {
  list,
  getDetail,
  createOverride,
  updateOverride,
  deleteOverride,
  getChannelOverrides,
  getRateMultiplierOverview,
}

export default modelPricingAPI
