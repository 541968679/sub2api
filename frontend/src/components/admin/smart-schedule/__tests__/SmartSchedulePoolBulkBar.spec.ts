import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SmartSchedulePoolBulkBar from '../SmartSchedulePoolBulkBar.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        if (params) return key.replace(/\{(\w+)\}/g, (_, name) => String(params[name] ?? ''))
        return key
      }
    })
  }
})

function mountBar(overrides: Record<string, unknown> = {}) {
  return mount(SmartSchedulePoolBulkBar, {
    props: {
      selectedIds: [],
      admissionIds: [],
      filteredCount: 2,
      bulkCap: null,
      ...overrides
    }
  })
}

describe('SmartSchedulePoolBulkBar', () => {
  it('emits type select and disables admission when nothing operable is selected', async () => {
    const w = mountBar()
    expect(w.get('[data-testid="smart-schedule-batch-paused"]').attributes('disabled')).toBeDefined()
    expect(w.get('[data-testid="smart-schedule-batch-selectable"]').attributes('disabled')).toBeDefined()
    expect(w.get('[data-testid="smart-schedule-batch-probing"]').attributes('disabled')).toBeDefined()
    await w.get('[data-testid="smart-schedule-select-oauth"]').trigger('click')
    await w.get('[data-testid="smart-schedule-select-apikey"]').trigger('click')
    expect(w.emitted('select-oauth')).toHaveLength(1)
    expect(w.emitted('select-apikey')).toHaveLength(1)
  })

  it('emits admission states for the current operable selection', async () => {
    const w = mountBar({ selectedIds: [11, 12], admissionIds: [12] })
    expect(w.get('[data-testid="smart-schedule-batch-paused"]').attributes('disabled')).toBeUndefined()
    await w.get('[data-testid="smart-schedule-batch-paused"]').trigger('click')
    await w.get('[data-testid="smart-schedule-batch-selectable"]').trigger('click')
    await w.get('[data-testid="smart-schedule-batch-probing"]').trigger('click')
    expect(w.emitted('apply-admission')).toEqual([['paused'], ['selectable'], ['probing']])
  })
})
