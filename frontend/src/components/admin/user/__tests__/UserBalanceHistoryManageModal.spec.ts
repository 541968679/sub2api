import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'

const getUserBalanceHistory = vi.hoisted(() => vi.fn())
const deleteRedeem = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      getUserBalanceHistory: (...args: unknown[]) => getUserBalanceHistory(...args)
    },
    redeem: {
      delete: (...args: unknown[]) => deleteRedeem(...args)
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        if (!params) return key
        return key.replace(/\{(\w+)\}/g, (_, k) => params[k] ?? '')
      }
    })
  }
})

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: {
    name: 'BaseDialog',
    props: ['show', 'title'],
    template: '<div v-if="show"><h1>{{ title }}</h1><slot /></div>'
  }
}))

vi.mock('@/components/common/ConfirmDialog.vue', () => ({
  default: {
    name: 'ConfirmDialog',
    props: ['show', 'title', 'message'],
    template:
      '<div v-if="show" data-test="confirm"><p>{{ message }}</p><button data-test="confirm-btn" @click="$emit(\'confirm\')">ok</button><button data-test="cancel-btn" @click="$emit(\'cancel\')">cancel</button></div>'
  }
}))

vi.mock('@/components/common/Select.vue', () => ({
  default: { name: 'Select', template: '<div />' }
}))

vi.mock('@/components/icons/Icon.vue', () => ({
  default: { name: 'Icon', template: '<i />' }
}))

import UserBalanceHistoryManageModal from '../UserBalanceHistoryManageModal.vue'

const sampleUser = {
  id: 7,
  email: 'user@example.com',
  username: 'demo',
  balance: 12.5,
  created_at: '2026-01-01T00:00:00Z'
}

const sampleItem = {
  id: 101,
  code: 'ABCD1234EFGH',
  type: 'admin_balance',
  value: 10,
  notes: 'manual topup',
  used_at: '2026-02-01T00:00:00Z',
  created_at: '2026-02-01T00:00:00Z'
}

async function mountAndOpen() {
  const wrapper = mount(UserBalanceHistoryManageModal, {
    props: {
      show: false,
      user: sampleUser as any
    }
  })
  await wrapper.setProps({ show: true })
  await flushPromises()
  return wrapper
}

describe('UserBalanceHistoryManageModal', () => {
  beforeEach(() => {
    getUserBalanceHistory.mockReset()
    deleteRedeem.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    getUserBalanceHistory.mockResolvedValue({
      items: [sampleItem],
      total: 1,
      page: 1,
      page_size: 15,
      pages: 1,
      total_recharged: 10
    })
    deleteRedeem.mockResolvedValue({ message: 'ok' })
  })

  it('loads history when opened and can delete a record', async () => {
    const wrapper = await mountAndOpen()

    expect(getUserBalanceHistory).toHaveBeenCalledWith(7, 1, 15, undefined)
    expect(wrapper.text()).toContain('admin.users.balanceHistoryManageTitle')

    const deleteBtn = wrapper.find('button[title="admin.users.deleteBalanceHistoryRecord"]')
    expect(deleteBtn.exists()).toBe(true)
    await deleteBtn.trigger('click')
    await nextTick()

    expect(wrapper.find('[data-test="confirm"]').exists()).toBe(true)
    await wrapper.get('[data-test="confirm-btn"]').trigger('click')
    await flushPromises()

    expect(deleteRedeem).toHaveBeenCalledWith(101)
    expect(showSuccess).toHaveBeenCalledWith('admin.users.deleteBalanceHistorySuccess')
    expect(getUserBalanceHistory).toHaveBeenCalledTimes(2)
  })
})
