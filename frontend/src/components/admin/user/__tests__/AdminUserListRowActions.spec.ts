import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { adminAPI } from '@/api/admin'
import AdminUserListRowActions from '../AdminUserListRowActions.vue'
import type { AdminUser } from '@/types'

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      toggleStatus: vi.fn(),
      delete: vi.fn()
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn()
  })
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

vi.mock('@/components/icons/Icon.vue', () => ({ default: { template: '<span />' } }))
vi.mock('@/components/common/ConfirmDialog.vue', () => ({
  default: {
    name: 'ConfirmDialog',
    props: ['show', 'title', 'message', 'danger', 'confirmText'],
    emits: ['confirm', 'cancel'],
    template:
      '<div v-if="show" class="confirm-dialog" :data-title="title" :data-message="message"><button class="confirm" @click="$emit(\'confirm\')" /><button class="cancel" @click="$emit(\'cancel\')" /></div>'
  }
}))
vi.mock('@/components/admin/user/UserEditModal.vue', () => ({
  default: {
    props: ['show', 'user'],
    template: '<div v-if="show" data-testid="user-edit-modal" />'
  }
}))
vi.mock('@/components/admin/user/UserApiKeysModal.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/admin/user/UserAllowedGroupsModal.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/admin/user/UserModelPricingModal.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/admin/user/UserPlatformQuotaModal.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/admin/user/UserBalanceModal.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/admin/user/UserBalanceHistoryModal.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/admin/user/UserBalanceHistoryManageModal.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/admin/usage/UsageErrorInspectDialog.vue', () => ({
  default: {
    props: ['show', 'scope'],
    template: '<div v-if="show" data-testid="user-usage-inspect" />'
  }
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

describe('AdminUserListRowActions', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders the users-list action bar and opens edit', async () => {
    const wrapper = mount(AdminUserListRowActions, { props: { user: makeUser() } })
    expect(wrapper.get('[data-testid="admin-user-list-row-actions"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="user-row-edit"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="user-view-usage"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="user-view-error-requests"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="user-row-toggle-status"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="user-row-more"]').exists()).toBe(true)
    await wrapper.get('[data-testid="user-row-edit"]').trigger('click')
    expect(wrapper.get('[data-testid="user-edit-modal"]').exists()).toBe(true)
  })

  it('opens a confirm dialog before disabling and does nothing on cancel', async () => {
    const toggleStatus = adminAPI.users.toggleStatus as ReturnType<typeof vi.fn>
    toggleStatus.mockReset()
    toggleStatus.mockResolvedValue({})
    const wrapper = mount(AdminUserListRowActions, { props: { user: makeUser() } })

    await wrapper.get('[data-testid="user-row-toggle-status"]').trigger('click')
    const dialog = wrapper.get('.confirm-dialog')
    expect(dialog.attributes('data-title')).toBe('admin.users.disableUser')
    expect(dialog.attributes('data-message')).toBe('admin.users.disableConfirm')
    expect(toggleStatus).not.toHaveBeenCalled()

    await dialog.get('.cancel').trigger('click')
    expect(wrapper.find('.confirm-dialog').exists()).toBe(false)
    expect(toggleStatus).not.toHaveBeenCalled()
  })
})
