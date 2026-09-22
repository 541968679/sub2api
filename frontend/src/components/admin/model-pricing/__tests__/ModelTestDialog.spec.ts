import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ModelTestDialog from '../ModelTestDialog.vue'

const { listAccounts } = vi.hoisted(() => ({
  listAccounts: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts
    }
  }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token' })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        if (params?.endpoint) return `${key}:${params.endpoint}`
        return key
      }
    })
  }
})

describe('ModelTestDialog', () => {
  beforeEach(() => {
    listAccounts.mockReset()
    listAccounts.mockResolvedValue({
      items: [
        {
          id: 7,
          name: 'relay',
          platform: 'openai',
          type: 'apikey',
          status: 'active',
          schedulable: true,
          credentials: { email: 'relay@example.com' }
        }
      ]
    })
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      body: {
        getReader: () => ({
          read: vi.fn().mockResolvedValue({ done: true, value: undefined })
        })
      }
    }) as any
  })

  it('posts the selected Responses or Chat Completions endpoint for an OpenAI API key', async () => {
    const wrapper = mount(ModelTestDialog, {
      props: { show: false, model: 'gpt-5.4', provider: 'openai' },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /></div>' }
        }
      }
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    const endpointSelect = wrapper.findAll('select').find((select) => select.html().includes('chat_completions'))
    expect(endpointSelect).toBeTruthy()
    await endpointSelect!.setValue('responses')

    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(1)
    const [url, request] = (global.fetch as any).mock.calls[0]
    expect(url).toBe('/api/v1/admin/accounts/7/test')
    expect(JSON.parse(request.body)).toMatchObject({
      model_id: 'gpt-5.4',
      api_mode: 'responses'
    })
  })

  it('does not send api_mode for a non-OpenAI provider', async () => {
    const wrapper = mount(ModelTestDialog, {
      props: { show: false, model: 'claude-haiku-4-5', provider: 'anthropic' },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /></div>' }
        }
      }
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(wrapper.text()).not.toContain('admin.accounts.testEndpointChatCompletions')
    await wrapper.get('button').trigger('click')
    await flushPromises()

    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body).api_mode).toBeUndefined()
  })
})
