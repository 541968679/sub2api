import type { SmartSchedulePairQuality } from '@/api/admin/users'

export const SMART_SCHEDULE_WINDOW_N_DEFAULT = 10
export const SMART_SCHEDULE_WINDOW_N_MIN = 1
export const SMART_SCHEDULE_WINDOW_N_MAX = 100

function finiteNumber(value: unknown): number | null {
  if (typeof value !== 'number' || !Number.isFinite(value)) return null
  return value
}

export function clampSmartScheduleWindowN(value: number | null | undefined): number {
  if (value == null || !Number.isFinite(value)) return SMART_SCHEDULE_WINDOW_N_DEFAULT
  return Math.min(
    SMART_SCHEDULE_WINDOW_N_MAX,
    Math.max(SMART_SCHEDULE_WINDOW_N_MIN, Math.round(value))
  )
}

/** Prefer explicit N; both legacy sample fields → min, then clamp (backend contract). */
export function resolveSmartScheduleWindowN(input: {
  quality_window_n?: number | null
  quality_window_samples?: number | null
  quality_min_success_samples?: number | null
  quality_min_ttft_samples?: number | null
}): number {
  const explicit = input.quality_window_n ?? input.quality_window_samples
  if (explicit != null && Number.isFinite(explicit)) {
    return clampSmartScheduleWindowN(explicit)
  }
  const success = input.quality_min_success_samples
  const ttft = input.quality_min_ttft_samples
  if (
    success != null
    && ttft != null
    && Number.isFinite(success)
    && Number.isFinite(ttft)
  ) {
    return clampSmartScheduleWindowN(Math.min(success, ttft))
  }
  return clampSmartScheduleWindowN(success ?? ttft ?? SMART_SCHEDULE_WINDOW_N_DEFAULT)
}

export function normalizeSmartSchedulePairQuality(
  raw: Partial<SmartSchedulePairQuality> & {
    p50_ttft_ms?: number | null
    ttft_count?: number | null
    ok_count?: number | null
    quality_window_n?: number | null
    quality_window_samples?: number | null
    success_samples?: number | null
  } | null | undefined
): SmartSchedulePairQuality | null {
  if (!raw) return null
  return {
    ttft_p50_ms: finiteNumber(raw.ttft_p50_ms ?? raw.p50_ttft_ms),
    success_rate:
      raw.success_rate != null && Number.isFinite(raw.success_rate) ? raw.success_rate : null,
    ttft_samples: Math.max(0, finiteNumber(raw.ttft_samples ?? raw.ttft_count) ?? 0),
    ok_samples: Math.max(0, finiteNumber(raw.ok_samples ?? raw.ok_count ?? raw.success_samples) ?? 0),
    n: clampSmartScheduleWindowN(raw.n ?? raw.quality_window_n ?? raw.quality_window_samples)
  }
}
