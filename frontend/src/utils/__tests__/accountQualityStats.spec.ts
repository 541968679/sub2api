import { describe, expect, it } from 'vitest'
import {
  formatQualityBridgeErrorRate,
  formatQualityErrorRate,
  formatQualityFailoverErrorRate,
  formatQualitySuccessRate,
  formatQualityTerminalErrorRate,
  hasDisplayableBridgeErrorRate,
  hasDisplayableQualityRate,
  qualityBridgeSampleCount,
  qualityRateSampleCount
} from '@/utils/accountQualityStats'

const emptyZeroRate = {
  success_count: 0,
  error_count: 0,
  success_rate: 0,
  error_rate: 0
}

const errorOnly = {
  success_count: 0,
  error_count: 33,
  success_rate: 0,
  error_rate: 1
}

const mixed = {
  success_count: 10,
  error_count: 1,
  success_rate: 10 / 11,
  error_rate: 1 / 11
}

describe('accountQualityStats rate display', () => {
  it('treats an empty window as no samples even when JSON emits 0', () => {
    expect(qualityRateSampleCount(emptyZeroRate)).toBe(0)
    expect(hasDisplayableQualityRate(emptyZeroRate)).toBe(false)
    expect(formatQualitySuccessRate(emptyZeroRate)).toBeNull()
    expect(formatQualityErrorRate(emptyZeroRate)).toBeNull()
  })

  it('does not render error-only windows as 0% (just-enabled / no usage_logs)', () => {
    expect(qualityRateSampleCount(errorOnly)).toBe(33)
    expect(hasDisplayableQualityRate(errorOnly)).toBe(false)
    expect(formatQualitySuccessRate(errorOnly)).toBeNull()
    expect(formatQualityErrorRate(errorOnly)).toBeNull()
  })

  it('hides rates below the min sample threshold', () => {
    expect(hasDisplayableQualityRate(mixed, 20)).toBe(false)
    expect(formatQualitySuccessRate(mixed, 20)).toBeNull()
    expect(formatQualityErrorRate(mixed, 20)).toBeNull()
  })

  it('formats success and error rates once a completed sample exists', () => {
    expect(hasDisplayableQualityRate(mixed)).toBe(true)
    expect(formatQualitySuccessRate(mixed)).toBe('90.9%')
    expect(formatQualityErrorRate(mixed)).toBe('9.1%')
  })

  it('formats a separate bridge error rate and hides empty bridge windows', () => {
    expect(qualityBridgeSampleCount(mixed)).toBe(0)
    expect(hasDisplayableBridgeErrorRate(mixed)).toBe(false)
    expect(formatQualityBridgeErrorRate(mixed)).toBeNull()

    const bridge = {
      ...mixed,
      bridge_success_count: 4,
      bridge_error_count: 6,
      bridge_error_rate: 0.6
    }
    expect(qualityBridgeSampleCount(bridge)).toBe(10)
    expect(hasDisplayableBridgeErrorRate(bridge)).toBe(true)
    expect(formatQualityBridgeErrorRate(bridge)).toBe('60.0%')
  })

  it('keeps success-rate display when failover_error_count is omitted', () => {
    expect(hasDisplayableQualityRate(mixed)).toBe(true)
    expect(formatQualitySuccessRate(mixed)).toBe('90.9%')
    expect(formatQualityErrorRate(mixed)).toBe('9.1%')
    expect(formatQualityFailoverErrorRate(mixed)).toBeNull()
  })

  it('formats terminal vs failover account error rates without inventing a missing failover window', () => {
    expect(formatQualityTerminalErrorRate(mixed)).toBe('9.1%')
    expect(formatQualityFailoverErrorRate(mixed)).toBeNull()

    const dual = {
      ...mixed,
      terminal_error_count: 5,
      terminal_error_rate: 5 / 15,
      failover_error_count: 20,
      failover_error_rate: 20 / 30
    }
    expect(formatQualityTerminalErrorRate(dual)).toBe('33.3%')
    expect(formatQualityFailoverErrorRate(dual)).toBe('66.7%')
  })
})
