import { describe, expect, it } from 'vitest'
import {
  ACCOUNT_QUALITY_WINDOW_N_DEFAULT,
  clampAccountQualityWindowN,
  echoAccountQualityWindowN,
  qualityRateWindowK,
  resolveAccountQualityWindowN
} from '@/utils/accountQualityWindowN'

describe('accountQualityWindowN', () => {
  it('clamps to 1–100 and defaults empty values to 20', () => {
    expect(clampAccountQualityWindowN(null)).toBe(ACCOUNT_QUALITY_WINDOW_N_DEFAULT)
    expect(clampAccountQualityWindowN(0)).toBe(1)
    expect(clampAccountQualityWindowN(7.4)).toBe(7)
    expect(clampAccountQualityWindowN(250)).toBe(100)
    expect(clampAccountQualityWindowN(1)).toBe(1)
  })

  it('prefers account_quality_window_n over legacy sample floors', () => {
    expect(
      resolveAccountQualityWindowN({
        account_quality_window_n: 14,
        min_success_samples: 20,
        min_ttft_samples: 10
      })
    ).toBe(14)
    expect(resolveAccountQualityWindowN({ window_n: 9, n: 3, min_success_samples: 20 })).toBe(9)
    expect(resolveAccountQualityWindowN({ n: 6 })).toBe(6)
  })

  it('does not take min(20,10) when only legacy floors exist', () => {
    expect(
      resolveAccountQualityWindowN({
        min_success_samples: 20,
        min_ttft_samples: 10
      })
    ).toBe(20)
    expect(resolveAccountQualityWindowN({ min_ttft_samples: 8 })).toBe(8)
    expect(resolveAccountQualityWindowN({})).toBe(20)
  })

  it('echoes one N onto both legacy sample fields', () => {
    expect(echoAccountQualityWindowN(14)).toEqual({
      account_quality_window_n: 14,
      min_success_samples: 14,
      min_ttft_samples: 14
    })
  })

  it('counts success+error as k', () => {
    expect(qualityRateWindowK({ success_count: 10, error_count: 1 })).toBe(11)
    expect(qualityRateWindowK(null)).toBe(0)
  })
})
