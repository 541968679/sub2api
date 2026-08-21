/** Account-global quality (Q_a) last-N window. Not smart-schedule pair N. */
export const ACCOUNT_QUALITY_WINDOW_N_DEFAULT = 20
export const ACCOUNT_QUALITY_WINDOW_N_MIN = 1
export const ACCOUNT_QUALITY_WINDOW_N_MAX = 100

export type AccountQualityWindowNInput = {
  account_quality_window_n?: number | null
  window_n?: number | null
  n?: number | null
  min_success_samples?: number | null
  min_ttft_samples?: number | null
  quality_min_success_samples?: number | null
  quality_min_ttft_samples?: number | null
}

function finiteNumber(value: unknown): number | null {
  if (typeof value !== 'number' || !Number.isFinite(value)) return null
  return value
}

export function clampAccountQualityWindowN(value: number | null | undefined): number {
  if (value == null || !Number.isFinite(value)) return ACCOUNT_QUALITY_WINDOW_N_DEFAULT
  return Math.min(
    ACCOUNT_QUALITY_WINDOW_N_MAX,
    Math.max(ACCOUNT_QUALITY_WINDOW_N_MIN, Math.round(value))
  )
}

/**
 * Prefer the explicit account-quality N. Legacy dual min-sample fields
 * fall back to success, then TTFT, then default 20 (do not take min(20,10)).
 */
export function resolveAccountQualityWindowN(input: AccountQualityWindowNInput | null | undefined): number {
  if (!input) return ACCOUNT_QUALITY_WINDOW_N_DEFAULT
  const explicit = finiteNumber(input.account_quality_window_n)
    ?? finiteNumber(input.window_n)
    ?? finiteNumber(input.n)
  if (explicit != null) return clampAccountQualityWindowN(explicit)

  const success = finiteNumber(input.min_success_samples)
    ?? finiteNumber(input.quality_min_success_samples)
  if (success != null) return clampAccountQualityWindowN(success)

  const ttft = finiteNumber(input.min_ttft_samples)
    ?? finiteNumber(input.quality_min_ttft_samples)
  if (ttft != null) return clampAccountQualityWindowN(ttft)

  return ACCOUNT_QUALITY_WINDOW_N_DEFAULT
}

/** Write the canonical N plus both legacy sample floors as the same value. */
export function echoAccountQualityWindowN(value: number | null | undefined): {
  account_quality_window_n: number
  min_success_samples: number
  min_ttft_samples: number
} {
  const n = clampAccountQualityWindowN(value)
  return {
    account_quality_window_n: n,
    min_success_samples: n,
    min_ttft_samples: n
  }
}

export function optionalAccountQualityWindowN(value: number | string | null | undefined): number | null {
  if (value === '' || value == null) return null
  const parsed = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(parsed)) return null
  return clampAccountQualityWindowN(parsed)
}

export function qualityRateWindowK(stats: {
  success_count?: number | null
  error_count?: number | null
} | null | undefined): number {
  if (!stats) return 0
  return Math.max(0, (stats.success_count ?? 0) + (stats.error_count ?? 0))
}
