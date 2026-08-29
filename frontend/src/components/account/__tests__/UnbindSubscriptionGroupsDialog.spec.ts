import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import UnbindSubscriptionGroupsDialog from '../UnbindSubscriptionGroupsDialog.vue'
import { adminAPI } from '@/api/admin'

const showSuccess = vi.fn()
const showError = vi.fn()

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    showInfo: vi.fn()
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      unbindSubscriptionGroupsByRate: vi.fn()
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

function previewResult() {
  return {
    matched: 1,
    would_apply: 1,
    skipped_no_subscription: 0,
    skipped_would_be_empty: 0,
    applied: 0,
    failed: 0,
    accounts: [
      {
        id: 7,
        name: 'high-rate',
        platform: 'anthropic',
        rate: 1.2,
        action: 'preview',
        remove_groups: [{ id: 20, name: 'subscription-b' }],
        keep_groups: [{ id: 10, name: 'standard-a' }],
        would_be_empty: false
      }
    ]
  }
}

function mountDialog() {
  return mount(UnbindSubscriptionGroupsDialog, {
    props: { show: true },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' }
      }
    }
  })
}

describe('UnbindSubscriptionGroupsDialog', () => {
  beforeEach(() => {
    showSuccess.mockReset()
    showError.mockReset()
    vi.mocked(adminAPI.accounts.unbindSubscriptionGroupsByRate).mockReset()
    vi.mocked(adminAPI.accounts.unbindSubscriptionGroupsByRate)
      .mockResolvedValueOnce(previewResult() as any)
      .mockResolvedValueOnce({
        ...previewResult(),
        applied: 1,
        accounts: [{ ...previewResult().accounts[0], action: 'applied' }]
      } as any)
  })

  it('previews then confirms apply with the same threshold', async () => {
    const wrapper = mountDialog()
    const threshold = wrapper.get('[data-testid="unbind-subscription-threshold"]')
    await threshold.setValue('1.2')

    await wrapper.get('[data-testid="unbind-subscription-preview"]').trigger('click')
    await flushPromises()

    expect(adminAPI.accounts.unbindSubscriptionGroupsByRate).toHaveBeenCalledWith({
      min_rate_multiplier: 1.2,
      platform: '',
      dry_run: true,
      allow_empty_groups: false
    })
    expect(wrapper.get('[data-testid="unbind-subscription-table"]').text()).toContain('high-rate')
    expect(wrapper.get('[data-testid="unbind-subscription-table"]').text()).toContain('subscription-b')

    await wrapper.get('[data-testid="unbind-subscription-apply"]').trigger('click')
    await flushPromises()

    expect(adminAPI.accounts.unbindSubscriptionGroupsByRate).toHaveBeenLastCalledWith({
      min_rate_multiplier: 1.2,
      platform: '',
      dry_run: false,
      allow_empty_groups: false
    })
    expect(wrapper.emitted('applied')).toHaveLength(1)
  })

  it('uses the shared Select for platform instead of a native dropdown', () => {
    const wrapper = mountDialog()
    expect(wrapper.find('select').exists()).toBe(false)
    expect(wrapper.get('[data-testid="unbind-subscription-platform"]').exists()).toBe(true)
  })
})
