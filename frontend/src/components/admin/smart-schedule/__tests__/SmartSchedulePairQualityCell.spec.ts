import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SmartSchedulePairQualityCell from '../SmartSchedulePairQualityCell.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, string | number>) => {
      if (params) return `${key}:${params.ttft}/${params.ok}/${params.n}`
      return key
    }
  })
}))

describe('SmartSchedulePairQualityCell', () => {
  it('renders p50, success rate, and window counts then emits click', async () => {
    const w = mount(SmartSchedulePairQualityCell, {
      props: {
        quality: {
          ttft_p50_ms: 180,
          success_rate: 0.95,
          ttft_samples: 4,
          ok_samples: 6,
          n: 10
        }
      }
    })
    expect(w.text()).toContain('180ms')
    expect(w.text()).toContain('95.0%')
    expect(w.get('[data-testid="smart-schedule-pair-quality-counts"]').text()).toContain('4/6/10')
    await w.get('[data-testid="smart-schedule-pair-quality-cell"]').trigger('click')
    expect(w.emitted('click')).toHaveLength(1)
  })
})
