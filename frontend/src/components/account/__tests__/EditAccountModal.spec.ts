import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

const {
  updateAccountMock,
  checkMixedChannelRiskMock,
  getQualityHardCloseSettings,
  updateQualityHardCloseSettings,
  resumeUserQuality,
  showSuccess
} = vi.hoisted(() => ({
  updateAccountMock: vi.fn(),
  checkMixedChannelRiskMock: vi.fn(),
  getQualityHardCloseSettings: vi.fn(),
  updateQualityHardCloseSettings: vi.fn(),
  resumeUserQuality: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess,
    showInfo: vi.fn()
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
      update: updateAccountMock,
      checkMixedChannelRisk: checkMixedChannelRiskMock,
      resumeUserQuality
    },
    settings: {
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false, providers: [] }),
      getSettings: vi.fn().mockResolvedValue({}),
      getQualityHardCloseSettings,
      updateQualityHardCloseSettings
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([])
    }
  }
}))

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn()
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

import EditAccountModal from '../EditAccountModal.vue'

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
  template: `
    <div>
      <button
        type="button"
        data-testid="rewrite-to-snapshot"
        @click="$emit('update:modelValue', ['gpt-5.2-2025-12-11'])"
      >
        rewrite
      </button>
      <span data-testid="model-whitelist-value">
        {{ Array.isArray(modelValue) ? modelValue.join(',') : '' }}
      </span>
    </div>
  `
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    modelValue: {
      type: [String, Number, Boolean, null],
      default: ''
    },
    options: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:modelValue'],
  template: `
    <select
      v-bind="$attrs"
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value)"
    >
      <option v-for="option in options" :key="option.value" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  `
})

function buildAccount() {
  return {
    id: 1,
    name: 'OpenAI Key',
    notes: '',
    platform: 'openai',
    type: 'apikey',
    credentials: {
      api_key: 'sk-test',
      base_url: 'https://api.openai.com',
      model_mapping: {
        'gpt-5.2': 'gpt-5.2'
      }
    },
    extra: {},
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    rate_multiplier: 1,
    status: 'active',
    group_ids: [],
    expires_at: null,
    auto_pause_on_expired: false
  } as any
}

function buildGroups() {
  return [
    { id: 10, name: 'OpenAI Group', platform: 'openai', rate_multiplier: 1, account_count: 1 },
    { id: 20, name: 'Antigravity Group', platform: 'antigravity', rate_multiplier: 1, account_count: 1 },
    { id: 30, name: 'Grok Group', platform: 'grok', rate_multiplier: 1, account_count: 1 }
  ] as any[]
}

function mountModal(account = buildAccount(), groups: any[] = []) {
  return mount(EditAccountModal, {
    props: {
      show: true,
      account,
      proxies: [],
      groups
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        Select: SelectStub,
        Icon: true,
        ProxySelector: true,
        GroupSelector: true,
        ModelWhitelistSelector: ModelWhitelistSelectorStub,
        OpenAIFastPolicyUserSelector: {
          props: ['modelValue', 'knownUsers'],
          emits: ['update:modelValue'],
          template: '<div data-testid="user-schedule-selector-stub" />'
        },
        AccountSchedulePanel: {
          props: ['account'],
          template: '<div data-testid="edit-account-schedule-panel" />'
        }
      }
    }
  })
}

