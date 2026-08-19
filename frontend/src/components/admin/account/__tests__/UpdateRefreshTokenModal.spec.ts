import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import UpdateRefreshTokenModal from '../UpdateRefreshTokenModal.vue'
import type { Account } from '@/types'

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess: vi.fn(),
    showError: vi.fn()
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      updateRefreshToken: vi.fn()
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

function makeAccount(overrides: Partial<Account> = {}): Account {
  return {
    id: 7,
    name: 'oai-1',
    platform: 'openai',
    type: 'oauth',
    proxy_id: null,
    concurrency: 1,
    priority: 50,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides
  }
}

function mountModal(account: Account) {
  return mount(UpdateRefreshTokenModal, {
    props: { show: true, account },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        Icon: { template: '<span />' }
      }
    }
  })
}

describe('UpdateRefreshTokenModal', () => {
  it('uses OpenAI session-JSON copy for OpenAI accounts', () => {
    const wrapper = mountModal(makeAccount({ platform: 'openai' }))
    const text = wrapper.text()
    expect(text).toContain('admin.accounts.updateRefreshTokenDescOpenAI')
    expect(text).toContain('admin.accounts.validateBeforeSaveHintOpenAI')
    expect(wrapper.get('textarea').attributes('placeholder')).toBe(
      'admin.accounts.updateRefreshTokenPlaceholderOpenAI'
    )
  })

  it('keeps generic copy for non-OpenAI accounts', () => {
    const wrapper = mountModal(makeAccount({ platform: 'gemini' }))
    const text = wrapper.text()
    expect(text).toContain('admin.accounts.updateRefreshTokenDesc')
    expect(text).not.toContain('admin.accounts.updateRefreshTokenDescOpenAI')
    expect(text).toContain('admin.accounts.validateBeforeSaveHint')
    expect(text).not.toContain('admin.accounts.validateBeforeSaveHintOpenAI')
    expect(wrapper.get('textarea').attributes('placeholder')).toBe(
      'admin.accounts.updateRefreshTokenPlaceholder'
    )
  })
})
