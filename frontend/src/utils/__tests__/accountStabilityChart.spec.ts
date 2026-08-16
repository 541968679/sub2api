import { afterEach, describe, expect, it } from 'vitest'
import {
  STABILITY_P95_CLIP_FACTOR,
  STABILITY_SHOW_P95_STORAGE_KEY,
  STABILITY_TTFT_HEADROOM,
  clampSeriesToMax,
  computeStabilityTtftAxis,
  finiteMax,
  readShowP95Preference,
  writeShowP95Preference
} from '@/utils/accountStabilityChart'

describe('accountStabilityChart', () => {
  afterEach(() => {
    localStorage.removeItem(STABILITY_SHOW_P95_STORAGE_KEY)
  })

  it('defaults the persisted p95 toggle to hidden', () => {
    expect(readShowP95Preference()).toBe(false)
    writeShowP95Preference(true)
    expect(localStorage.getItem(STABILITY_SHOW_P95_STORAGE_KEY)).toBe('1')
    expect(readShowP95Preference()).toBe(true)
    writeShowP95Preference(false)
    expect(readShowP95Preference()).toBe(false)
  })

  it('scales the left axis to visible p50 when p95 is hidden', () => {
    const axis = computeStabilityTtftAxis({
      p50Values: [300, 280, null],
      p95Values: [9000, 8000],
      showP95: false
    })

    expect(finiteMax([300, 280, null])).toBe(300)
    expect(axis.clipped).toBe(false)
    expect(axis.max).toBe(Math.ceil(300 * STABILITY_TTFT_HEADROOM))
  })

  it('does not use raw max(p50, p95) when that would flatten p50', () => {
    const axis = computeStabilityTtftAxis({
      p50Values: [300, 280],
      p95Values: [9000, 8000],
      showP95: true
    })

    expect(axis.clipped).toBe(true)
    expect(axis.max).toBe(Math.ceil(300 * STABILITY_P95_CLIP_FACTOR))
    expect(axis.max).toBeLessThan(9000)
  })

  it('keeps both series on one scale when p95 stays within the clip factor', () => {
    const axis = computeStabilityTtftAxis({
      p50Values: [300, 280],
      p95Values: [400, 360],
      showP95: true
    })

    expect(axis.clipped).toBe(false)
    expect(axis.max).toBe(Math.ceil(400 * STABILITY_TTFT_HEADROOM))
  })

  it('clamps overflow p95 points to the current axis max', () => {
    expect(clampSeriesToMax([300, 9000, null], 750)).toEqual([300, 750, null])
    expect(clampSeriesToMax([300, 400], undefined)).toEqual([300, 400])
  })
})
