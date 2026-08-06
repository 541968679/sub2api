import { describe, expect, it } from 'vitest'
import {
  clampColumnWidth,
  mergeAccountColumnOrder,
  moveAccountColumnOrder,
  parseAccountColumnLayout
} from '../accountColumnLayout'

const ALL_KEYS = [
  'select',
  'name',
  'status',
  'schedulable',
  'priority',
  'proxy',
  'actions'
]

describe('accountColumnLayout', () => {
  it('merges order with pinned select/name first and actions last', () => {
    const order = mergeAccountColumnOrder(['priority', 'schedulable', 'status'], ALL_KEYS)
    expect(order[0]).toBe('select')
    expect(order[1]).toBe('name')
    expect(order[order.length - 1]).toBe('actions')
    expect(order.indexOf('priority')).toBeLessThan(order.indexOf('schedulable'))
    expect(order).toContain('proxy')
  })

  it('moves priority next to schedulable via up/down', () => {
    let order = mergeAccountColumnOrder(null, ALL_KEYS)
    // default: ... status, schedulable, priority, proxy ...
    order = moveAccountColumnOrder(order, 'priority', 'up')
    // still after schedulable if priority was right after? default middle: status, schedulable, priority, proxy
    // move priority up -> status, priority, schedulable, proxy
    expect(order.indexOf('priority')).toBe(order.indexOf('schedulable') - 1)
    order = moveAccountColumnOrder(order, 'priority', 'down')
    // back: status, schedulable, priority
    expect(order.indexOf('priority')).toBe(order.indexOf('schedulable') + 1)
  })

  it('does not move pinned columns', () => {
    const base = mergeAccountColumnOrder(null, ALL_KEYS)
    expect(moveAccountColumnOrder(base, 'name', 'down')).toEqual(base)
    expect(moveAccountColumnOrder(base, 'actions', 'up')).toEqual(base)
  })

  it('clamps column widths', () => {
    expect(clampColumnWidth('name', 10)).toBe(64)
    expect(clampColumnWidth('select', 10)).toBe(40)
    expect(clampColumnWidth('name', 9999)).toBe(640)
    expect(clampColumnWidth('name', 120.6)).toBe(121)
  })

  it('parses layout JSON and drops unknown keys', () => {
    const raw = JSON.stringify({
      version: 1,
      order: ['priority', 'ghost', 'schedulable'],
      widths: { priority: 150, ghost: 90, name: 200 }
    })
    const layout = parseAccountColumnLayout(raw, ALL_KEYS)
    expect(layout.order).toContain('priority')
    expect(layout.order).not.toContain('ghost')
    expect(layout.widths.priority).toBe(150)
    expect(layout.widths.name).toBe(200)
    expect(layout.widths.ghost).toBeUndefined()
  })
})
