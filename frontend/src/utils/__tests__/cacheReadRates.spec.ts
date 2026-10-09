import { describe, expect, it } from 'vitest'
import type { TrendDataPoint } from '@/types'
import {
  buildStabilityCacheRateSeries,
  cacheReadTokenPercent,
  cacheRequestHitPercent,
  formatCachePercent,
  formatRateFraction
} from '../cacheReadRates'

function point(partial: Partial<TrendDataPoint>): TrendDataPoint {
  return {
    date: '2026-10-08 11:00',
    requests: 0,
    input_tokens: 0,
    output_tokens: 0,
    cache_creation_tokens: 0,
    cache_read_tokens: 0,
    total_tokens: 0,
    cost: 0,
    actual_cost: 0,
    ...partial
  }
}

describe('cache read rates', () => {
  it('uses request counts for the hit rate and all tokens for the read share', () => {
    expect(cacheRequestHitPercent(1, 4)).toBe(25)
    expect(cacheReadTokenPercent(40, 165)).toBeCloseTo((40 / 165) * 100)
    expect(formatCachePercent(25)).toBe('25.0%')
    expect(formatCachePercent((40 / 165) * 100)).toBe('24.2%')
    expect(formatRateFraction(1, 4)).toBe('1 / 4')
    expect(formatRateFraction(40, 165)).toBe('40 / 165')
  })

  it('keeps a real zero and hides a zero denominator', () => {
    expect(cacheRequestHitPercent(0, 4)).toBe(0)
    expect(formatCachePercent(0)).toBe('0%')
    expect(cacheRequestHitPercent(0, 0)).toBeNull()
    expect(cacheRequestHitPercent(null, 4)).toBeNull()
    expect(cacheReadTokenPercent(0, 10)).toBe(0)
    expect(cacheReadTokenPercent(5, 0)).toBeNull()
    expect(formatCachePercent(null)).toBe('—')
    expect(formatRateFraction(0, 0)).toBeNull()
    expect(formatCachePercent(0.01)).toBe('0.010%')
  })

  it('builds both percent series without turning cache read into a volume', () => {
    const series = buildStabilityCacheRateSeries([
      point({
        requests: 4,
        cache_hit_requests: 1,
        input_tokens: 100,
        output_tokens: 20,
        cache_creation_tokens: 5,
        cache_read_tokens: 40,
        total_tokens: 165
      }),
      point({ requests: 0, cache_hit_requests: 0 }),
      point({
        requests: 3,
        input_tokens: 10,
        cache_read_tokens: 0,
        total_tokens: 10
      })
    ])

    expect(series.input).toEqual([100, 0, 10])
    expect(series.output).toEqual([20, 0, 0])
    expect(series.cacheCreation).toEqual([5, 0, 0])
    expect(series.requestRate[0]).toBe(25)
    expect(series.requestRate[1]).toBeNull()
    expect(series.requestRate[2]).toBeNull()
    expect(series.tokenRatio[0]).toBeCloseTo((40 / 165) * 100)
    expect(series.tokenRatio[1]).toBeNull()
    expect(series.tokenRatio[2]).toBe(0)
    expect(series).not.toHaveProperty('cacheRead')
  })
})
