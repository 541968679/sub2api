import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'

const { createAccountMock, checkMixedChannelRiskMock } = vi.hoisted(() => ({
  createAccountMock: vi.fn(),
  checkMixedChannelRiskMock: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    isSimpleMode: true
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      create: createAccountMock,
      checkMixedChannelRisk: checkMixedChannelRiskMock
    },
    settings: {
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false, providers: [] }),
      getSettings: vi.fn().mockResolvedValue({})
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([])
    }
  }
}))

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn().mockResolvedValue([])
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

vi.mock('@/composables/useAccountOAuth', () => ({
  useAccountOAuth: () => mockOAuthComposable(),
}))

vi.mock('@/composables/useOpenAIOAuth', () => ({
  useOpenAIOAuth: () => mockOAuthComposable(),
}))

vi.mock('@/composables/useGeminiOAuth', () => ({
  useGeminiOAuth: () => mockOAuthComposable(),
}))

vi.mock('@/composables/useAntigravityOAuth', () => ({
  useAntigravityOAuth: () => mockOAuthComposable(),
}))

import CreateAccountModal from '../CreateAccountModal.vue'

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: {
    show: {
      type: Boolean,
      default: false
    }
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

const ModelWhitelistSelectorStub = defineComponent({
  name: 'ModelWhitelistSelector',
  props: {
    modelValue: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:modelValue'],
  template: '<div />'
})

function mockOAuthComposable() {
  return {
    authUrl: { value: '' },
    sessionId: { value: '' },
    loading: { value: false },
    error: { value: '' },
    oauthState: { value: '' },
    resetState: vi.fn(),
    generateAuthUrl: vi.fn(),
    exchangeAuthCode: vi.fn(),
    validateRefreshToken: vi.fn(),
    validateGoogleOneRefreshToken: vi.fn(),
    buildCredentials: vi.fn(),
    buildExtraInfo: vi.fn()
  }
}

async function mountOpenAIAPIKeyModal() {
  createAccountMock.mockReset()
  checkMixedChannelRiskMock.mockReset()
  checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
  createAccountMock.mockResolvedValue({})

  const wrapper = mount(CreateAccountModal, {
    props: {
      show: true,
      proxies: [],
      groups: []
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        ConfirmDialog: BaseDialogStub,
        Icon: true,
        ProxySelector: true,
        GroupSelector: true,
        ModelWhitelistSelector: ModelWhitelistSelectorStub,
        OAuthAuthorizationFlow: true,
        Select: true
      }
    }
  })

  await wrapper.findAll('button').find((button) => button.text().includes('OpenAI'))!.trigger('click')
  await wrapper.findAll('button').find((button) => button.text().includes('API Key'))!.trigger('click')
  await wrapper.get('input[type="password"]').setValue('sk-test')
  return wrapper
}

describe('CreateAccountModal', () => {
  it('omits OpenAI images endpoint toggle when enabled by default', async () => {
    const wrapper = await mountOpenAIAPIKeyModal()

    await wrapper.get('form#create-account-form').trigger('submit.prevent')

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.extra?.openai_images_endpoint_enabled).toBeUndefined()
  })

  it('submits disabled OpenAI images endpoint scheduling', async () => {
    const wrapper = await mountOpenAIAPIKeyModal()

    await wrapper.get('[data-testid="openai-images-endpoint-enabled"]').trigger('click')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.extra?.openai_images_endpoint_enabled).toBe(false)
  })
})
