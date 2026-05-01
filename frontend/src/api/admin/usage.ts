/**
 * Admin Usage API endpoints
 * Handles admin-level usage logs and statistics retrieval
 */

import { apiClient } from '../client'
import type { AdminUsageLog, UsageQueryParams, PaginatedResponse, UsageRequestType } from '@/types'
import type { EndpointStat } from '@/types'

// ==================== Types ====================

export interface AdminUsageStatsResponse {
  total_requests: number
  total_input_tokens: number
  total_output_tokens: number
  total_cache_tokens: number
  total_tokens: number
  total_cost: number
  total_actual_cost: number
  total_account_cost: number
  average_duration_ms: number
  endpoints?: EndpointStat[]
  upstream_endpoints?: EndpointStat[]
  endpoint_paths?: EndpointStat[]
}

export interface SimpleUser {
  id: number
  email: string
}

export interface SimpleApiKey {
  id: number
  name: string
  user_id: number
}

export interface UsageCleanupFilters {
  start_time: string
  end_time: string
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  model?: string | null
  request_type?: UsageRequestType | null
  stream?: boolean | null
  billing_type?: number | null
}

export interface UsageCleanupTask {
  id: number
  status: string
  filters: UsageCleanupFilters
  created_by: number
  deleted_rows: number
  error_message?: string | null
  canceled_by?: number | null
  canceled_at?: string | null
  started_at?: string | null
  finished_at?: string | null
  created_at: string
  updated_at: string
}

export interface CreateUsageCleanupTaskRequest {
  start_date: string
  end_date: string
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  model?: string | null
  request_type?: UsageRequestType | null
  stream?: boolean | null
  billing_type?: number | null
  timezone?: string
}

export interface AdminUsageQueryParams extends UsageQueryParams {
  user_id?: number
  exact_total?: boolean
  billing_mode?: string
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}

// ==================== API Functions ====================

/**
 * List all usage logs with optional filters (admin only)
 * @param params - Query parameters for filtering and pagination
 * @returns Paginated list of usage logs
 */
export async function list(
  params: AdminUsageQueryParams,
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<AdminUsageLog>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminUsageLog>>('/admin/usage', {
    params,
    signal: options?.signal
  })
  return data
}

/**
 * Get usage statistics with optional filters (admin only)
 * @param params - Query parameters for filtering
 * @returns Usage statistics
 */
export async function getStats(params: {
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  model?: string
  request_type?: UsageRequestType
  stream?: boolean
  period?: string
  start_date?: string
  end_date?: string
  timezone?: string
}): Promise<AdminUsageStatsResponse> {
  const { data } = await apiClient.get<AdminUsageStatsResponse>('/admin/usage/stats', {
    params
  })
  return data
}

/**
 * Search users by email keyword (admin only)
 * @param keyword - Email keyword to search
 * @returns List of matching users (max 30)
 */
export async function searchUsers(keyword: string): Promise<SimpleUser[]> {
  const { data } = await apiClient.get<SimpleUser[]>('/admin/usage/search-users', {
    params: { q: keyword }
  })
  return data
}

/**
 * Search API keys by user ID and/or keyword (admin only)
 * @param userId - Optional user ID to filter by
 * @param keyword - Optional keyword to search in key name
 * @returns List of matching API keys (max 30)
 */
export async function searchApiKeys(userId?: number, keyword?: string): Promise<SimpleApiKey[]> {
  const params: Record<string, unknown> = {}
  if (userId !== undefined) {
    params.user_id = userId
  }
  if (keyword) {
    params.q = keyword
  }
  const { data } = await apiClient.get<SimpleApiKey[]>('/admin/usage/search-api-keys', {
    params
  })
  return data
}

/**
 * List usage cleanup tasks (admin only)
 * @param params - Query parameters for pagination
 * @returns Paginated list of cleanup tasks
 */
export async function listCleanupTasks(
  params: { page?: number; page_size?: number },
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<UsageCleanupTask>> {
  const { data } = await apiClient.get<PaginatedResponse<UsageCleanupTask>>('/admin/usage/cleanup-tasks', {
    params,
    signal: options?.signal
  })
  return data
}

/**
 * Create a usage cleanup task (admin only)
 * @param payload - Cleanup task parameters
 * @returns Created cleanup task
 */
export async function createCleanupTask(payload: CreateUsageCleanupTaskRequest): Promise<UsageCleanupTask> {
  const { data } = await apiClient.post<UsageCleanupTask>('/admin/usage/cleanup-tasks', payload)
  return data
}

/**
 * Cancel a usage cleanup task (admin only)
 * @param taskId - Task ID to cancel
 */
export async function cancelCleanupTask(taskId: number): Promise<{ id: number; status: string }> {
  const { data } = await apiClient.post<{ id: number; status: string }>(
    `/admin/usage/cleanup-tasks/${taskId}/cancel`
  )
  return data
}

// ==================== Antigravity credit ratio ====================

export interface AntigravityUsageRatio {
  start: string
  end: string
  credits_consumed: number
  credits_by_type: Record<string, number>
  quota_used_usd: number
  call_count: number
  quota_per_credit?: number
  calls_per_credit?: number
  snapshot_count: number
  emails_sampled: number
  manual_refresh_throttled?: boolean
}

interface AntigravityStatsParams {
  period?: string
  start_date?: string
  end_date?: string
  timezone?: string
}

export async function getAntigravityStats(
  params: AntigravityStatsParams,
  options?: { signal?: AbortSignal }
): Promise<AntigravityUsageRatio> {
  const { data } = await apiClient.get<AntigravityUsageRatio>('/admin/usage/stats/antigravity', {
    params,
    signal: options?.signal
  })
  return data
}

export async function refreshAntigravityStats(
  params: AntigravityStatsParams
): Promise<AntigravityUsageRatio> {
  const { data } = await apiClient.post<AntigravityUsageRatio>(
    '/admin/usage/stats/antigravity/refresh',
    null,
    { params }
  )
  return data
}

// ==================== User-view preview (display transform debug) ====================

export interface UserViewSnapshot {
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_creation_tokens: number
  input_cost: number
  output_cost: number
  cache_read_cost: number
  cache_creation_cost: number
  total_cost: number
  actual_cost: number
  rate_multiplier: number
}

export interface UserViewConfigUsed {
  display_input_price: number | null
  display_output_price: number | null
  display_cache_read_price: number | null
  display_rate_multiplier: number | null
  cache_transfer_ratio: number | null
  user_group_rate: number | null
  has_user_override: boolean
  group_id: number | null
}

export interface UserViewPreview {
  log_id: number
  user_id: number
  model: string
  real: UserViewSnapshot
  user_view: UserViewSnapshot
  config_used: UserViewConfigUsed
}

/**
 * Fetch a side-by-side preview of "what the owning user sees on their own /usage page" vs the
 * admin's raw view, for a single usage_log row. Used by the admin compare drawer to verify
 * cache_transfer_ratio + display pricing produce the expected numbers without logging in as the user.
 */
export async function getUserViewPreview(logId: number): Promise<UserViewPreview> {
  const { data } = await apiClient.get<UserViewPreview>(`/admin/usage/${logId}/user-view`)
  return data
}

export const adminUsageAPI = {
  list,
  getStats,
  searchUsers,
  searchApiKeys,
  listCleanupTasks,
  createCleanupTask,
  cancelCleanupTask,
  getAntigravityStats,
  refreshAntigravityStats,
  getUserViewPreview
}

export default adminUsageAPI
