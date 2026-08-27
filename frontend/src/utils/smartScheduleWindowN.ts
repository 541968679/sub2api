import type { SmartSchedulePairQuality } from '@/api/admin/users'

export const SMART_SCHEDULE_WINDOW_N_DEFAULT = 10
export const SMART_SCHEDULE_WINDOW_N_MIN = 1
export const SMART_SCHEDULE_WINDOW_N_MAX = 100

export type SmartScheduleWindowNInput = {
  quality_window_n?: number | null
  quality_window_samples?: number | null
  quality_min_success_samples?: number | null
  quality_min_ttft_samples?: number | null
}

function finiteNumber(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

export function clampSmartScheduleWindowN(value: number | null | undefined): number {
  if (value == null || !Number.isFinite(value)) return SMART_SCHEDULE_WINDOW_N_DEFAULT
  return Math.min(
    SMART_SCHEDULE_WINDOW_N_MAX,
    Math.max(SMART_SCHEDULE_WINDOW_N_MIN, Math.round(value))
  )
}

function sharedWindowFallback(input: SmartScheduleWindowNInput): number | null {
  return finiteNumber(input.quality_window_n) ?? finiteNumber(input.quality_window_samples)
}

/** N首字. Launch: both stored columns equal the user's current single N. */
export function resolveSmartScheduleTtftN(input: SmartScheduleWindowNInput): number {
  const explicit = finiteNumber(input.quality_min_ttft_samples)
  if (explicit != null) return clampSmartScheduleWindowN(explicit)
  if (finiteNumber(input.quality_min_success_samples) != null) {
    return SMART_SCHEDULE_WINDOW_N_DEFAULT
  }
  return clampSmartScheduleWindowN(sharedWindowFallback(input))
}

/** N成功率. follow_n uses this. Do not inherit quality_window_n from N首字. */
export function resolveSmartScheduleSuccessN(input: SmartScheduleWindowNInput): number {
  const explicit = finiteNumber(input.quality_min_success_samples)
  if (explicit != null) return clampSmartScheduleWindowN(explicit)
  if (finiteNumber(input.quality_min_ttft_samples) != null) {
    return SMART_SCHEDULE_WINDOW_N_DEFAULT
  }
  return clampSmartScheduleWindowN(sharedWindowFallback(input))
}

/** Compat single N = max of the two metric windows, or explicit quality_window_n. */
export function resolveSmartScheduleWindowN(input: SmartScheduleWindowNInput): number {
  const explicit = sharedWindowFallback(input)
  if (explicit != null) return clampSmartScheduleWindowN(explicit)
  return Math.max(resolveSmartScheduleTtftN(input), resolveSmartScheduleSuccessN(input))
}

export function normalizeSmartSchedulePairQuality(
  raw: Partial<SmartSchedulePairQuality> & {
    p50_ttft_ms?: number | null
    ttft_count?: number | null
    ok_count?: number | null
    quality_window_n?: number | null
    quality_window_samples?: number | null
    success_samples?: number | null
    n_ttft?: number | null
    n_success?: number | null
    n_ok?: number | null
    ttft_slow_count?: number | null
    ttft_consecutive_slow?: number | null
    quality_sched_max_slow_in_window?: number | null
    quality_sched_max_consecutive_slow?: number | null
  } | null | undefined
): SmartSchedulePairQuality | null {
  if (!raw) return null
  const nTtft = clampSmartScheduleWindowN(
    finiteNumber(raw.n_ttft) ?? finiteNumber(raw.n) ?? finiteNumber(raw.quality_window_n)
  )
  const nSuccess = clampSmartScheduleWindowN(
    finiteNumber(raw.n_success) ?? finiteNumber(raw.n_ok) ?? finiteNumber(raw.n) ?? finiteNumber(raw.quality_window_n)
  )
  const out: SmartSchedulePairQuality = {
    ttft_p50_ms: finiteNumber(raw.ttft_p50_ms ?? raw.p50_ttft_ms),
    success_rate:
      raw.success_rate != null && Number.isFinite(raw.success_rate) ? raw.success_rate : null,
    ttft_samples: Math.max(0, finiteNumber(raw.ttft_samples ?? raw.ttft_count) ?? 0),
    ok_samples: Math.max(0, finiteNumber(raw.ok_samples ?? raw.ok_count ?? raw.success_samples) ?? 0),
    n: clampSmartScheduleWindowN(raw.n ?? Math.max(nTtft, nSuccess)),
    n_ttft: nTtft,
    n_success: nSuccess
  }
  const slow = finiteNumber(raw.ttft_slow_count)
  const consec = finiteNumber(raw.ttft_consecutive_slow)
  const k = finiteNumber(raw.quality_sched_max_slow_in_window)
  const c = finiteNumber(raw.quality_sched_max_consecutive_slow)
  if (slow != null) out.ttft_slow_count = Math.max(0, Math.round(slow))
  if (consec != null) out.ttft_consecutive_slow = Math.max(0, Math.round(consec))
  if (k != null && k > 0) out.quality_sched_max_slow_in_window = Math.round(k)
  if (c != null && c > 0) out.quality_sched_max_consecutive_slow = Math.round(c)
  if (raw.metrics_phase) out.metrics_phase = raw.metrics_phase
  if (raw.quality_reason) out.quality_reason = raw.quality_reason
  if (raw.probe) out.probe = raw.probe
  if (raw.sched) out.sched = raw.sched
  if (raw.soft) out.soft = raw.soft
  return out
}

export function pairQualityLatencyKCParams(quality: SmartSchedulePairQuality): {
  show: boolean
  showK: boolean
  showC: boolean
  slow: number
  consec: number
  k: number
  c: number
} {
  const k = finiteNumber(quality.quality_sched_max_slow_in_window)
  const c = finiteNumber(quality.quality_sched_max_consecutive_slow)
  const showK = k != null && k > 0
  const showC = c != null && c > 0
  return {
    show: showK || showC,
    showK,
    showC,
    slow: Math.max(0, finiteNumber(quality.ttft_slow_count) ?? 0),
    consec: Math.max(0, finiteNumber(quality.ttft_consecutive_slow) ?? 0),
    k: showK ? k! : 0,
    c: showC ? c! : 0
  }
}

export function pairQualityCountParams(quality: SmartSchedulePairQuality): {
  ttft: number
  ok: number
  nTtft: number
  nOk: number
  n: number
} {
  const nTtft = clampSmartScheduleWindowN(quality.n_ttft ?? quality.n)
  const nOk = clampSmartScheduleWindowN(quality.n_success ?? quality.n_ok ?? quality.n)
  return {
    ttft: quality.ttft_samples,
    ok: quality.ok_samples,
    nTtft,
    nOk,
    n: clampSmartScheduleWindowN(quality.n ?? Math.max(nTtft, nOk))
  }
}
