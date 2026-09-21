import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ChannelLoadtestView from '@/views/admin/ChannelLoadtestView.vue'

const { startRun, listAccounts, listProxies } = vi.hoisted(() => ({
  startRun: vi.fn(),
  listAccounts: vi.fn(),
  listProxies: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    channelLoadtest: {
      start: startRun,
      get: vi.fn(),
      latest: vi.fn().mockResolvedValue(null),
      stop: vi.fn()
    }
  }
}))

vi.mock('@/api/admin/accounts', () => ({
  list: (...args: unknown[]) => listAccounts(...args)
}))

vi.mock('@/api/admin/proxies', () => ({
  getAll: (...args: unknown[]) => listProxies(...args)
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const AppLayoutStub = defineComponent({
  template: '<main><slot /></main>'
})

describe('ChannelLoadtestView', () => {
  beforeEach(() => {
    startRun.mockReset()
    listAccounts.mockReset()
    listAccounts.mockResolvedValue({ items: [{ id: 12, name: 'kimi-prod', platform: 'kimi', type: 'apikey', proxy_id: null }] })
    listProxies.mockResolvedValue([{ id: 3, name: 'hk-1', protocol: 'http', host: '127.0.0.1', port: 7890 }])
  })

  it('starts a run with selected account', async () => {
    startRun.mockResolvedValue({
      id: 'run-1',
      status: 'running',
      inflight: 0,
      peak: 0,
      done: 0,
      ok: 0,
      success_rate: 0,
      sla: [],
      results: []
    })
    const wrapper = mount(ChannelLoadtestView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Icon: true
        }
      }
    })
    await flushPromises()
    await wrapper.get('select').setValue(12)
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    expect(startRun).toHaveBeenCalled()
    const payload = startRun.mock.calls[0][0]
    expect(payload.account_id).toBe(12)
    expect(payload.profile).toBe('smoke')
    expect(payload.proxy_id).toBe(0)
    expect(payload.api_mode).toBe('chat_completions')
    expect(payload.stream_mode).toBe('auto')
  })
})
