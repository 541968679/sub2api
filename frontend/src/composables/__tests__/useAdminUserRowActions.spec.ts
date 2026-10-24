import { describe, expect, it, vi, beforeEach } from 'vitest'
import { defineComponent } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useAdminUserRowActions } from '../useAdminUserRowActions'
import type { AdminUser } from '@/types'

const apiMocks = vi.hoisted(() => ({
  toggleStatus: vi.fn(),
  deleteUser: vi.fn()
}))

const storeMocks = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      toggleStatus: apiMocks.toggleStatus,
      delete: apiMocks.deleteUser
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => storeMocks
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

function makeUser(overrides: Partial<AdminUser> = {}): AdminUser {
  return {
    id: 99,
    email: 'u@example.com',
    username: 'u',
    role: 'user',
    status: 'active',
    balance: 1,
    concurrency: 1,
    ...overrides
  } as AdminUser
}

function mountActions(user: AdminUser, onChanged = vi.fn(), onDeleted = vi.fn()) {
  const Comp = defineComponent({
    setup() {
      return useAdminUserRowActions({
        getUser: () => user,
        onChanged,
        onDeleted
      })
    },
    template: '<div />'
  })
  return { wrapper: mount(Comp), onChanged, onDeleted }
}

describe('useAdminUserRowActions', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    apiMocks.toggleStatus.mockResolvedValue({})
    apiMocks.deleteUser.mockResolvedValue({ message: 'ok' })
  })

  it('toggles status only after confirmation', async () => {
    const { wrapper, onChanged } = mountActions(makeUser())
    await wrapper.vm.handleToggleStatus()
    expect(apiMocks.toggleStatus).not.toHaveBeenCalled()
    expect(wrapper.vm.showToggleStatusDialog).toBe(true)

    wrapper.vm.cancelToggleStatus()
    expect(apiMocks.toggleStatus).not.toHaveBeenCalled()
    expect(wrapper.vm.showToggleStatusDialog).toBe(false)

    await wrapper.vm.handleToggleStatus()
    await wrapper.vm.confirmToggleStatus()
    await flushPromises()
    expect(apiMocks.toggleStatus).toHaveBeenCalledWith(99, 'disabled')
    expect(onChanged).toHaveBeenCalledTimes(1)
    expect(storeMocks.showSuccess).toHaveBeenCalledWith('admin.users.userDisabled')
    expect(wrapper.vm.showToggleStatusDialog).toBe(false)
  })

  it('enables a disabled user only after confirmation', async () => {
    const { wrapper } = mountActions(makeUser({ status: 'disabled' }))
    await wrapper.vm.handleToggleStatus()
    await wrapper.vm.confirmToggleStatus()
    await flushPromises()
    expect(apiMocks.toggleStatus).toHaveBeenCalledWith(99, 'active')
    expect(storeMocks.showSuccess).toHaveBeenCalledWith('admin.users.userEnabled')
  })

  it('deletes the user and notifies the parent', async () => {
    const { wrapper, onDeleted } = mountActions(makeUser())
    await wrapper.vm.confirmDelete()
    await flushPromises()
    expect(apiMocks.deleteUser).toHaveBeenCalledWith(99)
    expect(onDeleted).toHaveBeenCalledTimes(1)
  })
})
