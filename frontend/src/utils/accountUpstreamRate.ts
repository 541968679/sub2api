export type UpstreamRateComparison = 'gt' | 'lt'

export function defaultUpstreamRateMultiplier(type: string): number {
  return type === 'oauth' || type === 'apikey' ? 0.15 : 1
}

/** Match backend Account.EffectiveUpstreamRate(): nil / non-finite / negative → type default. */
export function effectiveUpstreamRateMultiplier(
  type: string | undefined | null,
  rate: number | null | undefined
): number {
  if (rate == null || !Number.isFinite(rate) || rate < 0) {
    return defaultUpstreamRateMultiplier(type ?? '')
  }
  return rate
}

export function accountMatchesUpstreamRateSelection(
  account: { type?: string; upstream_rate_multiplier?: number | null },
  comparison: UpstreamRateComparison,
  threshold: number
): boolean {
  if (!Number.isFinite(threshold)) return false
  const rate = effectiveUpstreamRateMultiplier(account.type, account.upstream_rate_multiplier)
  return comparison === 'gt' ? rate > threshold : rate < threshold
}
