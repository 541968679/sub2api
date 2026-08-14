export const QUALITY_HARD_CLOSE_REASON_PREFIX = 'quality_hard_close'

export function isQualityHardCloseReason(reason: string | null | undefined): boolean {
  return typeof reason === 'string' && reason.startsWith(QUALITY_HARD_CLOSE_REASON_PREFIX)
}

export function isQualityHardClosePaused(
  until: string | null | undefined,
  reason: string | null | undefined,
  now = new Date()
): boolean {
  if (!isQualityHardCloseReason(reason) || !until) return false
  const resumeAt = new Date(until)
  return !Number.isNaN(resumeAt.getTime()) && resumeAt > now
}

/** Convert API success rate (0–1) to a percent field for admin forms. */
export function successRateToPercent(rate: number | null | undefined): number | null {
  if (rate == null || !Number.isFinite(rate)) return null
  return Math.round(rate * 1000) / 10
}

/** Convert a percent form field back to the API's 0–1 success rate. */
export function percentToSuccessRate(percent: number | string | null | undefined): number | null {
  if (percent === '' || percent == null) return null
  const value = typeof percent === 'number' ? percent : Number(percent)
  if (!Number.isFinite(value)) return null
  return value / 100
}

export function optionalNumber(value: number | string | null | undefined): number | null {
  if (value === '' || value == null) return null
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(parsed) ? parsed : null
}
