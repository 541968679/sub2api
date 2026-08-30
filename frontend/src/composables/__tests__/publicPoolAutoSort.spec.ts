import { describe, expect, it } from 'vitest'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'
import {
  assignPublicPoolReservedBand,
  isAccountListPinned,
  mapPublicPoolAccountToAutoSortItem,
  publicPoolAdmissionFromAccount,
  publicPoolReservedBandUnchanged,
  sortPublicPoolAutoSortItems,
  writablePublicPoolAutoSortIds,
  type PublicPoolAccountLike,
  type PublicPoolQualityLike
} from '../publicPoolAutoSort'

function account(
  overrides: Partial<PublicPoolAccountLike> & Pick<PublicPoolAccountLike, 'id'>
): PublicPoolAccountLike {
  return {
    status: 'active',
    schedulable: true,
    public_schedulable: true,
    fallback_only: false,
    concurrency: 1,
    current_concurrency: 0,
    priority: 10,
    type: 'apikey',
    upstream_rate_multiplier: 0.15,
    extra: {},
    ...overrides
  }
}

function view(overrides: PublicPoolQualityLike = { state: 'selectable', will_cool: false }): PublicPoolQualityLike {
  return overrides
}

function sortIds(
  rows: Array<{ account: PublicPoolAccountLike; view?: PublicPoolQualityLike }>
): number[] {
  return sortPublicPoolAutoSortItems(
    rows.map((row) => mapPublicPoolAccountToAutoSortItem(row.account, row.view))
  ).map((item) => item.id)
}

describe('publicPoolAutoSort AC8', () => {
  it('puts stopped last', () => {
    expect(
      sortIds([
        { account: account({ id: 2, status: 'disabled' }) },
        { account: account({ id: 1 }), view: view() }
      ])
    ).toEqual([1, 2])
  })

  it('lets an expensive schedulable account beat a cheap stopped account', () => {
    expect(
      sortIds([
        { account: account({ id: 2, status: 'disabled', upstream_rate_multiplier: 0.01 }) },
        { account: account({ id: 1, upstream_rate_multiplier: 9 }), view: view() }
      ])
    ).toEqual([1, 2])
  })

  it('ranks a producing account above an idle peer with a better priority', () => {
    expect(
      sortIds([
        {
          account: account({
            id: 2,
            concurrency: 4,
            current_concurrency: 0,
            priority: 1,
            upstream_rate_multiplier: 0.15
          }),
          view: view()
        },
        {
          account: account({
            id: 1,
            concurrency: 4,
            current_concurrency: 1,
            priority: 99,
            upstream_rate_multiplier: 0.15
          }),
          view: view()
        }
      ])
    ).toEqual([1, 2])
  })

  it('puts the lower upstream rate first in the same band', () => {
    expect(
      sortIds([
        { account: account({ id: 2, upstream_rate_multiplier: 0.5 }), view: view() },
        { account: account({ id: 1, upstream_rate_multiplier: 0.1 }), view: view() }
      ])
    ).toEqual([1, 2])
  })

  it('puts fallback_only later in the same admission band', () => {
    expect(
      sortIds([
        { account: account({ id: 2, fallback_only: true, upstream_rate_multiplier: 0.05 }), view: view() },
        { account: account({ id: 1, fallback_only: false, upstream_rate_multiplier: 0.9 }), view: view() }
      ])
    ).toEqual([1, 2])
  })

  it('omits pinned ids from the writable list', () => {
    const pinned = account({ id: 9, extra: { list_pinned: 'true', list_order: 9_000_000_000_000 } })
    const rows = [account({ id: 1 }), pinned, account({ id: 2 })]
    const sorted = sortPublicPoolAutoSortItems(
      rows.map((row) => mapPublicPoolAccountToAutoSortItem(row, view()))
    )
    const pinnedIds = rows.filter((row) => isAccountListPinned(row.extra)).map((row) => row.id)
    expect(writablePublicPoolAutoSortIds(sorted, pinnedIds)).toEqual(
      sorted.filter((item) => item.id !== 9).map((item) => item.id)
    )
    expect(isAccountListPinned({ list_pinned: true })).toBe(true)
    expect(isAccountListPinned({ list_pinned: 'true' })).toBe(true)
    expect(isAccountListPinned({ list_order: 1 })).toBe(false)
  })

  it('maps public availability, concurrency-full, and will_cool like the six-state cell', () => {
    expect(publicPoolAdmissionFromAccount(account({ id: 1, public_schedulable: false }), view())).toBe(
      'stopped'
    )
    expect(
      publicPoolAdmissionFromAccount(
        account({ id: 2, concurrency: 1, current_concurrency: 1 }),
        view()
      )
    ).toBe('pair_full')
    expect(
      publicPoolAdmissionFromAccount(account({ id: 3 }), view({ state: 'selectable', will_cool: true }))
    ).toBe('will_cool')
    expect(
      publicPoolAdmissionFromAccount(account({ id: 4 }), view({ state: 'cooling', will_cool: false }))
    ).toBe('cooling')
  })

  it('assigns reserved-band N…1 and detects an already-ordered set', () => {
    const assigned = assignPublicPoolReservedBand([3, 1, 2])
    expect(assigned).toEqual([
      { id: 3, listOrder: 3 },
      { id: 1, listOrder: 2 },
      { id: 2, listOrder: 1 }
    ])
    expect(
      publicPoolReservedBandUnchanged(
        [
          { id: 3, listOrder: 3 },
          { id: 1, listOrder: 2 },
          { id: 2, listOrder: 1 }
        ],
        assigned
      )
    ).toBe(true)
    expect(
      publicPoolReservedBandUnchanged(
        [
          { id: 3, listOrder: 1 },
          { id: 1, listOrder: 2 },
          { id: 2, listOrder: 3 }
        ],
        assigned
      )
    ).toBe(false)
  })
})

describe('public pool auto-sort copy', () => {
  it('says list order, not scheduling priority, in both locales', () => {
    expect(zh.admin.accounts.autoSortHint).toContain('账号管理列表顺序')
    expect(zh.admin.accounts.autoSortHint).toContain('不改调度优先级')
    expect(zh.admin.accounts.autoSortHint).toContain('不改热路径选号')
    expect(en.admin.accounts.autoSortHint).toContain('account management list order')
    expect(en.admin.accounts.autoSortHint).toContain('Does not change scheduling priority')
    expect(en.admin.accounts.autoSortHint).toContain('Does not change hot-path selection')
  })
})
