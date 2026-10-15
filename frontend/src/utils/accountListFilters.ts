import type { Account } from '@/types'

export const ACCOUNT_UNGROUPED_GROUP_QUERY_VALUE = 'ungrouped'
export const ACCOUNT_PRIVACY_MODE_UNSET_QUERY_VALUE = '__unset__'

export type AccountListFilterState = {
  platform?: string
  type?: string
  status?: string
  group?: string
  privacy_mode?: string
  search?: string
  /** Group ids that define the 国产模型 tab. Not an account.platform value. */
  platform_view_group_ids?: number[]
  /** Group ids hidden from the OpenAI tab when no single group is selected. */
  exclude_group_ids?: number[]
}

function accountGroupIds(account: Pick<Account, 'group_ids' | 'groups'>): number[] {
  return account.group_ids ?? account.groups?.map((group) => group.id) ?? []
}

function accountPrivacyMode(account: Pick<Account, 'extra'>): string {
  return typeof account.extra?.privacy_mode === 'string' ? account.extra.privacy_mode : ''
}

export function accountMatchesListFilters(
  account: Pick<
    Account,
    | 'platform'
    | 'type'
    | 'status'
    | 'schedulable'
    | 'rate_limit_reset_at'
    | 'temp_unschedulable_until'
    | 'group_ids'
    | 'groups'
    | 'extra'
    | 'name'
  >,
  filters: AccountListFilterState,
  now = Date.now()
): boolean {
  if (filters.platform === 'domestic') {
    if (!filters.group) {
      const wanted = filters.platform_view_group_ids ?? []
      const memberIDs = accountGroupIds(account)
      if (!memberIDs.some((id) => wanted.includes(id))) return false
    }
  } else if (filters.platform && account.platform !== filters.platform) {
    return false
  }
  if (!filters.group && filters.exclude_group_ids && filters.exclude_group_ids.length > 0) {
    const memberIDs = accountGroupIds(account)
    if (memberIDs.some((id) => filters.exclude_group_ids?.includes(id))) return false
  }
  if (filters.type && account.type !== filters.type) return false
  if (filters.status) {
    const rateLimitResetAt = account.rate_limit_reset_at ? new Date(account.rate_limit_reset_at).getTime() : Number.NaN
    const isRateLimited = Number.isFinite(rateLimitResetAt) && rateLimitResetAt > now
    const tempUnschedUntil = account.temp_unschedulable_until
      ? new Date(account.temp_unschedulable_until).getTime()
      : Number.NaN
    const isTempUnschedulable = Number.isFinite(tempUnschedUntil) && tempUnschedUntil > now

    if (filters.status === 'active') {
      if (account.status !== 'active' || isRateLimited || isTempUnschedulable || !account.schedulable) return false
    } else if (filters.status === 'rate_limited') {
      if (account.status !== 'active' || !isRateLimited || isTempUnschedulable) return false
    } else if (filters.status === 'temp_unschedulable') {
      if (account.status !== 'active' || !isTempUnschedulable) return false
    } else if (filters.status === 'unschedulable') {
      if (account.status !== 'active' || account.schedulable || isRateLimited || isTempUnschedulable) return false
    } else if (account.status !== filters.status) {
      return false
    }
  }
  if (filters.group) {
    const groupIds = accountGroupIds(account)
    if (filters.group === ACCOUNT_UNGROUPED_GROUP_QUERY_VALUE) {
      if (groupIds.length > 0) return false
    } else if (!groupIds.includes(Number(filters.group))) {
      return false
    }
  }
  const privacyMode = accountPrivacyMode(account)
  if (filters.privacy_mode) {
    if (filters.privacy_mode === ACCOUNT_PRIVACY_MODE_UNSET_QUERY_VALUE) {
      if (privacyMode.trim() !== '') return false
    } else if (privacyMode !== filters.privacy_mode) {
      return false
    }
  }
  const search = String(filters.search || '').trim().toLowerCase()
  if (search && !account.name.toLowerCase().includes(search)) return false
  return true
}
