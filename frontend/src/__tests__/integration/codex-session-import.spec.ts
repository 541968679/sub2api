import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import CodexSessionImportModal from '@/components/admin/account/CodexSessionImportModal.vue'
import { adminAPI } from '@/api/admin'

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      importCodexSession: vi.fn()
    }
  }
}))

const showSuccess = vi.fn()
const showError = vi.fn()
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError })
}))

const BaseDialogStub = {
  props: ['show', 'title'],
  template: '<section v-if="show"><h2>{{ title }}</h2><slot/><slot name="footer"/></section>'
}

const mountModal = () => mount(CodexSessionImportModal, {
  props: {
    show: true,
    proxies: [{ id: 7, name: 'proxy-7' }],
    groups: [{ id: 9, name: 'OpenAI', platform: 'openai' }]
  },
  global: {
    stubs: {
      BaseDialog: BaseDialogStub,
      ProxySelector: { props: ['modelValue'], template: '<div data-test="proxy-selector" />' },
      GroupSelector: { props: ['modelValue'], template: '<div data-test="group-selector" />' }
    },
    mocks: {
      $t: (key: string) => key
    },
    plugins: [{
      install(app: any) {
        app.config.globalProperties.$t = (key: string) => key
        app.provide('i18n', { t: (key: string) => key })
      }
    }]
  }
})

describe('CodexSessionImportModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(adminAPI.accounts.importCodexSession).mockResolvedValue({
      total: 1,
      created: 1,
      updated: 0,
      skipped: 0,
      failed: 0,
      items: []
    })
  })

  it('submits session content and fork account bindings', async () => {
    const wrapper = mountModal()
    await wrapper.get('textarea').setValue('{"access_token":"token"}')
    await wrapper.get('input[name="name"]').setValue('Imported Codex')
    await wrapper.get('form').trigger('submit')

    expect(adminAPI.accounts.importCodexSession).toHaveBeenCalledWith(expect.objectContaining({
      content: '{"access_token":"token"}',
      name: 'Imported Codex',
      update_existing: true
    }))
    expect(wrapper.emitted('imported')).toHaveLength(1)
  })

  it('keeps partial failure details visible', async () => {
    vi.mocked(adminAPI.accounts.importCodexSession).mockResolvedValue({
      total: 2,
      created: 1,
      updated: 0,
      skipped: 0,
      failed: 1,
      items: [{ index: 2, action: 'failed', message: 'expired token' }]
    })
    const wrapper = mountModal()
    await wrapper.get('textarea').setValue('token-1\ntoken-2')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.text()).toContain('expired token')
    expect(showError).toHaveBeenCalled()
  })
})
