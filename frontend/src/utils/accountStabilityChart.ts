export const STABILITY_SHOW_P95_STORAGE_KEY = 'sub2api.account-stability.show-p95'
export const STABILITY_TTFT_HEADROOM = 1.15
export const STABILITY_P95_CLIP_FACTOR = 2.5

export type StabilityTtftAxisInput = {
  p50Values: Array<number | null | undefined>
  p95Values: Array<number | null | undefined>
  showP95: boolean
}

export type StabilityTtftAxis = {
  max: number | undefined
  clipped: boolean
}

export function readShowP95Preference(): boolean {
  if (typeof localStorage === 'undefined') return false
  try {
    return localStorage.getItem(STABILITY_SHOW_P95_STORAGE_KEY) === '1'
  } catch {
    return false
  }
}

export function writeShowP95Preference(show: boolean): void {
  if (typeof localStorage === 'undefined') return
  try {
    localStorage.setItem(STABILITY_SHOW_P95_STORAGE_KEY, show ? '1' : '0')
  } catch {
    // Ignore quota / private-mode failures; the session toggle still works.
  }
}

export function finiteMax(values: Array<number | null | undefined>): number | null {
  let max = -Infinity
  for (const value of values) {
    if (typeof value === 'number' && Number.isFinite(value)) {
      max = Math.max(max, value)
    }
  }
  return Number.isFinite(max) ? max : null
}

function withHeadroom(value: number): number {
  if (value <= 0) return 1
  return Math.ceil(value * STABILITY_TTFT_HEADROOM)
}

/** Left TTFT axis from currently visible series. Hidden p95 scales to p50; both-visible clips if p95 would flatten p50. */
export function computeStabilityTtftAxis(input: StabilityTtftAxisInput): StabilityTtftAxis {
  const p50Max = finiteMax(input.p50Values)
  if (!input.showP95) {
    return { max: p50Max == null ? undefined : withHeadroom(p50Max), clipped: false }
  }

  const p95Max = finiteMax(input.p95Values)
  if (p50Max == null) {
    return { max: p95Max == null ? undefined : withHeadroom(p95Max), clipped: false }
  }

  const clipCap = p50Max * STABILITY_P95_CLIP_FACTOR
  if (p95Max != null && p95Max > clipCap) {
    return { max: Math.ceil(clipCap), clipped: true }
  }

  const visibleMax = p95Max == null ? p50Max : Math.max(p50Max, p95Max)
  return { max: withHeadroom(visibleMax), clipped: false }
}

export function clampSeriesToMax(
  values: Array<number | null | undefined>,
  max: number | undefined
): Array<number | null> {
  return values.map((value) => {
    if (value == null || !Number.isFinite(value)) return null
    if (max == null) return value
    return Math.min(value, max)
  })
}
