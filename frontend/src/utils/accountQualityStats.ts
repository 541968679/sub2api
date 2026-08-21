import type { AccountQualityStats } from '@/api/admin/accounts'

export type QualityRateStats = Pick<
  AccountQualityStats,
  | 'success_count'
  | 'error_count'
  | 'success_rate'
  | 'error_rate'
  | 'bridge_success_count'
  | 'bridge_error_count'
  | 'bridge_error_rate'
  | 'terminal_error_count'
  | 'terminal_error_rate'
  | 'failover_error_count'
  | 'failover_error_rate'
>

/** success + error rows in the last-N account-quality window. */
export function qualityRateSampleCount(stats: QualityRateStats | null | undefined): number {
  if (!stats) return 0
  return Math.max(0, (stats.success_count ?? 0) + (stats.error_count ?? 0))
}

/**
 * List cells may show a percentage only after a completed usage_log exists.
 * Empty windows and error-only windows (just-enabled / no traffic) must not
 * render as 0%. Gate math still uses success+error on the backend.
 */
export function hasDisplayableQualityRate(
  stats: QualityRateStats | null | undefined,
  minSamples = 1
): boolean {
  const needed = minSamples > 1 ? minSamples : 1
  if (qualityRateSampleCount(stats) < needed) return false
  return (stats?.success_count ?? 0) > 0
}

function ratioOrNull(
  explicit: number | null | undefined,
  numerator: number,
  samples: number
): number | null {
  if (explicit != null && Number.isFinite(explicit)) return explicit
  if (samples <= 0) return null
  return numerator / samples
}

export function formatQualitySuccessRate(
  stats: QualityRateStats | null | undefined,
  minSamples = 1
): string | null {
  if (!hasDisplayableQualityRate(stats, minSamples) || !stats) return null
  const value = ratioOrNull(stats.success_rate, stats.success_count ?? 0, qualityRateSampleCount(stats))
  if (value == null) return null
  return `${(value * 100).toFixed(1)}%`
}

export function formatQualityErrorRate(
  stats: QualityRateStats | null | undefined,
  minSamples = 1
): string | null {
  if (!hasDisplayableQualityRate(stats, minSamples) || !stats) return null
  const value = ratioOrNull(stats.error_rate, stats.error_count ?? 0, qualityRateSampleCount(stats))
  if (value == null) return null
  return `${(value * 100).toFixed(1)}%`
}

export function qualityBridgeSampleCount(stats: QualityRateStats | null | undefined): number {
  if (!stats) return 0
  return Math.max(0, (stats.bridge_success_count ?? 0) + (stats.bridge_error_count ?? 0))
}

export function hasDisplayableBridgeErrorRate(
  stats: QualityRateStats | null | undefined,
  minSamples = 1
): boolean {
  const needed = minSamples > 1 ? minSamples : 1
  return qualityBridgeSampleCount(stats) >= needed
}

export function formatQualityTerminalErrorRate(
  stats: QualityRateStats | null | undefined,
  minSamples = 1
): string | null {
  return formatPercent(
    namedErrorRateValue(
      stats,
      stats?.terminal_error_count ?? stats?.error_count ?? 0,
      stats?.terminal_error_rate,
      minSamples
    )
  )
}

function namedErrorRateValue(
  stats: QualityRateStats | null | undefined,
  errorCount: number,
  explicit: number | null | undefined,
  minSamples = 1
): number | null {
  if (!stats) return null
  const samples = Math.max(0, (stats.success_count ?? 0) + errorCount)
  if (samples < (minSamples > 1 ? minSamples : 1)) return null
  return ratioOrNull(explicit, errorCount, samples)
}

function formatPercent(value: number | null): string | null {
  if (value == null) return null
  return `${(value * 100).toFixed(1)}%`
}

export function qualityFailoverErrorRateValue(
  stats: QualityRateStats | null | undefined,
  minSamples = 1
): number | null {
  if (stats?.failover_error_count == null) return null
  return namedErrorRateValue(stats, stats.failover_error_count, stats.failover_error_rate, minSamples)
}

/** Display-only: 1 - failover_error_rate. Same samples as the dialog error rate. */
export function qualityFailoverSuccessRateValue(
  stats: QualityRateStats | null | undefined,
  minSamples = 1
): number | null {
  const errorRate = qualityFailoverErrorRateValue(stats, minSamples)
  if (errorRate == null) return null
  return 1 - errorRate
}

export function formatQualityFailoverErrorRate(
  stats: QualityRateStats | null | undefined,
  minSamples = 1
): string | null {
  return formatPercent(qualityFailoverErrorRateValue(stats, minSamples))
}

export function formatQualityFailoverSuccessRate(
  stats: QualityRateStats | null | undefined,
  minSamples = 1
): string | null {
  return formatPercent(qualityFailoverSuccessRateValue(stats, minSamples))
}

export function formatQualityBridgeErrorRate(
  stats: QualityRateStats | null | undefined,
  minSamples = 1
): string | null {
  if (!hasDisplayableBridgeErrorRate(stats, minSamples) || !stats) return null
  const value = ratioOrNull(
    stats.bridge_error_rate,
    stats.bridge_error_count ?? 0,
    qualityBridgeSampleCount(stats)
  )
  if (value == null) return null
  return `${(value * 100).toFixed(1)}%`
}
