import { describe, expect, it } from 'vitest'

import { durationSeverity, firstTokenSeverity, migrateLatencyHiddenColumns, tokensPerSecond } from '../latencyHealth'

describe('latencyHealth', () => {
  it('classifies first-token latency at 10s/30s/60s boundaries', () => {
    expect(firstTokenSeverity(9_999)).toBe('good')
    expect(firstTokenSeverity(10_000)).toBe('warn')
    expect(firstTokenSeverity(30_000)).toBe('slow')
    expect(firstTokenSeverity(60_000)).toBe('critical')
  })

  it('classifies total duration at 1min/3min/5min boundaries', () => {
    expect(durationSeverity(59_999)).toBe('good')
    expect(durationSeverity(60_000)).toBe('warn')
    expect(durationSeverity(180_000)).toBe('slow')
    expect(durationSeverity(300_000)).toBe('critical')
  })

  it('rounds input and output tokens per second from the whole request duration', () => {
    expect(tokensPerSecond(101, 2000)).toBe(51)
    expect(tokensPerSecond(4057, 10_000)).toBe(406)
    expect(tokensPerSecond(0, 2000)).toBeNull()
    expect(tokensPerSecond(10, 0)).toBeNull()
  })

  it('migrates persisted legacy latency columns without dropping unrelated preferences', () => {
    expect(migrateLatencyHiddenColumns(['first_token', 'user_agent'])).toEqual(['latency', 'user_agent'])
    expect(migrateLatencyHiddenColumns(['duration', 'first_token'])).toEqual(['latency'])
    expect(migrateLatencyHiddenColumns(['duration'])).toEqual(['latency'])
    expect(migrateLatencyHiddenColumns(['cost'])).toEqual(['cost'])
  })
})
