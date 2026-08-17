import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
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
vi.mock('@/components/common/ConfirmDialog.vue', () => ({ default: { template: '<div />' } }))
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
})
