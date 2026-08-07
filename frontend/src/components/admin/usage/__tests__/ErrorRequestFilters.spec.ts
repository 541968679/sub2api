import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'

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

vi.mock('@/components/common/Select.vue', () => ({
  default: {
    name: 'Select',
    props: ['modelValue', 'options', 'searchable'],
    emits: ['update:modelValue', 'change'],
    template: `
      <select
        data-test="select"
        :value="modelValue ?? ''"
        @change="$emit('update:modelValue', $event.target.value === '' ? null : Number.isNaN(Number($event.target.value)) ? $event.target.value : Number($event.target.value)); $emit('change')"
      >
        <option v-for="opt in options" :key="String(opt.value)" :value="opt.value ?? ''">{{ opt.label }}</option>
      </select>
    `
  }
}))

import ErrorRequestFilters, {
  type ErrorRequestFilterState
} from '../ErrorRequestFilters.vue'

const emptyFilters = (): ErrorRequestFilterState => ({
  user_id: undefined,
  api_key_id: undefined,
  model: null,
  account_id: undefined,
  group_id: null,
  platform: '',
  bridge: 'all',
  upstream_model: '',
  q: '',
  status_codes: []
})

describe('ErrorRequestFilters', () => {
  beforeEach(() => {
    listUsers.mockReset()
    searchUsers.mockReset()
    listAccounts.mockReset()
    listGroups.mockReset()
    getModelStats.mockReset()
    searchApiKeys.mockReset()

    listUsers.mockResolvedValue({
      items: [{ id: 1, email: 'alice@example.com' }],
      total: 1
    })
    searchUsers.mockResolvedValue([{ id: 2, email: 'bob@example.com', deleted: false }])
    listAccounts.mockResolvedValue({
      items: [{ id: 10, name: 'acct-alpha' }],
      total: 1
    })
    listGroups.mockResolvedValue({
      items: [{ id: 5, name: 'vip-group' }],
      total: 1
    })
    getModelStats.mockResolvedValue({
      models: [{ model: 'claude-sonnet-4' }]
    })
    searchApiKeys.mockResolvedValue([{ id: 99, name: 'main-key' }])
  })

  it('opens user dropdown on focus and selects user by name/email', async () => {
    const wrapper = mount(ErrorRequestFilters, {
      props: {
        modelValue: emptyFilters(),
        startDate: '2026-08-01',
        endDate: '2026-08-07'
      },
      attachTo: document.body
    })
    await flushPromises()

    const userInput = wrapper
      .findAll('input')
      .find((i) => i.attributes('placeholder') === 'admin.usage.searchUserPlaceholder')
    expect(userInput).toBeTruthy()
    await userInput!.trigger('focus')
    await flushPromises()

    expect(listUsers).toHaveBeenCalled()
    const option = Array.from(document.body.querySelectorAll('button')).find((el) =>
      el.textContent?.includes('alice@example.com')
    )
    expect(option).toBeTruthy()
    option!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()
    await nextTick()

    const emitted = wrapper.emitted('update:modelValue')
    expect(emitted).toBeTruthy()
    const last = emitted![emitted!.length - 1][0] as ErrorRequestFilterState
    expect(last.user_id).toBe(1)
    expect(wrapper.emitted('change')).toBeTruthy()

    wrapper.unmount()
  })

  it('opens account dropdown and selects by account name (not raw id input)', async () => {
    const wrapper = mount(ErrorRequestFilters, {
      props: {
        modelValue: emptyFilters(),
        startDate: '2026-08-01',
        endDate: '2026-08-07'
      },
      attachTo: document.body
    })
    await flushPromises()

    const accountInput = wrapper
      .findAll('input')
      .find((i) => i.attributes('placeholder') === 'admin.usage.searchAccountPlaceholder')
    expect(accountInput).toBeTruthy()
    await accountInput!.trigger('focus')
    await flushPromises()

    expect(listAccounts).toHaveBeenCalled()
    const option = Array.from(document.body.querySelectorAll('button')).find((el) =>
      el.textContent?.includes('acct-alpha')
    )
    expect(option).toBeTruthy()
    option!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()

    const emitted = wrapper.emitted('update:modelValue')
    const last = emitted![emitted!.length - 1][0] as ErrorRequestFilterState
    expect(last.account_id).toBe(10)

    // no raw "group id / account id" numeric placeholders
    const placeholders = wrapper.findAll('input').map((i) => i.attributes('placeholder') || '')
    expect(placeholders.some((p) => p.includes('idPlaceholder') || p === '数字 ID')).toBe(false)

    wrapper.unmount()
  })

  it('loads searchable group options by name', async () => {
    mount(ErrorRequestFilters, {
      props: {
        modelValue: emptyFilters(),
        startDate: '2026-08-01',
        endDate: '2026-08-07'
      }
    })
    await flushPromises()
    expect(listGroups).toHaveBeenCalled()
    expect(getModelStats).toHaveBeenCalled()
  })
})
