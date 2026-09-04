import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import OAuthAuthorizationFlow from '../OAuthAuthorizationFlow.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copied: { value: false }, copyToClipboard: vi.fn() })
}))

describe('OAuthAuthorizationFlow Codex session paste', () => {
  it('emits trimmed session JSON without exposing the ST-only radio', async () => {
    const wrapper = mount(OAuthAuthorizationFlow, {
      props: {
        addMethod: 'oauth',
        platform: 'openai',
        showCodexSessionImportOption: true,
        showSessionTokenOption: false
      },
      global: {
        stubs: { Icon: true }
      }
    })

    expect(wrapper.find('input[value="session_token"]').exists()).toBe(false)
    await wrapper.get('[data-testid="oauth-method-codex-session"]').setValue(true)
    await wrapper.get('[data-testid="codex-session-input"]').setValue('  {"accessToken":"at"}  ')
    await wrapper.get('[data-testid="codex-session-submit"]').trigger('click')

    expect(wrapper.emitted('import-codex-session')).toEqual([['{"accessToken":"at"}']])
  })
})
