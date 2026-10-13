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

    expect(wrapper.find('[data-testid="account-group-order-open"]').exists()).toBe(false)

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

  it('splits domestic OpenAI groups into their own dropdown on the OpenAI platform', async () => {
    const groups = [
      { id: 9, name: 'gpt 每月2000刀', platform: 'openai' },
      { id: 23, name: 'Ccmax高质量', platform: 'anthropic' },
      { id: 46, name: '国产模型：聚合渠道', platform: 'openai' },
      { id: 48, name: 'kimi k3 官key', platform: 'openai' },
      { id: 55, name: 'glm 高速分组（官转限时特惠）', platform: 'openai' }
    ]
    const wrapper = mount(AccountTableFilters, {
      props: {
        searchQuery: '',
        filters: { platform: 'openai', type: '', status: '', privacy_mode: '', group: '', sort_by: 'created_at', sort_order: 'desc' },
        groups: groups as never
      },
      global: {
        stubs: {
          SearchInput: true,
          Select: SelectStub
        }
      }
    })

    const groupSelect = wrapper.get('[data-testid="account-filter-group"]')
    const domesticSelect = wrapper.get('[data-testid="account-filter-domestic-group"]')
    const groupLabels = groupSelect.findAll('option').map((opt) => opt.text())
    const domesticLabels = domesticSelect.findAll('option').map((opt) => opt.text())

    expect(groupLabels).toEqual([
      'admin.accounts.allGroups',
      'admin.accounts.ungroupedGroup',
      'gpt 每月2000刀',
      'Ccmax高质量'
    ])
    expect(domesticLabels).toEqual([
      'admin.accounts.domesticGroups',
      '国产模型：聚合渠道',
      'kimi k3 官key',
      'glm 高速分组（官转限时特惠）'
    ])

    await domesticSelect.setValue('48')
    expect(wrapper.emitted('update:filters')?.at(-1)?.[0]).toMatchObject({ group: '48' })
    expect(wrapper.emitted('change')).toHaveLength(1)

    await wrapper.setProps({
      filters: { platform: 'openai', type: '', status: '', privacy_mode: '', group: '48', sort_by: 'created_at', sort_order: 'desc' }
    })
    expect(groupSelect.element.value).toBe('')
    expect(domesticSelect.element.value).toBe('48')

    await domesticSelect.setValue('')
    expect(wrapper.emitted('update:filters')?.at(-1)?.[0]).toMatchObject({ group: '' })

    await wrapper.setProps({
      filters: { platform: 'openai', type: '', status: '', privacy_mode: '', group: '9', sort_by: 'created_at', sort_order: 'desc' }
    })
    const changeCount = wrapper.emitted('change')?.length ?? 0
    await domesticSelect.setValue('')
    expect(wrapper.emitted('change')?.length ?? 0).toBe(changeCount)
    expect(wrapper.emitted('update:filters')?.at(-1)?.[0]).toMatchObject({ group: '' })

    await wrapper.setProps({
      filters: { platform: '', type: '', status: '', privacy_mode: '', group: '', sort_by: 'created_at', sort_order: 'desc' }
    })
    expect(wrapper.find('[data-testid="account-filter-domestic-group"]').exists()).toBe(false)
    const merged = wrapper.get('[data-testid="account-filter-group"]').findAll('option').map((opt) => opt.text())
    expect(merged).toContain('国产模型：聚合渠道')
    expect(merged).toContain('gpt 每月2000刀')
    wrapper.unmount()
  })

  it('orders group options by sort_order and saves a reordered dropdown', async () => {
    const reorderGroups = vi.fn().mockResolvedValue(undefined)
    const groups = [
      { id: 48, name: 'kimi k3', platform: 'openai', sort_order: 40 },
      { id: 9, name: 'gpt later', platform: 'openai', sort_order: 30 },
      { id: 46, name: '国产模型', platform: 'openai', sort_order: 20 },
      { id: 2, name: 'gpt first', platform: 'openai', sort_order: 10 }
    ]
    const wrapper = mount(AccountTableFilters, {
      props: {
        searchQuery: '',
        filters: { platform: 'openai', type: '', status: '', privacy_mode: '', group: '', sort_by: 'created_at', sort_order: 'desc' },
        groups: groups as never,
        reorderGroups
      },
      global: {
        stubs: {
          SearchInput: true,
          Select: SelectStub,
          Icon: true,
          BaseDialog: {
            props: ['show'],
            template: '<div v-if="show"><slot /><slot name="footer" /></div>'
          },
          VueDraggable: {
            props: ['modelValue'],
            template: '<div><slot /></div>'
          }
        }
      }
    })

    expect(wrapper.get('[data-testid="account-filter-group"]').findAll('option').map((opt) => opt.text())).toEqual([
      'admin.accounts.allGroups',
      'admin.accounts.ungroupedGroup',
      'gpt first',
      'gpt later'
    ])
    expect(wrapper.get('[data-testid="account-filter-domestic-group"]').findAll('option').map((opt) => opt.text())).toEqual([
      'admin.accounts.domesticGroups',
      '国产模型',
      'kimi k3'
    ])

    await wrapper.get('[data-testid="account-group-order-open"]').trigger('click')
    expect(wrapper.get('[data-testid="account-group-order-dialog"]').text()).toContain('admin.accounts.groupOrder.standardSection')
    expect(wrapper.get('[data-testid="account-group-order-dialog"]').text()).toContain('admin.accounts.domesticGroups')
    await wrapper.get('[data-testid="account-group-order-down-2"]').trigger('click')
    await wrapper.get('[data-testid="account-group-order-up-48"]').trigger('click')
    await wrapper.get('[data-testid="account-group-order-save"]').trigger('click')

    expect(reorderGroups).toHaveBeenCalledWith([
      [9, 2],
      [48, 46]
    ])
    wrapper.unmount()
  })
})
