import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountTableFilters from '../AccountTableFilters.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue', 'change'],
  template: `
    <select
      :value="modelValue"
      @change="$emit('update:modelValue', ($event.target).value); $emit('change')"
    >
      <option v-for="opt in options" :key="String(opt.value)" :value="opt.value">{{ opt.label }}</option>
    </select>
  `
}

describe('AccountTableFilters sort options', () => {
  it('includes upstream rate asc/desc in the sort dropdown', async () => {
    const wrapper = mount(AccountTableFilters, {
      props: {
        searchQuery: '',
        filters: { platform: '', type: '', status: '', privacy_mode: '', group: '', sort_by: 'created_at', sort_order: 'desc' }
      },
      global: {
        stubs: {
          SearchInput: true,
          Select: SelectStub
        }
      }
    })

    const values = wrapper.findAll('option').map((opt) => opt.attributes('value'))
    expect(values).toContain('upstream_rate_multiplier:asc')
    expect(values).toContain('upstream_rate_multiplier:desc')

    const selects = wrapper.findAll('select')
    const sortSelect = selects[selects.length - 1]
    await sortSelect.setValue('upstream_rate_multiplier:asc')
    expect(wrapper.emitted('update:filters')?.[0]?.[0]).toMatchObject({
      sort_by: 'upstream_rate_multiplier',
      sort_order: 'asc'
    })
    wrapper.unmount()
  })
})