describe('EditAccountModal', () => {
  it('shows the Responses probe result and can force the native Responses endpoint', async () => {
    const account = buildAccount()
    account.extra = {
      openai_responses_supported: false,
      preserved_setting: 'keep-me'
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    const modeSelect = wrapper.get<HTMLSelectElement>(
      '[data-testid="edit-openai-responses-mode"]'
    )

    expect(modeSelect.element.value).toBe('auto')
    expect(wrapper.get('[data-testid="edit-openai-responses-probe-status"]').text()).toContain(
      'admin.accounts.openai.responsesProbeUnsupported'
    )

    await modeSelect.setValue('force_responses')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).toMatchObject({
      openai_responses_mode: 'force_responses',
      openai_responses_supported: false,
      preserved_setting: 'keep-me'
    })
  })

  it('hydrates passthrough and shows both probe results', async () => {
    const account = buildAccount()
    account.extra = {
      openai_responses_mode: 'passthrough',
      openai_responses_supported: true,
      openai_chat_completions_supported: false,
      preserved_setting: 'keep-me'
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    const modeSelect = wrapper.get<HTMLSelectElement>(
      '[data-testid="edit-openai-responses-mode"]'
    )

    expect(modeSelect.element.value).toBe('passthrough')
    expect(wrapper.get('[data-testid="edit-openai-responses-probe-status"]').text()).toContain(
      'admin.accounts.openai.responsesProbeSupported'
    )
    expect(wrapper.get('[data-testid="edit-openai-chat-completions-probe-status"]').text()).toContain(
      'admin.accounts.openai.chatCompletionsProbeUnsupported'
    )

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).toMatchObject({
      openai_responses_mode: 'passthrough',
      openai_responses_supported: true,
      openai_chat_completions_supported: false,
      preserved_setting: 'keep-me'
    })
  })

  it('hydrates and can reset Anthropic API key bearer auth', async () => {
    const account = buildAccount()
    account.name = 'Anthropic Compatible'
    account.platform = 'anthropic'
    account.credentials = {
      api_key: 'anthropic-test',
      base_url: 'https://compatible.example.com'
    }
    account.extra = {
      anthropic_apikey_auth_scheme: 'authorization_bearer',
      anthropic_passthrough: true
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    const authScheme = wrapper.get<HTMLSelectElement>(
      '[data-testid="edit-anthropic-api-key-auth-scheme"]'
    )
    expect(authScheme.element.value).toBe('authorization_bearer')

    await authScheme.setValue('x_api_key')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.anthropic_apikey_auth_scheme).toBeUndefined()
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.anthropic_passthrough).toBe(true)
  })

  it('reopening the same account rehydrates the OpenAI whitelist from props', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2')

    await wrapper.get('[data-testid="rewrite-to-snapshot"]').trigger('click')
    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2-2025-12-11')

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })

    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_mapping).toEqual({
      'gpt-5.2': 'gpt-5.2'
    })
  })

  it('emits submitted model_mapping even when update response omits credentials', async () => {
    const account = buildAccount()
    account.credentials = {
      api_key: 'sk-test',
      base_url: 'https://api.openai.com'
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    // Simulate a buggy/partial API response without credentials.model_mapping.
    updateAccountMock.mockResolvedValue({
      ...account,
      credentials: { api_key: 'sk-test', base_url: 'https://api.openai.com' }
    })

    const wrapper = mountModal(account)
    await wrapper.get('[data-testid="rewrite-to-snapshot"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_mapping).toEqual({
      'gpt-5.2-2025-12-11': 'gpt-5.2-2025-12-11'
    })

    const emitted = wrapper.emitted('updated')
    expect(emitted).toBeTruthy()
    const patched = emitted?.[0]?.[0] as any
    expect(patched?.credentials?.model_mapping).toEqual({
      'gpt-5.2-2025-12-11': 'gpt-5.2-2025-12-11'
    })

    // Reopen with the patched list-row account: mapping must rehydrate.
    await wrapper.setProps({ show: false, account: null })
    await wrapper.setProps({ show: true, account: patched })
    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2-2025-12-11')
  })

  it('rehydrates OpenAI OAuth mapping mode from credentials.model_mapping', async () => {
    const account = buildAccount()
    account.type = 'oauth'
    account.credentials = {
      access_token: 'at',
      refresh_token: 'rt',
      model_mapping: {
        'gpt-5.6-luna': 'gpt-5.5'
      }
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    // Mapping mode is selected when from !== to; inputs should carry values.
    const fromInput = wrapper.findAll('input').find((input) => (input.element as HTMLInputElement).value === 'gpt-5.6-luna')
    const toInput = wrapper.findAll('input').find((input) => (input.element as HTMLInputElement).value === 'gpt-5.5')
    expect(fromInput).toBeTruthy()
    expect(toInput).toBeTruthy()
  })

  it('preserves Grok OAuth credentials and model mapping when edited', async () => {
    const account = buildAccount()
    account.name = 'Grok OAuth'
    account.platform = 'grok'
    account.type = 'oauth'
    account.credentials = {
      access_token: 'access-token',
      refresh_token: 'refresh-token',
      base_url: 'https://api.x.ai/v1',
      model_mapping: { 'grok-4': 'grok-4-fast' }
    }
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toMatchObject({
      access_token: 'access-token',
      refresh_token: 'refresh-token',
      base_url: 'https://api.x.ai/v1',
      model_mapping: { 'grok-4': 'grok-4-fast' }
    })
  })

  it('submits OpenAI compact mode and compact-only model mapping', async () => {
    const account = buildAccount()
    account.extra = {
      openai_compact_mode: 'force_on'
    }
    account.credentials = {
      ...account.credentials,
      compact_model_mapping: {
        'gpt-5.4': 'gpt-5.4-openai-compact'
      }
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_compact_mode).toBe('force_on')
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.compact_model_mapping).toEqual({
      'gpt-5.4': 'gpt-5.4-openai-compact'
    })
  })

  it('submits OpenAI Claude-GPT bridge flag and preserves model mapping', async () => {
    const account = buildAccount()
    account.credentials = {
      ...account.credentials,
      model_mapping: {
        'claude-opus-4-8': 'gpt-5.5'
      }
    }
    account.extra = {
      openai_claude_gpt_bridge_enabled: true
    }
    account.group_ids = [12]
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_claude_gpt_bridge_enabled).toBe(
      true
    )
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_mapping).toEqual({
      'claude-opus-4-8': 'gpt-5.5'
    })
    expect(updateAccountMock.mock.calls[0]?.[1]?.group_ids).toEqual([12])
  })

  async function openEditModal(account: any, groups = buildGroups()) {
    // Mirror real AccountsView flow: modal stays mounted closed, then opens
    // with an account so platform/bridge watchers observe a real transition.
    const wrapper = mount(EditAccountModal, {
      props: {
        show: false,
        account: null,
        proxies: [],
        groups
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Select: SelectStub,
          Icon: true,
          ProxySelector: true,
          GroupSelector: true,
          ModelWhitelistSelector: ModelWhitelistSelectorStub
        }
      }
    })
    await wrapper.setProps({ show: true, account })
    await wrapper.vm.$nextTick()
    return wrapper
  }

  it('preserves same-platform group selections when opening edit (OpenAI)', async () => {
    const account = buildAccount()
    account.group_ids = [10]
    account.extra = {}
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = await openEditModal(account)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.group_ids).toEqual([10])
  })

  it('preserves same-platform group selections when opening edit (Antigravity)', async () => {
    const account = buildAccount()
    account.platform = 'antigravity'
    account.type = 'oauth'
    account.group_ids = [20]
    account.credentials = {}
    account.extra = {}
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = await openEditModal(account)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.group_ids).toEqual([20])
  })

  it('hydrates group_ids from groups when group_ids is missing', async () => {
    const account = buildAccount()
    delete account.group_ids
    account.groups = [{ id: 10, name: 'OpenAI Group', platform: 'openai' }]
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = await openEditModal(account)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.group_ids).toEqual([10])
  })

  it('strips antigravity bridge groups on OpenAI when Claude-GPT bridge is off', async () => {
    const account = buildAccount()
    // Stale mixed selection: openai + antigravity without bridge enabled
    account.group_ids = [10, 20]
    account.extra = { openai_claude_gpt_bridge_enabled: false }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = await openEditModal(account)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.group_ids).toEqual([10])
  })

  it('preserves OpenAI+antigravity groups when Claude-GPT bridge is on', async () => {
    const account = buildAccount()
    account.group_ids = [10, 20]
    account.extra = { openai_claude_gpt_bridge_enabled: true }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = await openEditModal(account)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.group_ids).toEqual([10, 20])
  })

  it('does not re-hydrate groups when auto-refresh replaces the same account object', async () => {
    const account = buildAccount()
    account.group_ids = [10]
    account.extra = {}
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = await openEditModal(account)

    // Simulate list auto-refresh: same id, new object with different group_ids
    const refreshed = {
      ...account,
      group_ids: [99],
      current_concurrency: 3
    }
    await wrapper.setProps({ account: refreshed })
    await wrapper.vm.$nextTick()

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    // Form must keep the values from the original open, not the refresh payload.
    expect(updateAccountMock.mock.calls[0]?.[1]?.group_ids).toEqual([10])
  })

  it('submits OpenAI quota auto-pause thresholds and per-window disable flags', async () => {
    const account = buildAccount()
    account.extra = {
      auto_pause_5h_threshold: 0.9,
      auto_pause_7d_threshold: 0.8,
      openai_claude_gpt_bridge_enabled: true
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('[data-testid="auto-pause-5h-threshold"]').setValue('95')
    await wrapper.get('[data-testid="auto-pause-7d-disabled"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).toMatchObject({
      auto_pause_5h_threshold: 0.95,
      auto_pause_7d_threshold: 0.8,
      auto_pause_7d_disabled: true,
      openai_claude_gpt_bridge_enabled: true
    })
  })

  it('applies the OpenAI Claude-GPT bridge mapping template and overwrites matching mappings', async () => {
    const account = buildAccount()
    account.credentials = {
      ...account.credentials,
      model_mapping: {
        'claude-opus-4-8': 'custom-gpt-opus'
      }
    }
    account.extra = {
      openai_claude_gpt_bridge_enabled: true
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('[data-testid="apply-openai-claude-gpt-bridge-template"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_mapping).toMatchObject({
      'claude-opus-4-8': 'gpt-5.5',
      'claude-sonnet-4-6': 'gpt-5.4'
    })
  })

  it('submits OpenAI API Key endpoint capabilities', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('[data-testid="openai-endpoint-capability-chat_completions"]').setValue(false)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.openai_capabilities).toEqual([
      'embeddings'
    ])
  })

  it('omits OpenAI images endpoint toggle when enabled by default', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_images_endpoint_enabled).toBeUndefined()
  })

  it('submits disabled OpenAI images endpoint scheduling', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('[data-testid="openai-images-endpoint-enabled"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_images_endpoint_enabled).toBe(false)
  })

  it('removes disabled OpenAI images endpoint scheduling when re-enabled', async () => {
    const account = buildAccount()
    account.extra = {
      openai_images_endpoint_enabled: false
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('[data-testid="openai-images-endpoint-enabled"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_images_endpoint_enabled).toBeUndefined()
  })

  it('keeps at least one OpenAI API Key endpoint capability selected', async () => {
    const account = buildAccount()
    account.credentials = {
      ...account.credentials,
      openai_capabilities: ['embeddings']
    }
    const wrapper = mountModal(account)

    const embeddingCheckbox = wrapper.get<HTMLInputElement>(
      '[data-testid="openai-endpoint-capability-embeddings"]'
    )
    await embeddingCheckbox.setValue(false)

    expect(embeddingCheckbox.element.checked).toBe(true)
  })

  it('migrates legacy Codex image-generation bridge extra to the new field', async () => {
    const account = buildAccount()
    account.extra = {
      codex_image_generation_bridge_enabled: true
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.codex_image_generation_bridge).toBe(true)
    expect(
      updateAccountMock.mock.calls[0]?.[1]?.extra?.codex_image_generation_bridge_enabled
    ).toBeUndefined()
  })

  it('uses two top cards and a three-zone edit layout', async () => {
    const wrapper = mountModal()
    expect(wrapper.find('[data-testid="edit-account-tab-edit"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="edit-account-tab-schedule"]').exists()).toBe(true)
    const layout = wrapper.get('[data-testid="edit-account-layout"]')
    expect(layout.classes().join(' ')).toContain('xl:grid-cols-3')
    expect(layout.classes().join(' ')).toContain('lg:grid-cols-2')
    expect(wrapper.find('[data-testid="edit-account-zone-config"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="edit-account-zone-groups"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="edit-account-zone-other"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="edit-account-zone-user-schedule"]').exists()).toBe(false)
    const html = wrapper.html()
    expect(html.indexOf('edit-account-zone-groups')).toBeLessThan(
      html.indexOf('edit-account-zone-other')
    )
    await wrapper.get('[data-testid="edit-account-tab-schedule"]').trigger('click')
    expect(wrapper.find('[data-testid="edit-account-schedule-panel"]').exists()).toBe(true)
  })

  it('does not rewrite leftover user-schedule lists on save', async () => {
    const account = buildAccount()
    account.user_schedule_mode = 'allow'
    account.schedule_users = [
      { id: 16, email: 'a@x.com', deleted: false, allow: true, max_concurrency: 5 },
      { id: 42, email: 'b@x.com', deleted: false, deny: true }
    ]
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.allow_user_ids).toBeUndefined()
    expect(updateAccountMock.mock.calls[0]?.[1]?.deny_user_ids).toBeUndefined()
    expect(updateAccountMock.mock.calls[0]?.[1]?.user_concurrencies).toBeUndefined()
    expect(updateAccountMock.mock.calls[0]?.[1]?.user_quality_gates).toBeUndefined()
    expect(updateAccountMock.mock.calls[0]?.[1]?.user_schedule_mode).toBeUndefined()
  })

  it('writes oauth fleet extra as inherit / force-on / force-off', async () => {
    const account = buildAccount()
    account.type = 'oauth'
    account.extra = { preserved_setting: 'keep-me', oauth_fleet_soft_429: true }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    const override = wrapper.get('[data-testid="oauth-fleet-soft-429-override"]')
    expect(override.exists()).toBe(true)
    const mode = wrapper.get<HTMLSelectElement>('[data-testid="oauth-fleet-soft-429-mode"]')
    expect(mode.element.value).toBe('enabled')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).toMatchObject({
      preserved_setting: 'keep-me',
      oauth_fleet_soft_429: true
    })

    await mode.setValue('disabled')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    expect(updateAccountMock.mock.calls[1]?.[1]?.extra?.oauth_fleet_soft_429).toBe(false)
    expect(updateAccountMock.mock.calls[1]?.[1]?.extra?.preserved_setting).toBe('keep-me')

    await mode.setValue('inherit')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    expect(updateAccountMock.mock.calls[2]?.[1]?.extra?.oauth_fleet_soft_429).toBeUndefined()
    expect(updateAccountMock.mock.calls[2]?.[1]?.extra?.preserved_setting).toBe('keep-me')
  })

  it('hides oauth fleet override on API Key accounts', () => {
    const wrapper = mountModal(buildAccount())
    expect(wrapper.find('[data-testid="oauth-fleet-soft-429-override"]').exists()).toBe(false)
  })

  it('saves New API wallet credentials and can keep or clear the token', async () => {
    const account = buildAccount()
    account.credentials = {
      ...account.credentials,
      newapi_access_token: 'saved-token',
      newapi_user_id: '1'
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    expect(wrapper.find('[data-testid="newapi-wallet-fields"]').exists()).toBe(true)
    expect(wrapper.get<HTMLInputElement>('[data-testid="newapi-wallet-user-id"]').element.value).toBe(
      '1'
    )

    await wrapper.get('[data-testid="newapi-wallet-user-id"]').setValue('952')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    await flushPromises()
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toMatchObject({
      api_key: 'sk-test',
      newapi_user_id: '952',
      newapi_access_token: 'saved-token'
    })

    await wrapper.get('[data-testid="newapi-wallet-access-token"]').setValue('new-wallet-token')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    await flushPromises()
    expect(updateAccountMock.mock.calls[1]?.[1]?.credentials?.newapi_access_token).toBe(
      'new-wallet-token'
    )

    await wrapper.get('[data-testid="newapi-wallet-clear"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    await flushPromises()
    const cleared = updateAccountMock.mock.calls[2]?.[1]?.credentials as Record<string, unknown>
    expect(cleared.newapi_access_token).toBeUndefined()
    expect(cleared.newapi_user_id).toBeUndefined()
  })
})
