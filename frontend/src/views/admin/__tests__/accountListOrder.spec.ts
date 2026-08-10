import { describe, expect, it } from 'vitest'
import { moveAccountInPageList } from '../accountListOrder'

describe('moveAccountInPageList', () => {
  const page = [{ id: 1 }, { id: 2 }, { id: 3 }]

  it('moves an item before another', () => {
    expect(moveAccountInPageList(page, 3, 1).map((a) => a.id)).toEqual([3, 1, 2])
  })

  it('moves an item after another', () => {
    expect(moveAccountInPageList(page, 1, 3).map((a) => a.id)).toEqual([2, 3, 1])
  })

  it('no-ops for same id or missing id', () => {
    expect(moveAccountInPageList(page, 1, 1)).toBe(page)
    expect(moveAccountInPageList(page, 9, 1)).toBe(page)
  })
})
