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

export const ACCOUNT_QUALITY_WINDOW_SECONDS = 900
export const DEFAULT_USER_QUALITY_MIN_SUCCESS_SAMPLES = 20
export const DEFAULT_USER_QUALITY_MIN_TTFT_SAMPLES = 10

export type ScheduleUserQualityChipState = 'none' | 'configured' | 'blocked' | 'resumed'

export type ScheduleUserQualityChipInput = {
  quality_max_p50_ttft_ms?: number | null
  quality_min_success_rate?: number | null
  quality_min_success_samples?: number | null
  quality_min_ttft_samples?: number | null
  quality_condition?: string | null
  quality_blocked?: boolean
  quality_resumed_until?: number | null
  quality_window_until?: number | null
}

export type AccountQualityChipStats = {
  success_count?: number
  error_count?: number
  success_rate?: number | null
  p50_ttft_ms?: number | null
  ttft_samples?: number
}

function positiveOrDefault(value: number | null | undefined, fallback: number): number {
  return value != null && value >= 1 ? value : fallback
}

/** Same judged-metric rules as backend EvaluateAccountQualityHardClose (user gates always enabled). */
export function userQualityGateBreached(
  user: ScheduleUserQualityChipInput,
  stats: AccountQualityChipStats | null | undefined
): boolean {
  if (!scheduleUserHasQualityGate(user) || !stats) return false
  const minSuccess = positiveOrDefault(user.quality_min_success_samples, DEFAULT_USER_QUALITY_MIN_SUCCESS_SAMPLES)
  const minTtft = positiveOrDefault(user.quality_min_ttft_samples, DEFAULT_USER_QUALITY_MIN_TTFT_SAMPLES)
  const judged: boolean[] = []
  if (user.quality_max_p50_ttft_ms != null && (stats.ttft_samples ?? 0) >= minTtft && stats.p50_ttft_ms != null) {
    judged.push(stats.p50_ttft_ms > user.quality_max_p50_ttft_ms)
  }
  if (user.quality_min_success_rate != null) {
    const samples = (stats.success_count ?? 0) + (stats.error_count ?? 0)
    if (samples >= minSuccess) {
      const rate = stats.success_rate != null && Number.isFinite(stats.success_rate)
        ? stats.success_rate
        : samples > 0
          ? (stats.success_count ?? 0) / samples
          : 0
      judged.push(rate < user.quality_min_success_rate)
    }
  }
  if (judged.length === 0) return false
  if (user.quality_condition === 'and') return judged.every(Boolean)
  return judged.some(Boolean)
}

export function scheduleUserQualityChipState(
  user: ScheduleUserQualityChipInput,
  stats?: AccountQualityChipStats | null,
  nowMs = Date.now()
): ScheduleUserQualityChipState {
  if (!scheduleUserHasQualityGate(user)) return 'none'
  if (user.quality_resumed_until != null && user.quality_resumed_until * 1000 > nowMs) {
    return 'resumed'
  }
  // 点已恢复，或已恢复满 15 分钟：芯片回质量，并累计新窗口；窗口内不用旧数据打已停。
  if (user.quality_window_until != null && user.quality_window_until * 1000 > nowMs) {
    return 'configured'
  }
  if (user.quality_blocked || userQualityGateBreached(user, stats)) {
    return 'blocked'
  }
  return 'configured'
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
  schedule_use_failover_error_rate?: boolean
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
    condition: gate.quality_condition === 'and' ? 'and' : 'or',
    schedule_use_failover_error_rate: current.schedule_use_failover_error_rate === true
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
