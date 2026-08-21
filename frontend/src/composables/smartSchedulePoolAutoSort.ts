import { defaultUpstreamRateMultiplier } from '@/utils/accountUpstreamRate'
import type { PoolAdmissionState } from './smartSchedulePoolAdmission'

export const POOL_ADMISSION_SORT_RANK: Record<PoolAdmissionState, number> = {
  selectable: 0,
  probing: 0,
  resumed: 0,
  will_cool: 1,
  unsaved_preview: 1,
  pair_full: 2,
  cooling: 3,
  paused: 3,
  stopped: 4
}

/** Sentinel headroom when pair cap is empty/null. Not `account.concurrency`. */
export const UNCAPPED_PAIR_HEADROOM = 1_000_000

export type PoolAutoSortItem = {
  id: number
  admission: PoolAdmissionState
  pairCap: number | null
  pairCurrent: number
  concurrency: number
  priority: number
  upstreamRate: number
}

export type PoolSortAssignment = {
  id: number
  sortOrder: number
}

export type PoolMemberSortState = {
  id: number
  sortOrder: number | null
}

export function pairHeadroomForAutoSort(pairCap: number | null, pairCurrent: number): number {
  if (pairCap == null || pairCap < 1) return UNCAPPED_PAIR_HEADROOM
  return pairCap - pairCurrent
}

export function isPoolAutoSortProducing(pairCurrent: number): boolean {
  return pairCurrent > 0
}

/** Same defaulting as account `EffectiveUpstreamRate`: missing/negative uses type default. */
export function effectivePoolAutoSortRate(
  rate: number | null | undefined,
  type?: string | null
): number {
  if (rate != null && Number.isFinite(rate) && rate >= 0) return rate
  return defaultUpstreamRateMultiplier(type ?? '')
}

export function comparePoolAutoSort(a: PoolAutoSortItem, b: PoolAutoSortItem): number {
  const admission =
    (POOL_ADMISSION_SORT_RANK[a.admission] ?? 99) - (POOL_ADMISSION_SORT_RANK[b.admission] ?? 99)
  if (admission !== 0) return admission

  const producing =
    Number(isPoolAutoSortProducing(b.pairCurrent)) - Number(isPoolAutoSortProducing(a.pairCurrent))
  if (producing !== 0) return producing

  const rate = (a.upstreamRate ?? 0) - (b.upstreamRate ?? 0)
  if (rate !== 0) return rate

  const headroom =
    pairHeadroomForAutoSort(b.pairCap, b.pairCurrent) - pairHeadroomForAutoSort(a.pairCap, a.pairCurrent)
  if (headroom !== 0) return headroom

  const concurrency = (b.concurrency ?? 0) - (a.concurrency ?? 0)
  if (concurrency !== 0) return concurrency

  // Read-only tie-break: accounts.priority is scheduling weight (smaller = more preferred).
  const priority = (a.priority ?? 0) - (b.priority ?? 0)
  if (priority !== 0) return priority

  return a.id - b.id
}

export function sortSmartSchedulePoolMembers<T extends PoolAutoSortItem>(items: T[]): T[] {
  return [...items].sort(comparePoolAutoSort)
}

export function assignPoolAutoSortOrders(
  sorted: Array<Pick<PoolAutoSortItem, 'id'>>
): PoolSortAssignment[] {
  return sorted.map((item, index) => ({
    id: item.id,
    sortOrder: index + 1
  }))
}

export function comparePoolMemberDisplayOrder(a: PoolMemberSortState, b: PoolMemberSortState): number {
  if (a.sortOrder == null && b.sortOrder == null) return a.id - b.id
  if (a.sortOrder == null) return 1
  if (b.sortOrder == null) return -1
  if (a.sortOrder !== b.sortOrder) return a.sortOrder - b.sortOrder
  return a.id - b.id
}

export function assignPoolMoveToTopSortOrders(
  members: PoolMemberSortState[],
  targetId: number
): PoolSortAssignment[] {
  const target = members.find((item) => item.id === targetId)
  const others = members
    .filter((item) => item.id !== targetId)
    .sort(comparePoolMemberDisplayOrder)
  const ordered = target ? [target, ...others] : others
  return assignPoolAutoSortOrders(ordered)
}

export function poolSortOrdersUnchanged(
  current: PoolMemberSortState[],
  assigned: PoolSortAssignment[]
): boolean {
  const byID = new Map(current.map((item) => [item.id, item.sortOrder]))
  return assigned.every((row) => byID.get(row.id) === row.sortOrder)
}
