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
  if (filters.platform && account.platform !== filters.platform) return false
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
