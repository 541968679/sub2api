import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SmartSchedulePairQualityCell from '../SmartSchedulePairQualityCell.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, string | number>) => {
      if (params && params.slow != null) {
        return `${key}:${params.slow}/${params.k}/${params.consec ?? ''}/${params.c ?? ''}`
      }
      if (params) return `${key}:${params.ttft}/${params.ok}/${params.nTtft ?? params.n}/${params.nOk ?? params.n}`
      return key
    }
  })
}))

describe('SmartSchedulePairQualityCell', () => {
  it('renders p50, p95, success rate, and window counts then emits click', async () => {
    const w = mount(SmartSchedulePairQualityCell, {
      props: {
        quality: {
          ttft_p50_ms: 180,
          ttft_p95_ms: 900,
          success_rate: 0.95,
          ttft_samples: 4,
          ok_samples: 6,
          n: 10
        }
      }
    })
    expect(w.text()).toContain('180ms')
    expect(w.get('[data-testid="smart-schedule-pair-quality-p95"]').text()).toContain('900ms')
    expect(w.text()).toContain('95.0%')
    expect(w.get('[data-testid="smart-schedule-pair-quality-counts"]').text()).toContain('4/6/10/10')
    await w.get('[data-testid="smart-schedule-pair-quality-cell"]').trigger('click')
    expect(w.emitted('click')).toHaveLength(1)
  })

  it('shows an em dash when p95 is missing', () => {
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
    expect(w.get('[data-testid="smart-schedule-pair-quality-p95"]').text()).toContain('p95')
    expect(w.get('[data-testid="smart-schedule-pair-quality-p95"]').text()).toContain('—')
  })

  it('falls back to probe-window p95 when top-level alias is empty', () => {
    const w = mount(SmartSchedulePairQualityCell, {
      props: {
        quality: {
          ttft_p50_ms: null,
          ttft_p95_ms: null,
          p95_ttft_ms: null,
          success_rate: null,
          ttft_samples: 0,
          ok_samples: 0,
          n: 10,
          metrics_phase: 'soft',
          probe: {
            p50_ttft_ms: 1800,
            p95_ttft_ms: 4200,
            success_rate: 1,
            ttft_samples: 1,
            n_ttft: 10,
            ok_samples: 1,
            n_ok: 10
          }
        }
      }
    })
    expect(w.get('[data-testid="smart-schedule-pair-quality-p95"]').text()).toContain('4200ms')
  })

  it('uses separate TTFT and success denominators', () => {
    const w = mount(SmartSchedulePairQualityCell, {
      props: {
        quality: {
          ttft_p50_ms: 180,
          success_rate: 0.95,
          ttft_samples: 3,
          ok_samples: 6,
          n: 20,
          n_ttft: 3,
          n_success: 20
        }
      }
    })
    expect(w.get('[data-testid="smart-schedule-pair-quality-counts"]').text()).toContain('3/6/3/20')
    expect(w.find('[data-testid="smart-schedule-pair-quality-kc"]').exists()).toBe(false)
  })

  it('adds a K/C row under success when sched composite is present', () => {
    const w = mount(SmartSchedulePairQualityCell, {
      props: {
        quality: {
          ttft_p50_ms: 180,
          success_rate: 0.95,
          ttft_samples: 4,
          ok_samples: 6,
          n: 10,
          ttft_slow_count: 2,
          ttft_consecutive_slow: 1,
          quality_sched_max_slow_in_window: 3,
          quality_sched_max_consecutive_slow: 2
        }
      }
    })
    expect(w.get('[data-testid="smart-schedule-pair-quality-kc"]').text()).toContain('2/3/1/2')
  })

  it('shows p50 from a single underfull TTFT sample', () => {
    const w = mount(SmartSchedulePairQualityCell, {
      props: {
        quality: {
          ttft_p50_ms: 1800,
          success_rate: 1,
          ttft_samples: 1,
          ok_samples: 1,
          n: 10,
          n_ttft: 10,
          n_success: 10,
          metrics_phase: 'probe'
        }
      }
    })
    expect(w.text()).toContain('1800ms')
    expect(w.text()).not.toMatch(/p50\s+—/)
  })

  it('falls back to probe-window p50 when top-level alias is empty', () => {
    const w = mount(SmartSchedulePairQualityCell, {
      props: {
        quality: {
          ttft_p50_ms: null,
          success_rate: null,
          ttft_samples: 0,
          ok_samples: 0,
          n: 10,
          metrics_phase: 'soft',
          probe: {
            p50_ttft_ms: 1800,
            success_rate: 1,
            ttft_samples: 1,
            n_ttft: 10,
            ok_samples: 1,
            n_ok: 10
          }
        }
      }
    })
    expect(w.text()).toContain('1800ms')
    expect(w.text()).toContain('100.0%')
  })

  it('labels the active gate window', () => {
    const w = mount(SmartSchedulePairQualityCell, {
      props: {
        quality: {
          ttft_p50_ms: 2000,
          success_rate: 1,
          ttft_samples: 2,
          ok_samples: 10,
          n: 10,
          n_ttft: 2,
          n_success: 10,
          metrics_phase: 'sched'
        }
      }
    })
    expect(w.get('[data-testid="smart-schedule-pair-quality-phase"]').text()).toContain(
      'admin.users.smartSchedule.pairMetricsPhaseSched'
    )
  })
})
