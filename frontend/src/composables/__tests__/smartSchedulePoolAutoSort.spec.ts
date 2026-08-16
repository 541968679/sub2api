import { describe, expect, it } from 'vitest'
import {
  UNCAPPED_PAIR_HEADROOM,
  assignPoolAutoSortOrders,
  assignPoolMoveToTopSortOrders,
  comparePoolAutoSort,
  pairHeadroomForAutoSort,
  poolSortOrdersUnchanged,
  sortSmartSchedulePoolMembers,
  type PoolAutoSortItem
} from '../smartSchedulePoolAutoSort'

function item(overrides: Partial<PoolAutoSortItem> & Pick<PoolAutoSortItem, 'id'>): PoolAutoSortItem {
  return {
    admission: 'selectable',
    pairCap: null,
    pairCurrent: 0,
    concurrency: 1,
    priority: 0,
    lastUsedAt: null,
    ...overrides
  }
}

describe('pairHeadroomForAutoSort', () => {
  it('uses a high sentinel for uncapped pairs instead of account concurrency', () => {
    expect(pairHeadroomForAutoSort(null, 9)).toBe(UNCAPPED_PAIR_HEADROOM)
    expect(pairHeadroomForAutoSort(0, 9)).toBe(UNCAPPED_PAIR_HEADROOM)
    expect(pairHeadroomForAutoSort(2, 2)).toBe(0)
    expect(pairHeadroomForAutoSort(3, 1)).toBe(2)
    expect(UNCAPPED_PAIR_HEADROOM).toBeGreaterThan(2)
  })
})

describe('comparePoolAutoSort', () => {
  it('lets admission beat a much higher concurrency', () => {
    const selectable = item({ id: 1, admission: 'selectable', concurrency: 1 })
    const stopped = item({ id: 2, admission: 'stopped', concurrency: 99 })
    expect(comparePoolAutoSort(selectable, stopped)).toBeLessThan(0)
    expect(sortSmartSchedulePoolMembers([stopped, selectable]).map((row) => row.id)).toEqual([1, 2])
  })

  it('ranks uncapped headroom above a capped-full peer', () => {
    const uncapped = item({ id: 3, pairCap: null, pairCurrent: 8, concurrency: 1 })
    const full = item({ id: 4, admission: 'pair_full', pairCap: 2, pairCurrent: 2, concurrency: 8 })
    expect(sortSmartSchedulePoolMembers([full, uncapped]).map((row) => row.id)).toEqual([3, 4])
  })

  it('puts stopped last and treats preview/hint like will-cool, not a dead lock', () => {
    const rows = [
      item({ id: 5, admission: 'stopped' }),
      item({ id: 1, admission: 'selectable' }),
      item({ id: 6, admission: 'unsaved_preview' }),
      item({ id: 4, admission: 'will_cool' }),
      item({ id: 7, admission: 'resumed' }),
      item({ id: 2, admission: 'pair_full' }),
      item({ id: 3, admission: 'cooling' })
    ]
    expect(sortSmartSchedulePoolMembers(rows).map((row) => row.admission)).toEqual([
      'selectable',
      'resumed',
      'will_cool',
      'unsaved_preview',
      'pair_full',
      'cooling',
      'stopped'
    ])
  })

  it('reads account priority only as a later tie-break', () => {
    const preferred = item({ id: 2, priority: 1, concurrency: 4 })
    const other = item({ id: 9, priority: 80, concurrency: 4 })
    expect(sortSmartSchedulePoolMembers([other, preferred]).map((row) => row.id)).toEqual([2, 9])
  })

  it('breaks remaining ties by account id', () => {
    const left = item({ id: 20, priority: 7, lastUsedAt: '2026-08-01T00:00:00.000Z' })
    const right = item({ id: 8, priority: 7, lastUsedAt: '2026-08-01T00:00:00.000Z' })
    expect(comparePoolAutoSort(left, right)).toBeGreaterThan(0)
    expect(sortSmartSchedulePoolMembers([left, right]).map((row) => row.id)).toEqual([8, 20])
  })
})

describe('assignPoolAutoSortOrders', () => {
  it('writes 1..N pool order and never invents account priorities', () => {
    const assigned = assignPoolAutoSortOrders([{ id: 11 }, { id: 12 }, { id: 13 }])
    expect(assigned).toEqual([
      { id: 11, sortOrder: 1 },
      { id: 12, sortOrder: 2 },
      { id: 13, sortOrder: 3 }
    ])
  })
})

describe('assignPoolMoveToTopSortOrders', () => {
  it('puts the target first and keeps the rest by existing sort_order then id', () => {
    const assigned = assignPoolMoveToTopSortOrders(
      [
        { id: 11, sortOrder: 1 },
        { id: 12, sortOrder: 2 },
        { id: 13, sortOrder: 3 }
      ],
      13
    )
    expect(assigned).toEqual([
      { id: 13, sortOrder: 1 },
      { id: 11, sortOrder: 2 },
      { id: 12, sortOrder: 3 }
    ])
  })

  it('before any sort_order exists, still assigns a total 1..N order', () => {
    const assigned = assignPoolMoveToTopSortOrders(
      [
        { id: 20, sortOrder: null },
        { id: 8, sortOrder: null }
      ],
      20
    )
    expect(assigned).toEqual([
      { id: 20, sortOrder: 1 },
      { id: 8, sortOrder: 2 }
    ])
  })
})

describe('poolSortOrdersUnchanged', () => {
  it('is true only when every membership already has the assigned sort_order', () => {
    const current = [
      { id: 1, sortOrder: 1 },
      { id: 2, sortOrder: 2 }
    ]
    expect(poolSortOrdersUnchanged(current, [
      { id: 1, sortOrder: 1 },
      { id: 2, sortOrder: 2 }
    ])).toBe(true)
    expect(poolSortOrdersUnchanged(current, [
      { id: 2, sortOrder: 1 },
      { id: 1, sortOrder: 2 }
    ])).toBe(false)
    expect(poolSortOrdersUnchanged(
      [{ id: 1, sortOrder: null }, { id: 2, sortOrder: null }],
      [{ id: 1, sortOrder: 1 }, { id: 2, sortOrder: 2 }]
    )).toBe(false)
  })
})
