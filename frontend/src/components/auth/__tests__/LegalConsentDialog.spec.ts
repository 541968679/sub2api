import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LegalConsentDialog from '@/components/auth/LegalConsentDialog.vue'

const t = (key: string, params?: Record<string, unknown>) => {
  if (key === 'legalConsent.confirmationPhrase') {
    return '我已同意上述条款，如有任何风险或问题自行承担'
  }
  if (key === 'legalConsent.countdown') {
    return `请继续阅读 ${params?.seconds} 秒`
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
    await wrapper.get('[data-testid="legal-consent-region-check"]').setValue(true)
    await wrapper.get('[data-testid="legal-consent-confirmation"]').setValue(
      '我已同意上述条款，如有任何风险或问题自行承担'
    )
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
      typedConfirmation: '我已同意上述条款，如有任何风险或问题自行承担',
      dwellSeconds: 2,
      scrolledToBottom: true,
      regionAttestation: true,
    })
  })
})
