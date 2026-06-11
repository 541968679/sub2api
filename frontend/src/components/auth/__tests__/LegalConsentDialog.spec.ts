import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LegalConsentDialog from '@/components/auth/LegalConsentDialog.vue'
import { CURRENT_LEGAL_CONFIRMATION_PHRASE } from '@/utils/legalConsent'

const t = (key: string, params?: Record<string, unknown>) => {
  if (key === 'legalConsent.countdown') {
    return `请继续阅读 ${params?.seconds} 秒`
  }
  if (key === 'legalConsent.version') {
    return `条款版本：${params?.version}`
  }
  if (key === 'legalConsent.confirmationHint') {
    return `请逐字输入：${params?.phrase}`
  }
  return key
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t,
  }),
}))

describe('LegalConsentDialog', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('requires countdown, bottom scroll, checks, and exact confirmation phrase', async () => {
    const wrapper = mount(LegalConsentDialog, {
      props: {
        show: true,
        mode: 'register',
        minReadSeconds: 2,
      },
      global: {
        stubs: {
          Teleport: true,
          Transition: false,
          Icon: true,
        },
      },
    })

    const confirmButton = wrapper.get('[data-testid="legal-consent-confirm"]')
    expect(confirmButton.attributes('disabled')).toBeDefined()

    await wrapper.get('[data-testid="legal-consent-terms-check"]').setValue(true)
    await wrapper.get('[data-testid="legal-consent-authorized-use-check"]').setValue(true)
    await wrapper.get('[data-testid="legal-consent-confirmation"]').setValue(CURRENT_LEGAL_CONFIRMATION_PHRASE)
    vi.advanceTimersByTime(2500)
    await wrapper.vm.$nextTick()
    expect(confirmButton.attributes('disabled')).toBeDefined()

    const scrollEl = wrapper.get('[data-testid="legal-consent-scroll"]').element
    Object.defineProperty(scrollEl, 'scrollTop', { value: 800, configurable: true })
    Object.defineProperty(scrollEl, 'clientHeight', { value: 200, configurable: true })
    Object.defineProperty(scrollEl, 'scrollHeight', { value: 1000, configurable: true })

    await wrapper.get('[data-testid="legal-consent-scroll"]').trigger('scroll')
    await wrapper.vm.$nextTick()

    expect(wrapper.get('[data-testid="legal-consent-confirm"]').attributes('disabled')).toBeUndefined()

    await wrapper.get('[data-testid="legal-consent-confirm"]').trigger('click')
    expect(wrapper.emitted('accept')?.[0]?.[0]).toMatchObject({
      typedConfirmation: CURRENT_LEGAL_CONFIRMATION_PHRASE,
      dwellSeconds: 2,
      scrolledToBottom: true,
      authorizedUseAttestation: true,
    })
  })

  it('uses configured terms content and confirmation phrase', async () => {
    const wrapper = mount(LegalConsentDialog, {
      props: {
        show: true,
        mode: 'login',
        settings: {
          enabled: true,
          version: 'legal-v9',
          content: 'Custom internal-only terms',
          confirmation_phrase: 'I agree to legal-v9',
          min_read_seconds: 0,
        },
      },
      global: {
        stubs: {
          Teleport: true,
          Transition: false,
          Icon: true,
        },
      },
    })

    expect(wrapper.text()).toContain('Custom internal-only terms')
    expect(wrapper.text()).toContain('legal-v9')

    await wrapper.get('[data-testid="legal-consent-terms-check"]').setValue(true)
    await wrapper.get('[data-testid="legal-consent-authorized-use-check"]').setValue(true)
    await wrapper.get('[data-testid="legal-consent-confirmation"]').setValue('I agree to legal-v9')

    const scrollEl = wrapper.get('[data-testid="legal-consent-scroll"]').element
    Object.defineProperty(scrollEl, 'scrollTop', { value: 800, configurable: true })
    Object.defineProperty(scrollEl, 'clientHeight', { value: 200, configurable: true })
    Object.defineProperty(scrollEl, 'scrollHeight', { value: 1000, configurable: true })
    await wrapper.get('[data-testid="legal-consent-scroll"]').trigger('scroll')

    expect(wrapper.get('[data-testid="legal-consent-confirm"]').attributes('disabled')).toBeUndefined()
  })
})
