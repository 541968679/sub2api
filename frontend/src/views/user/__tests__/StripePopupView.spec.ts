import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'

const routeState = vi.hoisted(() => ({
  query: {
    order_id: '42',
    method: 'wechat_pay',
    amount: '10.00',
  } as Record<string, unknown>,
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return { ...actual, useRoute: () => routeState }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

import StripePopupView from '../StripePopupView.vue'

describe('StripePopupView', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('cancels the fallback timeout after receiving popup initialization', () => {
    const clearTimeoutSpy = vi.spyOn(globalThis, 'clearTimeout')
    let messageHandler: ((event: MessageEvent) => void) | undefined
    vi.spyOn(window, 'addEventListener').mockImplementation(((type: string, listener: EventListenerOrEventListenerObject) => {
      if (type === 'message' && typeof listener === 'function') {
        messageHandler = listener as (event: MessageEvent) => void
      }
    }) as typeof window.addEventListener)
    const wrapper = shallowMount(StripePopupView)

    expect(messageHandler).toBeTypeOf('function')
    const initEvent = new MessageEvent('message', {
      data: {
        type: 'STRIPE_POPUP_INIT',
        clientSecret: 'client-secret',
        publishableKey: 'pk_test',
      },
    })
    Object.defineProperty(initEvent, 'origin', { value: window.location.origin })
    messageHandler?.(initEvent)

    expect(clearTimeoutSpy).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
})
