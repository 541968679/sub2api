import { describe, expect, it } from 'vitest'
import { compareAdminGroupsBySort, mergeGroupSortOrders } from '../groupSortOrder'

describe('mergeGroupSortOrders', () => {
  it('sorts missing sort_order as 0, then by id', () => {
    const groups = [
      { id: 3, sort_order: 0 },
      { id: 1 },
      { id: 2, sort_order: 5 }
    ]
    expect([...groups].sort(compareAdminGroupsBySort).map((group) => group.id)).toEqual([1, 3, 2])
  })

  it('splices one subset into its old slots and renumbers every group', () => {
    const updates = mergeGroupSortOrders(
      [
        { id: 1, sort_order: 0 },
        { id: 2, sort_order: 0 },
        { id: 3, sort_order: 0 },
        { id: 4, sort_order: 0 }
      ],
      [[4, 2]]
    )
    expect(updates).toEqual([
      { id: 1, sort_order: 10 },
      { id: 4, sort_order: 20 },
      { id: 3, sort_order: 30 },
      { id: 2, sort_order: 40 }
    ])
  })

  it('applies disjoint dropdown lists without mixing their members', () => {
    const updates = mergeGroupSortOrders(
      [
        { id: 9, sort_order: 0 },
        { id: 23, sort_order: 0 },
        { id: 46, sort_order: 0 },
        { id: 48, sort_order: 0 }
      ],
      [
        [23, 9],
        [48, 46]
      ]
    )
    expect(updates.map((item) => item.id)).toEqual([23, 9, 48, 46])
  })

  it('starts from sort_order, not the input array order', () => {
    const updates = mergeGroupSortOrders(
      [
        { id: 3, sort_order: 20 },
        { id: 1, sort_order: 5 },
        { id: 2, sort_order: 5 }
      ],
      [[2, 1]]
    )
    expect(updates.map((item) => item.id)).toEqual([2, 1, 3])
  })

  it('ignores ids that are not in the loaded groups', () => {
    const updates = mergeGroupSortOrders([{ id: 1, sort_order: 0 }, { id: 2, sort_order: 0 }], [[2, 99, 1]])
    expect(updates.map((item) => item.id)).toEqual([2, 1])
  })
})
