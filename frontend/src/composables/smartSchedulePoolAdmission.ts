import type { Account } from '@/types'
import type { AccountQualityStats } from '@/api/admin/accounts'
import type { SmartSchedulePlatform, UserSmartScheduleView } from '@/api/admin/users'
import { percentToSuccessRate, userQualityGateBreached } from '@/utils/accountQualityHardClose'

const DEFAULT_PLATFORMS: SmartSchedulePlatform[] = [
  'anthropic',
  'openai',
  'gemini',
  'antigravity',
  'grok'
]

export type PoolAdmissionState = 'stopped' | 'cooling' | 'pair_full' | 'quality_blocked' | 'selectable'

export type PoolAdmission = {
  state: PoolAdmissionState
  cooldownUntil?: string
}

export type SmartSchedulePoolFilters = {
  search: string
  type: string
  schedulable: '' | 'on' | 'off'
  admission: '' | PoolAdmissionState
}

export const EMPTY_SMART_SCHEDULE_POOL_FILTERS: SmartSchedulePoolFilters = {
  search: '',
  type: '',
  schedulable: '',
  admission: ''
}

export function resolvePairCap(maxConcurrency: number | null | undefined): number | null {
  return maxConcurrency != null && maxConcurrency >= 1 ? maxConcurrency : null
}

export function isPairCooldownActive(cooldownUntil?: string | null, now = Date.now()): boolean {
  if (!cooldownUntil) return false
  const until = new Date(cooldownUntil).getTime()
  return Number.isFinite(until) && until > now
}

export function cooldownRemainingMinutes(cooldownUntil?: string | null, now = Date.now()): number {
  if (!cooldownUntil) return 0
  const until = new Date(cooldownUntil).getTime()
  if (!Number.isFinite(until) || until <= now) return 0
  return Math.max(1, Math.ceil((until - now) / 60_000))
}

export function resolvePoolAdmission(input: {
  account: Pick<Account, 'status' | 'schedulable' | 'temp_unschedulable_until' | 'rate_limit_reset_at'>
  pairCap: number | null
  pairCurrent: number
  cooldownUntil?: string | null
  qualityBlocked?: boolean
  now?: number
}): PoolAdmission {
  const now = input.now ?? Date.now()
  if (!isCurrentlySchedulingAccount(input.account)) {
    return { state: 'stopped' }
  }
  if (isPairCooldownActive(input.cooldownUntil, now)) {
    return { state: 'cooling', cooldownUntil: input.cooldownUntil ?? undefined }
  }
  if (input.pairCap != null && input.pairCap >= 1 && input.pairCurrent >= input.pairCap) {
    return { state: 'pair_full' }
  }
  if (input.qualityBlocked) {
    return { state: 'quality_blocked' }
  }
  return { state: 'selectable' }
}

export function qualityBlockedFromDraft(
  draft: {
    maxP50: number | ''
    successPercent: number | ''
    minSuccessSamples: number | ''
    minTtftSamples: number | ''
    condition: 'or' | 'and'
  } | undefined,
  stats: AccountQualityStats | null | undefined
): boolean {
  if (!draft) return false
  return userQualityGateBreached(
    {
      quality_max_p50_ttft_ms: draft.maxP50 === '' ? null : Number(draft.maxP50),
      quality_min_success_rate: percentToSuccessRate(draft.successPercent),
      quality_min_success_samples: draft.minSuccessSamples === '' ? null : Number(draft.minSuccessSamples),
      quality_min_ttft_samples: draft.minTtftSamples === '' ? null : Number(draft.minTtftSamples),
      quality_condition: draft.condition
    },
    stats
  )
}

export function matchesPoolFilters(
  account: Account,
  admission: PoolAdmissionState,
  filters: SmartSchedulePoolFilters
): boolean {
  const query = filters.search.trim().toLowerCase()
  if (query) {
    const email = String(account.extra?.email_address || account.credentials?.email || account.parent_email || '')
    const haystack = `${account.name} ${account.id} ${email}`.toLowerCase()
    if (!haystack.includes(query)) return false
  }
  if (filters.type && account.type !== filters.type) return false
  if (filters.schedulable === 'on' && !account.schedulable) return false
  if (filters.schedulable === 'off' && account.schedulable) return false
  if (filters.admission && admission !== filters.admission) return false
  return true
}

export function isCurrentlySchedulingAccount(account: {
  status?: string
  schedulable?: boolean
  temp_unschedulable_until?: string | null
  rate_limit_reset_at?: string | null
}): boolean {
  if (account.status !== 'active' || !account.schedulable) return false
  const now = Date.now()
  if (account.temp_unschedulable_until) {
    const until = new Date(account.temp_unschedulable_until).getTime()
    if (Number.isFinite(until) && until > now) return false
  }
  if (account.rate_limit_reset_at) {
    const until = new Date(account.rate_limit_reset_at).getTime()
    if (Number.isFinite(until) && until > now) return false
  }
  return true
}

export function pickDefaultSmartSchedulePlatform(
  view: UserSmartScheduleView | null | undefined,
  platforms: readonly SmartSchedulePlatform[] = DEFAULT_PLATFORMS
): SmartSchedulePlatform {
  const hinted = view?.default_platform
  if (hinted && platforms.includes(hinted as SmartSchedulePlatform)) {
    return hinted as SmartSchedulePlatform
  }
  const rows = platforms.map((platform) => {
    const row = view?.platforms?.[platform]
    return {
      platform,
      enabled: Boolean(row?.enabled && (row.accounts?.length ?? 0) > 0),
      members: row?.accounts?.length ?? 0,
      updatedAt: row?.updated_at ? new Date(row.updated_at).getTime() : 0
    }
  })
  const enabled = rows.filter((row) => row.enabled)
  const withPool = rows.filter((row) => row.members > 0)
  const pool = enabled.length > 0 ? enabled : withPool
  if (pool.length === 0) return platforms[0] ?? 'anthropic'
  return pool.reduce((best, row) => {
    if (row.updatedAt > best.updatedAt) return row
    if (row.updatedAt === best.updatedAt && row.members > best.members) return row
    return best
  }).platform
}
