import { describe, expect, it } from 'vitest'
import {
  UNCAPPED_PAIR_HEADROOM,
  assignPoolAutoSortPriorities,
  comparePoolAutoSort,
  nextPriorityForPoolMoveToTop,
  pairHeadroomForAutoSort,
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

  it('breaks remaining ties by account id', () => {
    const left = item({ id: 20, priority: 7, lastUsedAt: '2026-08-01T00:00:00.000Z' })
    const right = item({ id: 8, priority: 7, lastUsedAt: '2026-08-01T00:00:00.000Z' })
    expect(comparePoolAutoSort(left, right)).toBeGreaterThan(0)
    expect(sortSmartSchedulePoolMembers([left, right]).map((row) => row.id)).toEqual([8, 20])
  })
})

describe('assignPoolAutoSortPriorities', () => {
  it('gives the first row the smallest existing slot so Claude ranking stays lower-is-better', () => {
    const assigned = assignPoolAutoSortPriorities([
      { id: 1, priority: 80 },
      { id: 2, priority: 50 },
      { id: 3, priority: 100 }
    ])
    expect(assigned).toEqual([
      { id: 1, nextPriority: 50 },
      { id: 2, nextPriority: 80 },
      { id: 3, nextPriority: 100 }
    ])
  })

  it('spreads duplicate slots so the written order is total', () => {
    const assigned = assignPoolAutoSortPriorities([
      { id: 1, priority: 0 },
      { id: 2, priority: 0 },
      { id: 3, priority: 0 }
    ])
    expect(assigned.map((row) => row.nextPriority)).toEqual([0, 1, 2])
  })
})

describe('nextPriorityForPoolMoveToTop', () => {
  it('after auto-sort slots, a later row gets a priority strictly below the pool min', () => {
    expect(nextPriorityForPoolMoveToTop([0, 1, 2], 2)).toBe(-1)
    expect(nextPriorityForPoolMoveToTop([0, 1, 2], 1)).toBe(-1)
  })

  it('before auto-sort, a tied default priority still becomes uniquely first', () => {
    expect(nextPriorityForPoolMoveToTop([50, 50, 50], 50)).toBe(49)
  })

  it('skips the write when the row is already the unique pool min', () => {
    expect(nextPriorityForPoolMoveToTop([0, 1, 2], 0)).toBeNull()
    expect(nextPriorityForPoolMoveToTop([7], 7)).toBeNull()
  })
})
