import { describe, expect, it } from 'vitest'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'
import {
  UNCAPPED_PAIR_HEADROOM,
  assignPoolAutoSortOrders,
  assignPoolMoveToTopSortOrders,
  comparePoolAutoSort,
  effectivePoolAutoSortRate,
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
    upstreamRate: 0.15,
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
      item({ id: 8, admission: 'probing' }),
      item({ id: 2, admission: 'pair_full' }),
      item({ id: 3, admission: 'cooling' })
    ]
    expect(sortSmartSchedulePoolMembers(rows).map((row) => row.admission)).toEqual([
      'selectable',
      'resumed',
      'probing',
      'will_cool',
      'unsaved_preview',
      'pair_full',
      'cooling',
      'stopped'
    ])
  })

  it('ranks pinned with selectable and resumed', () => {
    const pinned = item({ id: 1, admission: 'pinned' })
    const selectable = item({ id: 2, admission: 'selectable' })
    const cooling = item({ id: 3, admission: 'cooling' })
    expect(sortSmartSchedulePoolMembers([cooling, pinned, selectable]).map((row) => row.admission)).toEqual([
      'pinned',
      'selectable',
      'cooling'
    ])
  })

  it('ranks paused with cooling, above stopped', () => {
    const rows = [
      item({ id: 1, admission: 'paused' }),
      item({ id: 2, admission: 'stopped' }),
      item({ id: 3, admission: 'pair_full' })
    ]
    expect(sortSmartSchedulePoolMembers(rows).map((row) => row.admission)).toEqual([
      'pair_full',
      'paused',
      'stopped'
    ])
  })

  it('reads account priority only as a later tie-break', () => {
    const preferred = item({ id: 2, priority: 1, concurrency: 4 })
    const other = item({ id: 9, priority: 80, concurrency: 4 })
    expect(sortSmartSchedulePoolMembers([other, preferred]).map((row) => row.id)).toEqual([2, 9])
  })

  it('breaks remaining ties by account id', () => {
    const left = item({ id: 20, priority: 7 })
    const right = item({ id: 8, priority: 7 })
    expect(comparePoolAutoSort(left, right)).toBeGreaterThan(0)
    expect(sortSmartSchedulePoolMembers([left, right]).map((row) => row.id)).toEqual([8, 20])
  })

  it('puts this-user producing 0.06 accounts ahead of an idle better-priority 0.08', () => {
    const tokenBits = item({ id: 1606, pairCurrent: 2, upstreamRate: 0.06, priority: 1 })
    const aiwanwu = item({ id: 1689, pairCurrent: 1, upstreamRate: 0.06, priority: 7 })
    const tokenbits08 = item({ id: 1685, pairCurrent: 0, upstreamRate: 0.08, priority: 2 })
    expect(sortSmartSchedulePoolMembers([tokenbits08, aiwanwu, tokenBits]).map((row) => row.id)).toEqual([
      1606, 1689, 1685
    ])
  })

  it('ranks cheaper rate first when nobody is producing', () => {
    const cheap = item({ id: 2, upstreamRate: 0.06, priority: 7 })
    const expensive = item({ id: 3, upstreamRate: 0.08, priority: 1 })
    expect(sortSmartSchedulePoolMembers([expensive, cheap]).map((row) => row.id)).toEqual([2, 3])
  })

  it('lets a producing expensive account beat an idle cheaper peer', () => {
    const idleCheap = item({ id: 6, pairCurrent: 0, upstreamRate: 0.06, priority: 1 })
    const busyExpensive = item({ id: 8, pairCurrent: 3, upstreamRate: 0.08, priority: 9 })
    expect(sortSmartSchedulePoolMembers([idleCheap, busyExpensive]).map((row) => row.id)).toEqual([8, 6])
  })

  it('among producing accounts, still prefers the cheaper upstream rate', () => {
    const cheap = item({ id: 6, pairCurrent: 1, upstreamRate: 0.06, priority: 9 })
    const expensive = item({ id: 8, pairCurrent: 4, upstreamRate: 0.08, priority: 1 })
    expect(sortSmartSchedulePoolMembers([expensive, cheap]).map((row) => row.id)).toEqual([6, 8])
  })

  it('does not rank a stopped cheaper account ahead of a selectable more expensive peer', () => {
    const stoppedCheap = item({
      id: 1,
      admission: 'stopped',
      pairCurrent: 4,
      upstreamRate: 0.06,
      priority: 1,
      concurrency: 99
    })
    const selectableExpensive = item({
      id: 2,
      admission: 'selectable',
      pairCurrent: 0,
      upstreamRate: 0.08,
      priority: 80,
      concurrency: 1
    })
    expect(sortSmartSchedulePoolMembers([stoppedCheap, selectableExpensive]).map((row) => row.id)).toEqual([
      2, 1
    ])
  })

  it('treats this-user producing as binary, not by occupancy count', () => {
    const heavier = item({ id: 9, pairCurrent: 8, upstreamRate: 0.06, priority: 9 })
    const lighter = item({ id: 2, pairCurrent: 1, upstreamRate: 0.06, priority: 9 })
    expect(sortSmartSchedulePoolMembers([heavier, lighter]).map((row) => row.id)).toEqual([2, 9])
  })
})

describe('effectivePoolAutoSortRate', () => {
  it('keeps an explicit non-negative rate and defaults missing or negative by type', () => {
    expect(effectivePoolAutoSortRate(0.06, 'apikey')).toBe(0.06)
    expect(effectivePoolAutoSortRate(0, 'oauth')).toBe(0)
    expect(effectivePoolAutoSortRate(null, 'apikey')).toBe(0.15)
    expect(effectivePoolAutoSortRate(-1, 'oauth')).toBe(0.15)
    expect(effectivePoolAutoSortRate(undefined, 'setup-token')).toBe(1)
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

describe('autoSortHint', () => {
  it('lists the same new key order in zh and en and says hot-path selection is unchanged', () => {
    expect(zh.admin.users.smartSchedule.autoSortHint).toContain(
      '入池状态 → 本用户是否出量（配对占用>0） → 上游倍率（低优先） → 配对剩余 → 账号并发 → 原账号优先级（只读平局） → id'
    )
    expect(zh.admin.users.smartSchedule.autoSortHint).toContain('不改账号调度优先级')
    expect(zh.admin.users.smartSchedule.autoSortHint).toContain('不改热路径选号')
    expect(en.admin.users.smartSchedule.autoSortHint).toContain(
      'admission → this-user producing (pair occupancy > 0) → upstream rate (lower first) → pair headroom → account concurrency → existing account priority (read-only tie-break) → id'
    )
    expect(en.admin.users.smartSchedule.autoSortHint).toContain('Does not change account scheduling priority')
    expect(en.admin.users.smartSchedule.autoSortHint).toContain('Does not change hot-path selection')
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
