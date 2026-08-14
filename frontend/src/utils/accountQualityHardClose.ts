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

export function scheduleUserHasQualityGate(user: {
  quality_max_p50_ttft_ms?: number | null
  quality_min_success_rate?: number | null
  quality_min_success_samples?: number | null
  quality_min_ttft_samples?: number | null
  quality_condition?: string | null
}): boolean {
  return user.quality_max_p50_ttft_ms != null || user.quality_min_success_rate != null
}

/** Shared threshold form used by user-schedule quality gates. Not bound to a user id. */
export type QualityGateFormFields = {
  quality_max_p50_ttft_ms: number | null
  quality_min_success_rate_percent: number | null
  quality_min_success_samples: number | null
  quality_min_ttft_samples: number | null
  quality_condition: 'or' | 'and'
}

export type QualityThresholdTemplate = {
  max_p50_ttft_ms: number | null
  min_success_rate: number | null
  pause_minutes: number
  min_success_samples: number
  min_ttft_samples: number
  condition: 'or' | 'and'
  enabled: boolean
}

export function qualityGateFormFromTemplate(template: QualityThresholdTemplate): QualityGateFormFields {
  return {
    quality_max_p50_ttft_ms: template.max_p50_ttft_ms,
    quality_min_success_rate_percent: successRateToPercent(template.min_success_rate),
    quality_min_success_samples: template.min_success_samples,
    quality_min_ttft_samples: template.min_ttft_samples,
    quality_condition: template.condition === 'and' ? 'and' : 'or'
  }
}

/** Overwrite shared threshold fields; keep master switch and pause minutes. */
export function mergeQualityTemplateFromGate(
  current: QualityThresholdTemplate,
  gate: QualityGateFormFields
): QualityThresholdTemplate {
  return {
    enabled: current.enabled,
    max_p50_ttft_ms: gate.quality_max_p50_ttft_ms,
    min_success_rate: percentToSuccessRate(gate.quality_min_success_rate_percent),
    pause_minutes: current.pause_minutes,
    min_success_samples: gate.quality_min_success_samples ?? current.min_success_samples,
    min_ttft_samples: gate.quality_min_ttft_samples ?? current.min_ttft_samples,
    condition: gate.quality_condition === 'and' ? 'and' : 'or'
  }
}

export function qualityGateFormFromDraft(draft: {
  maxP50: string | number
  successPercent: string | number
  minSuccessSamples: string | number
  minTtftSamples: string | number
  condition: string
}): QualityGateFormFields {
  return {
    quality_max_p50_ttft_ms: optionalNumber(draft.maxP50),
    quality_min_success_rate_percent: optionalNumber(draft.successPercent),
    quality_min_success_samples: optionalNumber(draft.minSuccessSamples),
    quality_min_ttft_samples: optionalNumber(draft.minTtftSamples),
    quality_condition: draft.condition === 'and' ? 'and' : 'or'
  }
}

export function applyQualityGateFormToDraft(
  draft: {
    maxP50: string | number
    successPercent: string | number
    minSuccessSamples: string | number
    minTtftSamples: string | number
    condition: 'or' | 'and'
  },
  fields: QualityGateFormFields
) {
  draft.maxP50 = fields.quality_max_p50_ttft_ms ?? ''
  draft.successPercent = fields.quality_min_success_rate_percent ?? ''
  draft.minSuccessSamples = fields.quality_min_success_samples ?? ''
  draft.minTtftSamples = fields.quality_min_ttft_samples ?? ''
  draft.condition = fields.quality_condition
}
