import { describe, expect, it } from 'vitest'
import {
  formatAccountBurnRateAmount,
  moneyBurnRateFromUsage,
  sumAccountBalanceBurnRates
} from '../accountBurnRateTotal'

const now = new Date('2026-10-12T02:00:00.000Z')
const at = (hoursAgo: number, value: number, kind: string) => ({
  t: new Date(now.getTime() - hoursAgo * 3_600_000).toISOString(),
  v: value,
  kind
})
const spanHours = 0.125
const usdSamples = (startValue: number, dropPerHour: number) => [
  at(spanHours, startValue, 'balance_usd'),
  at(0, startValue - dropPerHour * spanHours, 'balance_usd')
]

describe('sumAccountBalanceBurnRates', () => {
  it('adds the displayed balance burn rates into one total and skips rows that cannot be summed', () => {
    const total = sumAccountBalanceBurnRates(
      [
        { extra: { burn_samples: usdSamples(100, 10) } },
        { extra: { burn_samples: usdSamples(50, 4) } },
        {
          extra: {
            upstream_balance_currency: 'CNY',
            burn_samples: [at(spanHours, 80, 'balance_cny'), at(0, 80 - 2 * spanHours, 'balance_cny')]
          }
        },
        { extra: { burn_samples: [at(0, 10, 'balance_usd')] } },
        {
          extra: {
            upstream_balance_unlimited: true,
            burn_samples: usdSamples(20, 5)
          }
        },
        {
          extra: {
            burn_samples: [at(spanHours, 40, 'oauth_7d_remaining_pct'), at(0, 30, 'oauth_7d_remaining_pct')]
          }
        },
        {
          extra: {
            burn_samples: [at(48, 100, 'balance_usd'), at(24, 50, 'balance_usd')]
          }
        }
      ],
      now
    )

    expect(total.perHour).toBeCloseTo(16, 2)
    expect(total.accountCount).toBe(3)
    expect(total.visible).toBe(true)
    expect(formatAccountBurnRateAmount(total.perHour)).toBe('16.0')
  })

  it('stays hidden when no account has a usable balance burn rate', () => {
    expect(sumAccountBalanceBurnRates([])).toMatchObject({ visible: false, accountCount: 0 })
    expect(sumAccountBalanceBurnRates([{ extra: {} }]).visible).toBe(false)
  })

  it('uses the money rate a usage cell is already showing', () => {
    const displayed = new Map<number, number | null>([
      [1, 4],
      [2, null]
    ])
    const total = sumAccountBalanceBurnRates(
      [
        { id: 1, extra: { burn_samples: usdSamples(100, 10) } },
        { id: 2, extra: { burn_samples: usdSamples(50, 4) } },
        { id: 3, extra: {} }
      ],
      now,
      displayed
    )

    expect(total.perHour).toBe(4)
    expect(total.accountCount).toBe(1)
    expect(moneyBurnRateFromUsage({ burn_rate_per_hour: 1.2, burn_rate_unit: 'usd' })).toBe(1.2)
    expect(moneyBurnRateFromUsage({ burn_rate_per_hour: 8, burn_rate_unit: 'percent' })).toBeNull()
    expect(moneyBurnRateFromUsage({ burn_rate_per_hour: 3, burn_rate_unit: 'usd', burn_insufficient: true })).toBeNull()
  })
})
