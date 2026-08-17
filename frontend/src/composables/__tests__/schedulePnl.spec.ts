import { describe, expect, it } from 'vitest'
import {
  formatSchedulePnlMargin,
  formatSchedulePnlUsd,
  formatSchedulePnlUsdPlain,
  formatSchedulePnlWindow,
  hasSchedulePnlSummary,
  pairAccountBalanceUsd
} from '../schedulePnl'

describe('schedulePnl formatters', () => {
  it('renders margin as dash when revenue is zero / null', () => {
    expect(formatSchedulePnlMargin(null)).toBe('—')
    expect(formatSchedulePnlMargin(undefined)).toBe('—')
    expect(formatSchedulePnlMargin(0.75)).toBe('75.0%')
  })

  it('formats usd and treats empty summary as empty', () => {
    expect(formatSchedulePnlUsdPlain(1.2)).toBe('$1.20')
    expect(formatSchedulePnlUsd(-0.5)).toBe('-$0.50')
    expect(hasSchedulePnlSummary({ today: null, seven_day: null })).toBe(false)
    expect(hasSchedulePnlSummary({ today: { revenue: 0, cost: 1, profit: -1, margin: null }, seven_day: null })).toBe(true)
  })

  it('formats a window with revenue, cost, profit, and dash margin', () => {
    expect(formatSchedulePnlWindow({ revenue: 1.2, cost: 0.3, profit: 0.9, margin: 0.75 })).toEqual({
      revenue: '$1.20',
      cost: '$0.30',
      profit: '+$0.90',
      margin: '75.0%'
    })
    expect(formatSchedulePnlWindow({ revenue: 0, cost: 1, profit: -1, margin: null })).toEqual({
      revenue: '$0.00',
      cost: '$1.00',
      profit: '-$1.00',
      margin: '—'
    })
    expect(formatSchedulePnlWindow(null).profit).toBe('—')
  })

  it('reads an existing account balance field only', () => {
    expect(pairAccountBalanceUsd({ usage: { balance_usd: 12.5 } })).toBe(12.5)
    expect(pairAccountBalanceUsd({ balance_usd: 3 })).toBe(3)
    expect(pairAccountBalanceUsd({ quota_limit: 100 } as never)).toBeNull()
  })
})
