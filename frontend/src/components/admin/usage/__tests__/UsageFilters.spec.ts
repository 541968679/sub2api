import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const listUsers = vi.hoisted(() => vi.fn())
const searchUsers = vi.hoisted(() => vi.fn())
const listAccounts = vi.hoisted(() => vi.fn())
const listGroups = vi.hoisted(() => vi.fn())
const getModelStats = vi.hoisted(() => vi.fn())
const searchApiKeys = vi.hoisted(() => vi.fn())

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      list: (...args: unknown[]) => listUsers(...args),
      getById: vi.fn()
    },
    accounts: {
      list: (...args: unknown[]) => listAccounts(...args),
      getById: vi.fn()
    },
    groups: {
      list: (...args: unknown[]) => listGroups(...args)
    },
    dashboard: {
      getModelStats: (...args: unknown[]) => getModelStats(...args)
    },
    usage: {
      searchUsers: (...args: unknown[]) => searchUsers(...args),
      searchApiKeys: (...args: unknown[]) => searchApiKeys(...args)
    }
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ resolve: vi.fn() })
}))

vi.mock('@/components/common/Select.vue', () => ({
  default: {
    name: 'Select',
    props: ['modelValue', 'options', 'searchable'],
    emits: ['update:modelValue', 'change'],
    template: '<select data-test="select" />'
  }
}))

import UsageFilters from '../UsageFilters.vue'

const emptyFilters = () => ({
  user_id: undefined,
  model: undefined,
  group_id: undefined,
  request_type: undefined,
  billing_type: null,
  billing_mode: undefined
})

describe('UsageFilters locked identity', () => {
  beforeEach(() => {
    listUsers.mockReset()
    searchUsers.mockReset()
    listAccounts.mockReset()
    listGroups.mockReset()
    getModelStats.mockReset()
    searchApiKeys.mockReset()
    listUsers.mockResolvedValue({ items: [], total: 0 })
    searchUsers.mockResolvedValue([])
    listAccounts.mockResolvedValue({ items: [], total: 0 })
    listGroups.mockResolvedValue({ items: [], total: 0 })
    getModelStats.mockResolvedValue({ models: [] })
    searchApiKeys.mockResolvedValue([])
  })

  it('renders a locked user chip and hides the searchable user input', async () => {
    const wrapper = mount(UsageFilters, {
      props: {
        modelValue: { ...emptyFilters(), user_id: 1 },
        exporting: false,
        startDate: '2026-08-01',
        endDate: '2026-08-07',
        lockedUserId: 1,
        lockedUserLabel: 'alice@example.com'
      }
    })
    await flushPromises()

    expect(wrapper.find('[data-testid="locked-user-filter"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="locked-user-filter"]').text()).toContain('alice@example.com')
    const userInput = wrapper
      .findAll('input')
      .find((i) => i.attributes('placeholder') === 'admin.usage.searchUserPlaceholder')
    expect(userInput).toBeUndefined()
    wrapper.unmount()
  })

  it('renders a locked account chip and hides export/cleanup when those actions are off', async () => {
    const wrapper = mount(UsageFilters, {
      props: {
        modelValue: { ...emptyFilters(), account_id: 10 },
        exporting: false,
        startDate: '2026-08-01',
        endDate: '2026-08-07',
        showExport: false,
        showCleanup: false,
        lockedAccountId: 10,
        lockedAccountLabel: 'acct-alpha'
      }
    })
    await flushPromises()

    expect(wrapper.find('[data-testid="locked-account-filter"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="locked-account-filter"]').text()).toContain('acct-alpha')
    expect(wrapper.text()).toContain('common.refresh')
    expect(wrapper.text()).toContain('common.reset')
    expect(wrapper.text()).not.toContain('usage.exportExcel')
    expect(wrapper.text()).not.toContain('admin.usage.cleanup.button')
    wrapper.unmount()
  })
})
