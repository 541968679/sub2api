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

describe('OAuthAuthorizationFlow Codex PAT mode', () => {
  it('emits a trimmed PAT without exposing it through another auth mode', async () => {
    const wrapper = mount(OAuthAuthorizationFlow, {
      props: {
        addMethod: 'oauth',
        platform: 'openai',
        showCodexPatOption: true
      },
      global: {
        stubs: { Icon: true }
      }
    })

    await wrapper.get('input[value="codex_pat"]').setValue(true)
    await wrapper.get('[data-testid="codex-pat-input"]').setValue('  at-frontend  ')
    await wrapper.get('[data-testid="codex-pat-submit"]').trigger('click')

    expect(wrapper.emitted('import-codex-pat')).toEqual([['at-frontend']])
  })
})
