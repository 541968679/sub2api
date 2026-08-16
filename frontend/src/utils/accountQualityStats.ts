import type { AccountQualityStats } from '@/api/admin/accounts'

export type QualityRateStats = Pick<
  AccountQualityStats,
  'success_count' | 'error_count' | 'success_rate' | 'error_rate'
>

/** success + error rows in the 15-minute window. */
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
