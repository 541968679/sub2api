/**
 * Admin Users API endpoints
 * Handles user management for administrators
 */

import { apiClient } from '../client'
import type { AdminUser, UpdateUserRequest, PaginatedResponse, ApiKey, DownstreamUsageTokenMode, UserStatus } from '@/types'
import type {
  AccountQualityHistoryResponse,
  BatchQualityStatsResponse
} from './accounts'
import { normalizeSmartSchedulePairQuality } from '@/utils/smartScheduleWindowN'

export interface AdminBindAuthIdentityChannelRequest {
  channel: string
  channel_app_id: string
  channel_subject: string
  metadata?: Record<string, unknown> | null
}

export interface AdminBindAuthIdentityRequest {
  provider_type: string
  provider_key: string
  provider_subject: string
  issuer?: string | null
  metadata?: Record<string, unknown> | null
  channel?: AdminBindAuthIdentityChannelRequest
}

export interface AdminBoundAuthIdentityChannel {
  channel: string
  channel_app_id: string
  channel_subject: string
  metadata: Record<string, unknown> | null
  created_at: string
  updated_at: string
}

export interface AdminBoundAuthIdentity {
  user_id: number
  provider_type: string
  provider_key: string
  provider_subject: string
  verified_at?: string | null
  issuer?: string | null
  metadata: Record<string, unknown> | null
  created_at: string
  updated_at: string
  channel?: AdminBoundAuthIdentityChannel | null
}

/**
 * List all users with pagination
 * @param page - Page number (default: 1)
 * @param pageSize - Items per page (default: 20)
 * @param filters - Optional filters (status, role, search, attributes)
 * @param options - Optional request options (signal)
 * @returns Paginated list of users
 */
export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    status?: UserStatus
    role?: 'admin' | 'user'
    search?: string
    group_name?: string         // fuzzy filter by allowed group name
    attributes?: Record<number, string>  // attributeId -> value
    include_subscriptions?: boolean
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  },
  options?: {
    signal?: AbortSignal
  }
): Promise<PaginatedResponse<AdminUser>> {
  // Build params with attribute filters in attr[id]=value format
  const params: Record<string, any> = {
    page,
    page_size: pageSize,
    status: filters?.status,
    role: filters?.role,
    search: filters?.search,
    group_name: filters?.group_name,
    include_subscriptions: filters?.include_subscriptions,
    sort_by: filters?.sort_by,
    sort_order: filters?.sort_order
  }

  // Add attribute filters as attr[id]=value
  if (filters?.attributes) {
    for (const [attrId, value] of Object.entries(filters.attributes)) {
      if (value) {
        params[`attr[${attrId}]`] = value
      }
    }
  }
  const { data } = await apiClient.get<PaginatedResponse<AdminUser>>('/admin/users', {
    params,
    signal: options?.signal
  })
  return data
}

/**
 * Get user by ID
 * @param id - User ID
 * @param includeDeleted - Whether to include soft-deleted users
 * @returns User details
 */
export async function getById(id: number, includeDeleted = false): Promise<AdminUser> {
  const url = includeDeleted ? `/admin/users/${id}?include_deleted=true` : `/admin/users/${id}`
  const { data } = await apiClient.get<AdminUser>(url)
  return data
}

/**
 * Create new user
 * @param userData - User data (email, password, etc.)
 * @returns Created user
 */
export async function create(userData: {
  email: string
  password: string
	role?: 'admin' | 'user'
  balance?: number
  concurrency?: number
  downstream_usage_token_mode?: DownstreamUsageTokenMode
  allowed_groups?: number[] | null
}): Promise<AdminUser> {
  const { data } = await apiClient.post<AdminUser>('/admin/users', userData)
  return data
}

/**
 * Update user
 * @param id - User ID
 * @param updates - Fields to update
 * @returns Updated user
 */
export async function update(id: number, updates: UpdateUserRequest): Promise<AdminUser> {
  const { data } = await apiClient.put<AdminUser>(`/admin/users/${id}`, updates)
  return data
}

