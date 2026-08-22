import type { Account } from '@/types'
import {
  accountMatchesListFilters,
  type AccountListFilterState
} from '@/utils/accountListFilters'
import { isCurrentlySchedulingAccount } from '@/composables/smartSchedulePoolAdmission'

export type SmartScheduleAddCandidateFilters = AccountListFilterState & {
  schedulable: '' | 'on' | 'off'
  scheduling: '' | 'on' | 'off'
  proxy: string
}

export const EMPTY_SMART_SCHEDULE_ADD_FILTERS: SmartScheduleAddCandidateFilters = {
  platform: '',
  type: '',
  status: '',
  group: '',
  privacy_mode: '',
  search: '',
  schedulable: '',
  scheduling: '',
  proxy: ''
}

export function emptySmartScheduleAddFilters(
  platform: string
): SmartScheduleAddCandidateFilters {
  if (platform === 'antigravity') {
    return { ...EMPTY_SMART_SCHEDULE_ADD_FILTERS, platform: '' }
  }
  return { ...EMPTY_SMART_SCHEDULE_ADD_FILTERS, platform }
}

function accountSearchHaystack(account: Account): string {
  const email = String(account.extra?.email_address || account.credentials?.email || account.parent_email || '')
  return `${account.name} ${account.id} ${email}`.toLowerCase()
}

export function matchesAddCandidateFilters(
  account: Account,
  filters: SmartScheduleAddCandidateFilters,
  now = Date.now()
): boolean {
  const { search, schedulable, scheduling, proxy, ...listFilters } = filters
  if (!accountMatchesListFilters(account, { ...listFilters, search: '' }, now)) return false

  const query = String(search || '').trim().toLowerCase()
  if (query && !accountSearchHaystack(account).includes(query)) return false

  if (schedulable === 'on' && !account.schedulable) return false
  if (schedulable === 'off' && account.schedulable) return false

  if (scheduling === 'on' && !isCurrentlySchedulingAccount(account)) return false
  if (scheduling === 'off' && isCurrentlySchedulingAccount(account)) return false

  if (proxy === 'none') {
    if (account.proxy_id != null) return false
  } else if (proxy) {
    if (String(account.proxy_id ?? '') !== proxy) return false
  }

  return true
}

export function filterAddCandidates(
  accounts: Account[],
  filters: SmartScheduleAddCandidateFilters,
  now = Date.now()
): Account[] {
  return accounts.filter((account) => matchesAddCandidateFilters(account, filters, now))
}
