import { describe, expect, it } from 'vitest'
import {
  compareBalanceBurnToCost,
  computeBalanceBurnPerHour,
  formatSchedulePnlMargin,
  formatSchedulePnlUsd,
  formatSchedulePnlUsdPlain,
  formatSchedulePnlWindow,
  hasSchedulePnlSummary,
  impliedCostBurnPerHour,
  isOauthAccountType,
  oauthSevenDayQuota,
  applyUsageBalanceToAccountExtra,
  pairAccountBalanceUsd,
  pairAccountSubscriptionUsd,
  pairAccountWalletUsd,
  shouldRefreshPairBalance,
  supportsPairBalanceProbe
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
    expect(pairAccountBalanceUsd({ extra: { upstream_balance_usd: 8.25 } })).toBe(8.25)
    expect(pairAccountBalanceUsd({ extra: { upstream_balance_usd: '9.5' } })).toBe(9.5)
    expect(pairAccountBalanceUsd({
      extra: {
        upstream_balance_usd: 336.61,
        upstream_balance_wallet_usd: 0,
        upstream_balance_subscription_usd: 336.61
      }
    })).toBe(336.61)
    expect(pairAccountBalanceUsd({ quota_limit: 100 } as never)).toBeNull()
    expect(pairAccountWalletUsd({ extra: { upstream_balance_wallet_usd: 0 } })).toBe(0)
    expect(pairAccountWalletUsd({ usage: { balance_wallet_usd: 12 } })).toBe(12)
    expect(pairAccountSubscriptionUsd({ extra: { upstream_balance_subscription_usd: 336.61 } })).toBe(336.61)
    expect(pairAccountSubscriptionUsd({ extra: { upstream_balance_subscription_usd: 0 } })).toBeNull()
    expect(
      applyUsageBalanceToAccountExtra(
        { upstream_balance_subscription_usd: 100, burn_samples: [] },
        { balance_usd: 12.5, balance_wallet_usd: 12.5 }
      )
    ).toEqual({
      burn_samples: [],
      upstream_balance_usd: 12.5,
      upstream_balance_wallet_usd: 12.5
    })
  })

  it('refreshes api-key balance only when the snapshot is missing or older than 6 minutes', () => {
    const now = new Date('2026-08-22T06:10:00.000Z')
    expect(supportsPairBalanceProbe({ type: 'oauth', platform: 'openai' })).toBe(false)
    expect(supportsPairBalanceProbe({ type: 'apikey', platform: 'gemini' })).toBe(false)
    expect(supportsPairBalanceProbe({ type: 'apikey', platform: 'openai' })).toBe(true)
    expect(shouldRefreshPairBalance({ type: 'apikey', platform: 'openai', extra: {} }, now)).toBe(true)
    expect(
      shouldRefreshPairBalance(
        { type: 'apikey', platform: 'openai', extra: { upstream_balance_at: '2026-08-22T06:08:00.000Z' } },
        now
      )
    ).toBe(false)
    expect(
      shouldRefreshPairBalance(
        { type: 'apikey', platform: 'anthropic', extra: { upstream_balance_at: '2026-08-22T06:04:00.000Z' } },
        now
      )
    ).toBe(true)
    expect(
      shouldRefreshPairBalance(
        { type: 'apikey', platform: 'openai', extra: { upstream_balance_at: '2026-08-22T06:08:00.000Z' } },
        now,
        true
      )
    ).toBe(true)
    expect(
      applyUsageBalanceToAccountExtra({ burn_samples: [] }, {
        balance_usd: 12.5,
        balance_updated_at: '2026-08-22T06:10:00.000Z',
        balance_used_usd: 3
      })
    ).toEqual({
      burn_samples: [],
      upstream_balance_usd: 12.5,
      upstream_balance_at: '2026-08-22T06:10:00.000Z',
      upstream_balance_used_usd: 3,
      display_balance_used_usd: 3
    })
  })

  it('fits balance burn from extra samples and compares it to today billed cost', () => {
    const now = new Date('2026-08-17T12:00:00')
    const samples = [
      { t: '2026-08-17T10:00:00.000Z', v: 100, kind: 'balance_usd' },
      { t: '2026-08-17T11:00:00.000Z', v: 90, kind: 'balance_usd' },
      { t: '2026-08-17T12:00:00.000Z', v: 80, kind: 'balance_usd' }
    ]
    expect(computeBalanceBurnPerHour(samples.map((s) => ({ t: new Date(s.t), v: s.v, kind: s.kind })))).toBeCloseTo(10, 2)
    expect(impliedCostBurnPerHour(120, now)).toBeCloseTo(10, 2)

    const account = { extra: { upstream_balance_usd: 80, burn_samples: samples } }
    expect(compareBalanceBurnToCost(account, 120, now)?.status).toBe('match')
    expect(compareBalanceBurnToCost(account, 12, now)?.status).toBe('mismatch')
    expect(compareBalanceBurnToCost({ extra: { burn_samples: samples } }, 120, now)).toBeNull()
    expect(compareBalanceBurnToCost(account, null, now)).toBeNull()
  })

  it('reads oauth 7-day quota from cached extra only', () => {
    const now = new Date('2026-08-18T12:00:00.000Z')
    expect(isOauthAccountType({ type: 'apikey' })).toBe(false)
    expect(oauthSevenDayQuota({ type: 'apikey', extra: { passive_usage_7d_utilization: 0.4 } })).toBeNull()
    expect(
      oauthSevenDayQuota({
        type: 'oauth',
        platform: 'anthropic',
        extra: { passive_usage_7d_utilization: 0.42, passive_usage_7d_reset: 1787040000 }
      })
    ).toEqual({
      utilization: 42,
      resetsAt: new Date(1787040000 * 1000).toISOString()
    })
    expect(
      oauthSevenDayQuota(
        {
          type: 'oauth',
          platform: 'openai',
          extra: {
            codex_7d_used_percent: 67,
            codex_7d_reset_at: '2026-08-19T12:00:00.000Z'
          }
        },
        now
      )
    ).toEqual({
      utilization: 67,
      resetsAt: '2026-08-19T12:00:00.000Z'
    })
    expect(
      oauthSevenDayQuota(
        {
          type: 'oauth',
          platform: 'openai',
          extra: {
            codex_7d_used_percent: 80,
            codex_7d_reset_at: '2026-08-17T12:00:00.000Z'
          }
        },
        now
      )
    ).toEqual({
      utilization: 0,
      resetsAt: '2026-08-17T12:00:00.000Z'
    })
    expect(oauthSevenDayQuota({ type: 'oauth', platform: 'anthropic', extra: {} })).toBeNull()
  })
})