/**
 * Delete user
 * @param id - User ID
 * @returns Success confirmation
 */
export async function deleteUser(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/users/${id}`)
  return data
}

/**
 * Update user balance
 * @param id - User ID
 * @param balance - New balance
 * @param operation - Operation type ('set', 'add', 'subtract')
 * @param notes - Optional notes for the balance adjustment
 * @returns Updated user
 */
export async function updateBalance(
  id: number,
  balance: number,
  operation: 'set' | 'add' | 'subtract' = 'set',
  notes?: string
): Promise<AdminUser> {
  const { data } = await apiClient.post<AdminUser>(`/admin/users/${id}/balance`, {
    balance,
    operation,
    notes: notes || ''
  })
  return data
}

/**
 * Update user concurrency
 * @param id - User ID
 * @param concurrency - New concurrency limit
 * @returns Updated user
 */
export async function updateConcurrency(id: number, concurrency: number): Promise<AdminUser> {
  return update(id, { concurrency })
}

/**
 * Toggle user status
 * @param id - User ID
 * @param status - New status
 * @returns Updated user
 */
export async function toggleStatus(id: number, status: UserStatus): Promise<AdminUser> {
  return update(id, { status })
}

/**
 * Get user's API keys
 * @param id - User ID
 * @returns List of user's API keys
 */
export async function getUserApiKeys(id: number): Promise<PaginatedResponse<ApiKey>> {
  const { data } = await apiClient.get<PaginatedResponse<ApiKey>>(`/admin/users/${id}/api-keys`)
  return data
}

/**
 * Get user's usage statistics
 * @param id - User ID
 * @param period - Time period
 * @returns User usage statistics
 */
export async function getUserUsageStats(
  id: number,
  period: string = 'month'
): Promise<{
  total_requests: number
  total_cost: number
  total_tokens: number
}> {
  const { data } = await apiClient.get<{
    total_requests: number
    total_cost: number
    total_tokens: number
  }>(`/admin/users/${id}/usage`, {
    params: { period }
  })
  return data
}

/**
 * Balance history item returned from the API
 */
export interface BalanceHistoryItem {
  id: number
  code: string
  type: string
  value: number
  status: string
  used_by: number | null
  used_at: string | null
  created_at: string
  group_id: number | null
  validity_days: number
  notes: string
  user?: { id: number; email: string } | null
  group?: { id: number; name: string } | null
}

// Balance history response extends pagination with total_recharged summary
export interface BalanceHistoryResponse extends PaginatedResponse<BalanceHistoryItem> {
  total_recharged: number
}

/**
 * Get user's balance/concurrency change history
 * @param id - User ID
 * @param page - Page number
 * @param pageSize - Items per page
 * @param type - Optional type filter (balance, admin_balance, concurrency, admin_concurrency, subscription)
 * @returns Paginated balance history with total_recharged
 */
export async function getUserBalanceHistory(
  id: number,
  page: number = 1,
  pageSize: number = 20,
  type?: string
): Promise<BalanceHistoryResponse> {
  const params: Record<string, any> = { page, page_size: pageSize }
  if (type) params.type = type
  const { data } = await apiClient.get<BalanceHistoryResponse>(
    `/admin/users/${id}/balance-history`,
    { params }
  )
  return data
}

/**
 * Replace user's exclusive group
 * @param userId - User ID
 * @param oldGroupId - Current group ID to replace
 * @param newGroupId - New group ID to replace with
 * @returns Number of migrated keys
 */
export async function replaceGroup(
  userId: number,
  oldGroupId: number,
  newGroupId: number
): Promise<{ migrated_keys: number }> {
  const { data } = await apiClient.post<{ migrated_keys: number }>(
    `/admin/users/${userId}/replace-group`,
    { old_group_id: oldGroupId, new_group_id: newGroupId }
  )
  return data
}

export async function bindUserAuthIdentity(
  userId: number,
  input: AdminBindAuthIdentityRequest
): Promise<AdminBoundAuthIdentity> {
  const { data } = await apiClient.post<AdminBoundAuthIdentity>(
    `/admin/users/${userId}/auth-identities`,
    input
  )
  return data
}

/**
 * Platform quota types
 */
export type PlatformQuotaPlatform = 'anthropic' | 'openai' | 'gemini' | 'antigravity' | 'grok'
export type PlatformQuotaWindow = 'daily' | 'weekly' | 'monthly'

export interface PlatformQuotaItem {
  platform: PlatformQuotaPlatform
  daily_limit_usd: number | null
  weekly_limit_usd: number | null
  monthly_limit_usd: number | null
  daily_usage_usd: number
  weekly_usage_usd: number
  monthly_usage_usd: number
  daily_window_start?: string | null
  weekly_window_start?: string | null
  monthly_window_start?: string | null
  daily_window_resets_at?: string | null
  weekly_window_resets_at?: string | null
  monthly_window_resets_at?: string | null
}

export interface PlatformQuotaUpdateItem {
  platform: PlatformQuotaPlatform
  daily_limit_usd: number | null
  weekly_limit_usd: number | null
  monthly_limit_usd: number | null
}

export interface PlatformQuotasResponse {
  platform_quotas: PlatformQuotaItem[]
}

/**
 * Get user's platform quotas
 */
export async function getPlatformQuotas(id: number): Promise<PlatformQuotasResponse> {
  const { data } = await apiClient.get<PlatformQuotasResponse>(
    `/admin/users/${id}/platform-quotas`
  )
  return data
}

/**
 * Replace user's platform quotas (全量替换)
 */
export async function updatePlatformQuotas(
  id: number,
  quotas: PlatformQuotaUpdateItem[]
): Promise<PlatformQuotasResponse> {
  const { data } = await apiClient.put<PlatformQuotasResponse>(
    `/admin/users/${id}/platform-quotas`,
    { quotas }
  )
  return data
}

/**
 * Reset a single (platform, window) usage immediately
 */
export async function resetPlatformQuotaWindow(
  id: number,
  platform: PlatformQuotaPlatform,
  window: PlatformQuotaWindow
): Promise<PlatformQuotasResponse> {
  const { data } = await apiClient.post<PlatformQuotasResponse>(
    `/admin/users/${id}/platform-quotas/reset`,
    { platform, window }
  )
  return data
}

/**
 * Batch fetch user last-N quality metrics (this user, all accounts).
 * Window N is this user's Q_u override, or the site-wide `account_quality_window_n` when inherited.
 */
export type SmartSchedulePlatform = PlatformQuotaPlatform

export interface SmartScheduleAccountMember {
  account_id: number
  platform?: string
  max_concurrency?: number | null
  sort_order?: number | null
  priority?: number
  current_concurrency?: number
  cooldown_until?: string | null
  cooldown_reason?: string | null
  resume_until?: string | null
  resume_chip_until?: string | null
  paused?: boolean
  probing?: boolean
  pinned?: boolean
  admission?: string | null
  state?: string | null
  probe_cap?: number | null
  probing_cap?: number | null
  in_flight_cap?: number | null
  pair_probe_cap?: number | null
}

export interface SmartScheduleSortAssignment {
  account_id: number
  sort_order: number
}

export {
  SMART_SCHEDULE_WINDOW_N_DEFAULT,
  SMART_SCHEDULE_WINDOW_N_MIN,
  SMART_SCHEDULE_WINDOW_N_MAX,
  clampSmartScheduleWindowN,
  resolveSmartScheduleWindowN,
  normalizeSmartSchedulePairQuality
} from '@/utils/smartScheduleWindowN'

/** Policy in-flight cap while probing. Not pair window N and not account-quality N. */
export type SmartScheduleProbeConcurrencyMode = 'follow_n' | 'custom'

export interface SmartSchedulePlatformView {
  enabled: boolean
  quality_max_p50_ttft_ms: number | null
  quality_max_p50_duration_ms?: number | null
  quality_max_slow_in_window?: number | null
  quality_max_consecutive_slow?: number | null
  quality_sched_window_n?: number | null
  quality_sched_max_slow_in_window?: number | null
  quality_sched_max_consecutive_slow?: number | null
  quality_min_success_rate: number | null
  /** Compat max of N首字 and N成功率. Do not use as a gate floor. */
  quality_window_n?: number | null
  /** Same compat max as quality_window_n. */
  quality_window_samples?: number | null
  /** N成功率: success last-N and open-judgment floor. */
  quality_min_success_samples: number | null
  /** N首字: TTFT last-N and open-judgment floor. */
  quality_min_ttft_samples: number | null
  quality_condition: 'or' | 'and' | null
  /** `follow_n` (default) uses N成功率; `custom` uses `probe_concurrency`. */
  probe_concurrency_mode?: SmartScheduleProbeConcurrencyMode | null
  /** Custom probe in-flight cap (1–100). Meaningful when mode is `custom`. */
  probe_concurrency?: number | null
  cooldown_minutes: number
  updated_at?: string
  accounts: SmartScheduleAccountMember[]
}

export interface UserSmartScheduleView {
  user_id: number
  default_platform?: SmartSchedulePlatform
  platforms: Record<string, SmartSchedulePlatformView>
}

export interface SmartScheduleSummary {
  enabled_platforms: string[]
  pool_counts: Record<string, number>
}

export interface BatchSmartScheduleSummariesResponse {
  summaries: Record<string, SmartScheduleSummary>
}

export interface SmartSchedulePlatformWrite {
  enabled: boolean
  quality_max_p50_ttft_ms?: number | null
  quality_max_p50_duration_ms?: number | null
  quality_max_slow_in_window?: number | null
  quality_max_consecutive_slow?: number | null
  quality_sched_window_n?: number | null
  quality_sched_max_slow_in_window?: number | null
  quality_sched_max_consecutive_slow?: number | null
  quality_min_success_rate?: number | null
  quality_window_n?: number | null
  quality_min_success_samples?: number | null
  quality_min_ttft_samples?: number | null
  quality_condition?: 'or' | 'and' | null
  probe_concurrency_mode?: SmartScheduleProbeConcurrencyMode | null
  probe_concurrency?: number | null
  cooldown_minutes: number
  accounts: SmartScheduleAccountMember[]
}

export type SmartSchedulePairQuality = {
  ttft_p50_ms?: number | null
  success_rate?: number | null
  ttft_samples: number
  ok_samples: number
  n: number
  n_ttft?: number
  n_success?: number
  n_ok?: number
}

export type SmartSchedulePairQualitySnapshot = SmartSchedulePairQuality & {
  captured_at: string
}

export type SmartSchedulePairQualityEventType =
  | 'cooldown_start'
  | 'cooldown_end'
  | 'resumed'
  | 'selectable'
  | 'pinned'
  | 'expiry_zero'
  | 'probe_enter'
  | 'probe_graduate'

export type SmartSchedulePairQualityEvent = {
  at: string
  type: SmartSchedulePairQualityEventType | string
  detail?: string | null
}

export type BatchSmartSchedulePairQualityResponse = {
  pairs: Record<string, SmartSchedulePairQuality>
}

export type SmartSchedulePairQualityDetail = {
  current?: SmartSchedulePairQuality | null
  snapshots: SmartSchedulePairQualitySnapshot[]
  events: SmartSchedulePairQualityEvent[]
}

export async function getSmartSchedule(id: number): Promise<UserSmartScheduleView> {
  const { data } = await apiClient.get<UserSmartScheduleView>(`/admin/users/${id}/smart-schedule`)
  return data
}

export async function updateSmartSchedule(
  id: number,
  platform: SmartSchedulePlatform,
  payload: SmartSchedulePlatformWrite
): Promise<UserSmartScheduleView> {
  const { data } = await apiClient.put<UserSmartScheduleView>(
    `/admin/users/${id}/smart-schedule/${platform}`,
    payload
  )
  return data
}

export async function updateSmartScheduleSortOrder(
  id: number,
  platform: SmartSchedulePlatform,
  payload: { accounts: SmartScheduleSortAssignment[] }
): Promise<UserSmartScheduleView> {
  const { data } = await apiClient.patch<UserSmartScheduleView>(
    `/admin/users/${id}/smart-schedule/${platform}/sort-order`,
    payload
  )
  return data
}

export async function copySmartSchedule(
  id: number,
  toPlatform: SmartSchedulePlatform,
  fromPlatform: SmartSchedulePlatform
): Promise<UserSmartScheduleView> {
  const { data } = await apiClient.post<UserSmartScheduleView>(
    `/admin/users/${id}/smart-schedule/${toPlatform}/copy`,
    { from_platform: fromPlatform }
  )
  return data
}

export async function getBatchSmartScheduleSummaries(
  userIds: number[]
): Promise<BatchSmartScheduleSummariesResponse> {
  const { data } = await apiClient.post<BatchSmartScheduleSummariesResponse>(
    '/admin/users/smart-schedule/summaries',
    { user_ids: userIds }
  )
  return data
}

export type SchedulePnlWindow = {
  revenue: number
  cost: number
  profit: number
  margin: number | null
}

export type SchedulePnlSummary = {
  today: SchedulePnlWindow | null
  seven_day: SchedulePnlWindow | null
}

export type SchedulePnlTrendRange = '24h' | 'today' | 'yesterday' | '7d'

export type SchedulePnlTrendPoint = {
  bucket: string
  revenue: number | null
  cost: number | null
  profit: number | null
  margin: number | null
}

export type SchedulePnlTrend = {
  range: SchedulePnlTrendRange | string
  granularity: 'hour' | 'day' | string
  points: SchedulePnlTrendPoint[]
}

export type BatchSmartSchedulePnlSummariesResponse = {
  summaries: Record<string, SchedulePnlSummary>
}

export type SmartSchedulePnlPairsResponse = {
  pairs: Record<string, SchedulePnlSummary>
}

function adminTimezoneParam() {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}

export async function getBatchSmartSchedulePnlSummaries(
  userIds: number[]
): Promise<BatchSmartSchedulePnlSummariesResponse> {
  const { data } = await apiClient.post<BatchSmartSchedulePnlSummariesResponse>(
    '/admin/users/smart-schedule/pnl/summaries',
    { user_ids: userIds },
    { params: { timezone: adminTimezoneParam() } }
  )
  return data
}

export async function getSmartSchedulePnlPairs(
  userId: number,
  accountIds: number[]
): Promise<SmartSchedulePnlPairsResponse> {
  const { data } = await apiClient.post<SmartSchedulePnlPairsResponse>(
    `/admin/users/${userId}/smart-schedule/pnl/pairs`,
    { account_ids: accountIds },
    { params: { timezone: adminTimezoneParam() } }
  )
  return data
}

export async function getSmartSchedulePairQualityBatch(
  userId: number,
  accountIds: number[],
  platform?: string
): Promise<BatchSmartSchedulePairQualityResponse> {
  const { data } = await apiClient.post<
    BatchSmartSchedulePairQualityResponse & { stats?: Record<string, SmartSchedulePairQuality> }
  >(
    `/admin/users/${userId}/smart-schedule/pair-quality`,
    { account_ids: accountIds, platform }
  )
  const bag = data.pairs ?? data.stats ?? {}
  const pairs: Record<string, SmartSchedulePairQuality> = {}
  for (const [key, value] of Object.entries(bag)) {
    const normalized = normalizeSmartSchedulePairQuality(value)
    if (normalized) pairs[key] = normalized
  }
  return { pairs }
}

function rfc3339FromUnixSeconds(ts: number | null | undefined): string {
  if (typeof ts !== 'number' || !Number.isFinite(ts) || ts <= 0) return ''
  return new Date(ts * 1000).toISOString()
}

type SmartSchedulePairQualityDetailResponse = {
  current?: SmartSchedulePairQuality | null
  live?: SmartSchedulePairQuality | null
  snapshots?: Array<Partial<SmartSchedulePairQualitySnapshot> & { ts?: number }>
  events?: Array<Partial<SmartSchedulePairQualityEvent> & { ts?: number; type?: string }>
}

export async function getSmartSchedulePairQualityDetail(
  userId: number,
  accountId: number,
  platform?: string
): Promise<SmartSchedulePairQualityDetail> {
  const path = platform
    ? `/admin/users/${userId}/smart-schedule/${encodeURIComponent(platform)}/accounts/${accountId}/pair-quality`
    : `/admin/users/${userId}/smart-schedule/pair-quality/${accountId}`
  const { data } = await apiClient.get<SmartSchedulePairQualityDetailResponse>(path)
  return {
    current: normalizeSmartSchedulePairQuality(data.current ?? data.live),
    snapshots: (data.snapshots ?? [])
      .map((point) => {
        const normalized = normalizeSmartSchedulePairQuality(point)
        if (!normalized) return null
        return {
          ...normalized,
          captured_at: point.captured_at || rfc3339FromUnixSeconds(point.ts)
        }
      })
      .filter((point): point is SmartSchedulePairQualitySnapshot => point != null),
    events: (data.events ?? []).map((event) => ({
      type: event.type ?? '',
      at: event.at || rfc3339FromUnixSeconds(event.ts),
      detail: event.detail ?? null
    }))
  }
}

export async function getSmartSchedulePnlTrend(
  userId: number,
  range: SchedulePnlTrendRange = '24h',
  accountId?: number
): Promise<SchedulePnlTrend> {
  const { data } = await apiClient.get<SchedulePnlTrend>(
    `/admin/users/${userId}/smart-schedule/pnl/trend`,
    {
      params: {
        range,
        timezone: adminTimezoneParam(),
        ...(accountId && accountId > 0 ? { account_id: accountId } : {})
      }
    }
  )
  return data
}

export async function getBatchQualityStats(userIds: number[]): Promise<BatchQualityStatsResponse> {
  const { data } = await apiClient.post<BatchQualityStatsResponse>('/admin/users/quality-stats/batch', {
    user_ids: userIds
  })
  return data
}

/**
 * Fetch persisted last-N quality snapshots for one user (all accounts).
 * Do not call account quality-history with a user id.
 * Omit from/to to use the server default (last 24 hours).
 */
export async function getQualityHistory(
  id: number,
  params?: { from?: string; to?: string }
): Promise<AccountQualityHistoryResponse> {
  const { data } = await apiClient.get<AccountQualityHistoryResponse>(
    `/admin/users/${id}/quality-history`,
    { params }
  )
  return data
}

export const usersAPI = {
  list,
  getById,
  create,
  update,
  delete: deleteUser,
  updateBalance,
  updateConcurrency,
  toggleStatus,
  getUserApiKeys,
  getUserUsageStats,
  getUserBalanceHistory,
  replaceGroup,
  bindUserAuthIdentity,
  getPlatformQuotas,
  updatePlatformQuotas,
  resetPlatformQuotaWindow,
  getSmartSchedule,
  updateSmartSchedule,
  updateSmartScheduleSortOrder,
  copySmartSchedule,
  getBatchSmartScheduleSummaries,
  getBatchSmartSchedulePnlSummaries,
  getSmartSchedulePnlPairs,
  getSmartSchedulePnlTrend,
  getSmartSchedulePairQualityBatch,
  getSmartSchedulePairQualityDetail,
  getBatchQualityStats,
  getQualityHistory,
}

export default usersAPI
