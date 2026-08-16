import type { PoolAdmissionState } from './smartSchedulePoolAdmission'

export const POOL_ADMISSION_SORT_RANK: Record<PoolAdmissionState, number> = {
  selectable: 0,
  resumed: 0,
  will_cool: 1,
  unsaved_preview: 1,
  pair_full: 2,
  cooling: 3,
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
  lastUsedAt?: string | null
}

export type PoolAutoSortAssignment = {
  id: number
  nextPriority: number
}

export function pairHeadroomForAutoSort(pairCap: number | null, pairCurrent: number): number {
  if (pairCap == null || pairCap < 1) return UNCAPPED_PAIR_HEADROOM
  return pairCap - pairCurrent
}

function lastUsedMs(value?: string | null): number {
  if (!value) return Number.NEGATIVE_INFINITY
  const ms = new Date(value).getTime()
  return Number.isFinite(ms) ? ms : Number.NEGATIVE_INFINITY
}

export function comparePoolAutoSort(a: PoolAutoSortItem, b: PoolAutoSortItem): number {
  const admission =
    (POOL_ADMISSION_SORT_RANK[a.admission] ?? 99) - (POOL_ADMISSION_SORT_RANK[b.admission] ?? 99)
  if (admission !== 0) return admission

  const headroom =
    pairHeadroomForAutoSort(b.pairCap, b.pairCurrent) - pairHeadroomForAutoSort(a.pairCap, a.pairCurrent)
  if (headroom !== 0) return headroom

  const concurrency = (b.concurrency ?? 0) - (a.concurrency ?? 0)
  if (concurrency !== 0) return concurrency

  const priority = (b.priority ?? 0) - (a.priority ?? 0)
  if (priority !== 0) return priority

  const lastUsed = lastUsedMs(a.lastUsedAt) - lastUsedMs(b.lastUsedAt)
  if (Number.isFinite(lastUsed) && lastUsed !== 0) return lastUsed

  return a.id - b.id
}

export function sortSmartSchedulePoolMembers<T extends PoolAutoSortItem>(items: T[]): T[] {
  return [...items].sort(comparePoolAutoSort)
}

/**
 * Reassign this pool's existing priority slots so the first (most desirable) row
 * gets the smallest number. Claude / same-rate OpenAI prefer lower `priority`.
 * Duplicates are spread upward so the order is total, without inventing 1..N
 * that would jump the whole pool ahead of the rest of the fleet.
 */
export function makeDistinctAscendingPrioritySlots(values: number[]): number[] {
  const slots = values.map((value) => {
    if (!Number.isFinite(value)) return 0
    return Math.max(0, Math.trunc(value))
  })
  slots.sort((left, right) => left - right)
  for (let i = 1; i < slots.length; i++) {
    if (slots[i] <= slots[i - 1]) {
      slots[i] = slots[i - 1] + 1
    }
  }
  return slots
}

export function assignPoolAutoSortPriorities(
  sorted: Array<Pick<PoolAutoSortItem, 'id' | 'priority'>>
): PoolAutoSortAssignment[] {
  const slots = makeDistinctAscendingPrioritySlots(sorted.map((item) => item.priority ?? 0))
  return sorted.map((item, index) => ({
    id: item.id,
    nextPriority: slots[index] ?? index
  }))
}

function normalizePoolPriority(value: number | null | undefined): number {
  if (value == null || !Number.isFinite(value)) return 0
  return Math.trunc(value)
}

/**
 * Priority to write so this row is first under `priority asc` (smaller = more
 * preferred, same as Claude / same-rate OpenAI). `null` means the account is
 * already the unique pool min — no write needed, only switch the table sort.
 */
export function nextPriorityForPoolMoveToTop(
  poolPriorities: Array<number | null | undefined>,
  currentPriority: number | null | undefined
): number | null {
  const slots = (poolPriorities.length > 0 ? poolPriorities : [currentPriority]).map(normalizePoolPriority)
  const current = normalizePoolPriority(currentPriority)
  const min = slots.reduce((lowest, value) => Math.min(lowest, value), Number.POSITIVE_INFINITY)
  if (!Number.isFinite(min)) return -1
  const minCount = slots.filter((value) => value === min).length
  if (current === min && minCount === 1) return null
  return min - 1
}
