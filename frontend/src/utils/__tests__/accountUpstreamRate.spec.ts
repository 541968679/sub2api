import { describe, expect, it } from 'vitest'
import {
  accountMatchesUpstreamRateSelection,
  defaultUpstreamRateMultiplier,
  effectiveUpstreamRateMultiplier
} from '../accountUpstreamRate'

describe('effectiveUpstreamRateMultiplier', () => {
  it('uses type defaults for missing or illegal rates', () => {
    expect(defaultUpstreamRateMultiplier('oauth')).toBe(0.15)
    expect(defaultUpstreamRateMultiplier('apikey')).toBe(0.15)
    expect(defaultUpstreamRateMultiplier('setup-token')).toBe(1)
    expect(effectiveUpstreamRateMultiplier('oauth', undefined)).toBe(0.15)
    expect(effectiveUpstreamRateMultiplier('oauth', null)).toBe(0.15)
    expect(effectiveUpstreamRateMultiplier('oauth', Number.NaN)).toBe(0.15)
    expect(effectiveUpstreamRateMultiplier('oauth', -0.1)).toBe(0.15)
    expect(effectiveUpstreamRateMultiplier('setup-token', undefined)).toBe(1)
  })

  it('keeps an explicit zero and does not fall back to 1 for oauth', () => {
    expect(effectiveUpstreamRateMultiplier('oauth', 0)).toBe(0)
    expect(effectiveUpstreamRateMultiplier('oauth', 0.15)).toBe(0.15)
    expect(effectiveUpstreamRateMultiplier('oauth', 1.2)).toBe(1.2)
  })
})

describe('accountMatchesUpstreamRateSelection', () => {
  it('selects oauth default 0.15 as below 1 and not equal or above', () => {
    const oauthUnset = { type: 'oauth' }
    expect(accountMatchesUpstreamRateSelection(oauthUnset, 'lt', 1)).toBe(true)
    expect(accountMatchesUpstreamRateSelection(oauthUnset, 'gt', 1)).toBe(false)
    expect(accountMatchesUpstreamRateSelection({ type: 'oauth', upstream_rate_multiplier: 1 }, 'lt', 1)).toBe(false)
    expect(accountMatchesUpstreamRateSelection({ type: 'oauth', upstream_rate_multiplier: 1 }, 'gt', 1)).toBe(false)
  })

  it('selects only strictly greater rates and ignores billing-like 0.5 when upstream is 1.2', () => {
    const highUpstream = { type: 'oauth', upstream_rate_multiplier: 1.2, rate_multiplier: 0.5 }
    expect(accountMatchesUpstreamRateSelection(highUpstream, 'gt', 1)).toBe(true)
    expect(accountMatchesUpstreamRateSelection(highUpstream, 'lt', 1)).toBe(false)
    expect(accountMatchesUpstreamRateSelection({ type: 'setup-token', upstream_rate_multiplier: 1 }, 'gt', 1)).toBe(false)
  })

  it('rejects a non-finite threshold', () => {
    expect(accountMatchesUpstreamRateSelection({ type: 'oauth' }, 'lt', Number.NaN)).toBe(false)
  })
})
