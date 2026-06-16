/**
 * Admin Cost Analysis API endpoints.
 *
 * Cost-side metrics for the relay station. The first module covers per-subscription
 * profit for monthly / daily-limited users:
 *   real cost (RMB) = total tokens × purchase price (RMB per million tokens)
 *   revenue        = paid plan list price, or 0 for redeem/admin/default grants
 *   all matching subscriptions are counted and tagged with their source.
 */

import { apiClient } from '../client'

export interface SubscriptionProfitRow {
  subscription_id: number
  user_id: number
  user_email: string
  group_id: number
  group_name: string
  plan_id: number
  plan_name: string
  plan_price: number
  source: 'paid' | 'redeem' | 'admin' | 'default' | 'system' | string
  has_paid_order: boolean
  status: string
  starts_at: string
  expires_at: string
  daily_limit_usd: number
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  total_tokens: number
  request_count: number
  consumed_usd: number
  cache_rate: number
  real_cost_rmb: number
  avg_price_per_dollar: number
  real_cost_per_dollar: number
  gross_profit_rmb: number
  profit_multiple: number
  equivalent_full_days: number
}

export interface SubscriptionProfitSummary {
  subscription_count: number
  total_revenue_rmb: number
  total_real_cost_rmb: number
  total_gross_profit_rmb: number
  total_consumed_usd: number
  avg_profit_multiple: number
  loss_count: number
  below_two_count: number
  cost_mode: string
  purchase_price: number
}

export interface SubscriptionPlanProfit {
  plan_id: number
  plan_name: string
  plan_price: number
  count: number
  total_revenue_rmb: number
  total_real_cost_rmb: number
  avg_profit_multiple: number
  avg_equivalent_full_days: number
  avg_cache_rate: number
}

export interface SubscriptionProfitResponse {
  summary: SubscriptionProfitSummary
  by_plan: SubscriptionPlanProfit[]
  rows: SubscriptionProfitRow[]
}

export type CostMode = 'per_mtok' | 'per_dollar'

export interface SubscriptionProfitParams {
  active_only?: boolean
  start_date?: string
  end_date?: string
  cost_mode?: CostMode
  purchase_price?: number
}

/**
 * Get per-subscription cost/profit statistics for monthly (daily-limited) users.
 */
export async function getSubscriptionProfit(
  params?: SubscriptionProfitParams
): Promise<SubscriptionProfitResponse> {
  const { data } = await apiClient.get<SubscriptionProfitResponse>(
    '/admin/dashboard/subscription-profit',
    { params }
  )
  return data
}
