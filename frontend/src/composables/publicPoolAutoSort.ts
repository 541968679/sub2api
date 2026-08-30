import type { PublicScheduleQualityView } from '@/api/admin/accounts'
import {
  POOL_ADMISSION_SORT_RANK,
  effectivePoolAutoSortRate,
  sortSmartSchedulePoolMembers,
  type PoolAutoSortItem
} from './smartSchedulePoolAutoSort'
import type { PoolAdmissionState } from './smartSchedulePoolAdmission'

/** Matches backend maxAccountAutoSortIDs (select-all-filtered scale). */
export const PUBLIC_POOL_AUTO_SORT_MAX_IDS = 2000

export type PublicPoolAccountLike = {
  id: number
  status?: string | null
  schedulable?: boolean | null
  public_schedulable?: boolean | null
  fallback_only?: boolean | null
  concurrency?: number | null
  current_concurrency?: number | null
  priority?: number | null
  type?: string | null
  upstream_rate_multiplier?: number | null
  extra?: Record<string, unknown> | null
}

export type PublicPoolQualityLike = Pick<PublicScheduleQualityView, 'state' | 'will_cool'> | null | undefined

export type PublicPoolAutoSortItem = PoolAutoSortItem & {
  fallbackOnly: boolean
}

export type PublicPoolListOrderAssignment = {
  id: number
  listOrder: number
}

export function isPublicPoolFallbackOnly(account: PublicPoolAccountLike): boolean {
  return account.fallback_only === true || account.extra?.fallback_only === true
}

export function isAccountListPinned(extra?: Record<string, unknown> | null): boolean {
  const raw = extra?.list_pinned
  if (raw === true) return true
  if (typeof raw === 'string') return raw.trim().toLowerCase() === 'true'
  return false
}

export function accountListOrderFromExtra(extra?: Record<string, unknown> | null): number {
  const raw = extra?.list_order
  if (typeof raw === 'number' && Number.isFinite(raw)) return raw
  if (typeof raw === 'string') {
    const parsed = Number(raw)
    if (Number.isFinite(parsed)) return parsed
  }
  return 0
}

export function publicPoolAdmissionFromAccount(
  account: PublicPoolAccountLike,
  view?: PublicPoolQualityLike
): PoolAdmissionState {
  if (account.public_schedulable === false || account.status !== 'active' || account.schedulable === false) {
    return 'stopped'
  }
  const concurrency = account.concurrency ?? 0
  const current = account.current_concurrency ?? 0
  if (concurrency >= 1 && current >= concurrency) {
    return 'pair_full'
  }
  if (!view) return 'selectable'
  if (view.state === 'selectable' && view.will_cool) return 'will_cool'
  if (
    view.state === 'cooling' ||
    view.state === 'paused' ||
    view.state === 'probing' ||
    view.state === 'resumed' ||
    view.state === 'pinned'
  ) {
    return view.state
  }
  return 'selectable'
}

export function mapPublicPoolAccountToAutoSortItem(
  account: PublicPoolAccountLike,
  view?: PublicPoolQualityLike
): PublicPoolAutoSortItem {
  const concurrency = account.concurrency ?? 0
  const current = account.current_concurrency ?? 0
  return {
    id: account.id,
    admission: publicPoolAdmissionFromAccount(account, view),
    pairCap: concurrency >= 1 ? concurrency : null,
    pairCurrent: current,
    concurrency,
    priority: account.priority ?? 0,
    upstreamRate: effectivePoolAutoSortRate(account.upstream_rate_multiplier, account.type),
    fallbackOnly: isPublicPoolFallbackOnly(account)
  }
}

function publicPoolAutoSortGroupKey(item: PublicPoolAutoSortItem): string {
  const rank = POOL_ADMISSION_SORT_RANK[item.admission] ?? 99
  return `${String(rank).padStart(2, '0')}:${item.fallbackOnly ? '1' : '0'}`
}

/**
 * Apply fallback_only inside the same admission band, then reuse the pool sorter.
 * Does not change comparePoolAutoSort.
 */
export function sortPublicPoolAutoSortItems<T extends PublicPoolAutoSortItem>(items: T[]): T[] {
  const buckets = new Map<string, T[]>()
  for (const item of items) {
    const key = publicPoolAutoSortGroupKey(item)
    const bucket = buckets.get(key)
    if (bucket) bucket.push(item)
    else buckets.set(key, [item])
  }
  return [...buckets.keys()]
    .sort((a, b) => (a < b ? -1 : a > b ? 1 : 0))
    .flatMap((key) => sortSmartSchedulePoolMembers(buckets.get(key)!))
}

export function writablePublicPoolAutoSortIds(
  sorted: Array<{ id: number }>,
  pinnedIds: Iterable<number>
): number[] {
  const pinned = new Set(pinnedIds)
  return sorted.filter((item) => !pinned.has(item.id)).map((item) => item.id)
}

/** Reserved band N…1 (larger = higher), matching backend ReorderAccountsAutoSort. */
export function assignPublicPoolReservedBand(ids: number[]): PublicPoolListOrderAssignment[] {
  const n = ids.length
  return ids.map((id, index) => ({ id, listOrder: n - index }))
}

export function publicPoolReservedBandUnchanged(
  current: Array<{ id: number; listOrder: number }>,
  assigned: PublicPoolListOrderAssignment[]
): boolean {
  if (assigned.length !== current.length) return false
  const byID = new Map(current.map((row) => [row.id, row.listOrder]))
  return assigned.every((row) => byID.get(row.id) === row.listOrder)
}
