import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ChannelLoadtestView from '@/views/admin/ChannelLoadtestView.vue'

const { startRun, listAccounts, listProxies, exportExcel } = vi.hoisted(() => ({
  startRun: vi.fn(),
  listAccounts: vi.fn(),
  listProxies: vi.fn(),
  exportExcel: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    channelLoadtest: {
      start: startRun,
      get: vi.fn(),
      latest: vi.fn().mockResolvedValue(null),
      stop: vi.fn(),
      exportExcel
    }
  }
}))

vi.stubGlobal('URL', {
  createObjectURL: vi.fn(() => 'blob:mock'),
  revokeObjectURL: vi.fn()
})
HTMLAnchorElement.prototype.click = vi.fn()

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
    expect(payload.total).toBe(40)
    expect(payload.size_cap).toBe(0)
    expect(payload.input_tokens).toBe(0)
    expect(payload.tiers).toEqual([{ input_tokens: 80, count: 40 }])
  })

  it('fills absolute tiers when selecting a traffic profile', async () => {
    const wrapper = await mountStarted()
    await wrapper.get('[data-testid=loadtest-profile]').setValue('user363-sla')
    await flushPromises()
    const inputs = wrapper.findAll('[data-testid=tier-input]')
    const counts = wrapper.findAll('[data-testid=tier-count]')
    expect(inputs).toHaveLength(4)
    expect(Number((inputs[0].element as HTMLInputElement).value)).toBe(50000)
    expect(Number((counts[0].element as HTMLInputElement).value)).toBe(50)
    expect(Number((inputs[3].element as HTMLInputElement).value)).toBe(380000)
    expect(Number((counts[3].element as HTMLInputElement).value)).toBe(2)
  })

  it('auto-selects api mode from the model list', async () => {
    const wrapper = await mountStarted()
    expect((wrapper.get('[data-testid=loadtest-api-mode]').element as HTMLSelectElement).value).toBe('chat_completions')
    await wrapper.get('[data-testid=loadtest-models]').setValue('glm-5.3')
    await flushPromises()
    expect((wrapper.get('[data-testid=loadtest-api-mode]').element as HTMLSelectElement).value).toBe('responses')
    await wrapper.get('[data-testid=loadtest-models]').setValue('kimi-k3')
    await flushPromises()
    expect((wrapper.get('[data-testid=loadtest-api-mode]').element as HTMLSelectElement).value).toBe('chat_completions')
  })

  it('fills the general balanced tier mix', async () => {
    const wrapper = await mountStarted()
    await wrapper.get('[data-testid=loadtest-profile]').setValue('general')
    await flushPromises()
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    const payload = startRun.mock.calls[0][0]
    expect(payload.profile).toBe('general')
    expect(payload.stream_mode).toBe('stream')
    expect(payload.total).toBe(100)
    expect(payload.tiers).toEqual([
      { input_tokens: 4000, count: 40 },
      { input_tokens: 16000, count: 30 },
      { input_tokens: 32000, count: 20 },
      { input_tokens: 64000, count: 10 }
    ])
  })

  it('opens an export dialog and downloads with selected options', async () => {
    exportExcel.mockResolvedValue({
      blob: new Blob(['xlsx']),
      filename: '20260925-100000_kimi-k3.xlsx'
    })
    startRun.mockResolvedValue({
      id: 'run-done',
      status: 'done',
      done: 10,
      ok: 10,
      success_rate: 100,
      total: 10,
      peak: 2,
      inflight: 0,
      models: ['kimi-k3'],
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
    await wrapper.get('[data-testid=export-excel]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid=export-modal]').exists()).toBe(true)
    await wrapper.get('[data-testid=export-opt-conditions]').setValue(false)
    await wrapper.get('[data-testid=export-confirm]').trigger('click')
    await flushPromises()
    expect(exportExcel).toHaveBeenCalled()
    const [, options] = exportExcel.mock.calls[0]
    expect(options.include_conditions).toBe(false)
    expect(options.include_overview).toBe(true)
    expect(wrapper.find('[data-testid=export-modal]').exists()).toBe(false)
  })

  async function mountStarted() {
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
    return wrapper
  }

  it('sends the SLA sheet as absolute tier counts', async () => {
    const wrapper = await mountStarted()
    await wrapper.get('[data-testid=apply-sla-preset]').trigger('click')
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    const payload = startRun.mock.calls[0][0]
    expect(payload.profile).toBe('user363-sla')
    expect(payload.stream_mode).toBe('stream')
    expect(payload.total).toBe(100)
    expect(payload.max_tokens).toBe(256)
    expect(payload.tiers).toEqual([
      { input_tokens: 50000, count: 50 },
      { input_tokens: 80000, count: 38 },
      { input_tokens: 160000, count: 10 },
      { input_tokens: 380000, count: 2 }
    ])
  })

  it('derives total from edited tier counts', async () => {
    const wrapper = await mountStarted()
    // Replace smoke's single row with two custom rows.
    await wrapper.get('[data-testid=reset-tiers]').trigger('click')
    const firstCount = wrapper.findAll('[data-testid=tier-count]')[0]
    await firstCount.setValue(5)
    await wrapper.get('[data-testid=add-tier]').trigger('click')
    const inputs = wrapper.findAll('[data-testid=tier-input]')
    const counts = wrapper.findAll('[data-testid=tier-count]')
    await inputs[1].setValue(90000)
    await counts[1].setValue(4)
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    const payload = startRun.mock.calls[0][0]
    expect(payload.total).toBe(9)
    expect(payload.size_cap).toBe(0)
    expect(payload.input_tokens).toBe(0)
    expect(payload.tiers).toEqual([
      { input_tokens: 80, count: 5 },
      { input_tokens: 90000, count: 4 }
    ])
  })

  it('resets tiers back to the current profile mix', async () => {
    const wrapper = await mountStarted()
    await wrapper.get('[data-testid=loadtest-profile]').setValue('general')
    await flushPromises()
    await wrapper.get('[data-testid=add-tier]').trigger('click')
    expect(wrapper.findAll('[data-testid=tier-input]').length).toBe(5)
    await wrapper.get('[data-testid=reset-tiers]').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('[data-testid=tier-input]').length).toBe(4)
    expect(Number((wrapper.findAll('[data-testid=tier-input]')[0].element as HTMLInputElement).value)).toBe(4000)
  })
})
