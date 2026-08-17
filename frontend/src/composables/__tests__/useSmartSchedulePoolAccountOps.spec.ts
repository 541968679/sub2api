import { describe, expect, it, vi, beforeEach } from 'vitest'
import { defineComponent } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useSmartSchedulePoolAccountOps } from '../useSmartSchedulePoolAccountOps'
import type { Account } from '@/types'

const apiMocks = vi.hoisted(() => ({
  getById: vi.fn(),
  getAllWithCount: vi.fn(),
  getAllGroups: vi.fn()
}))

const storeMocks = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
  showWarning: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getById: apiMocks.getById
    },
    proxies: {
      getAllWithCount: apiMocks.getAllWithCount
    },
    groups: {
      getAll: apiMocks.getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => storeMocks
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

function mountOps() {
  const patchPoolAccount = vi.fn()
  const Comp = defineComponent({
    setup() {
      return useSmartSchedulePoolAccountOps({ patchPoolAccount })
    },
    template: '<div />'
  })
  return { wrapper: mount(Comp), patchPoolAccount }
}

describe('useSmartSchedulePoolAccountOps handleEdit', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    apiMocks.getAllWithCount.mockResolvedValue([])
    apiMocks.getAllGroups.mockResolvedValue([])
  })

  it('loads the full account before opening the editor', async () => {
    const full = {
      id: 21,
      name: 'oa-1',
      notes: 'live notes',
      platform: 'openai',
      type: 'oauth',
      credentials: { access_token: 'tok', email: 'a@b.com' },
      extra: { openai_passthrough: true },
      group_ids: [3]
    }
    apiMocks.getById.mockResolvedValue(full)
    const { wrapper } = mountOps()
    await wrapper.vm.handleEdit({
      id: 21,
      name: 'oa-1',
      platform: 'openai',
      type: 'oauth',
      credentials: { plan_type: 'pro' }
    } as Account)
    await flushPromises()
    expect(apiMocks.getById).toHaveBeenCalledWith(21)
    expect(wrapper.vm.showEdit).toBe(true)
    expect(wrapper.vm.edAcc).toEqual(full)
  })

  it('does not open the editor when the full account fails to load', async () => {
    apiMocks.getById.mockRejectedValue(new Error('boom'))
    const { wrapper } = mountOps()
    await wrapper.vm.handleEdit({ id: 21, name: 'oa-1' } as Account)
    await flushPromises()
    expect(wrapper.vm.showEdit).toBe(false)
    expect(wrapper.vm.edAcc).toBeNull()
    expect(storeMocks.showError).toHaveBeenCalled()
  })
})
