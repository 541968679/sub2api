import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountBulkActionsBar from '../AccountBulkActionsBar.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

function mountBar(selectedIds: number[] = []) {
  return mount(AccountBulkActionsBar, {
    props: {
      selectedIds,
      total: 12,
      selectingAllFiltered: false
    }
  })
}

describe('AccountBulkActionsBar', () => {
  it('shows unbind-subscription button without a selection', async () => {
    const wrapper = mountBar([])
    const button = wrapper.get('[data-testid="unbind-subscription-by-rate"]')
    expect(button.exists()).toBe(true)
    expect((button.element as HTMLButtonElement).disabled).toBe(false)
    await button.trigger('click')
    expect(wrapper.emitted('unbind-subscription-by-rate')).toHaveLength(1)
  })

  it('keeps the unbind-subscription button when accounts are selected', async () => {
    const wrapper = mountBar([1, 2])
    expect(wrapper.get('[data-testid="unbind-subscription-by-rate"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="bulk-edit-filtered"]').exists()).toBe(true)
  })
})
