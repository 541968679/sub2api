import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SupportContactBar from '../SupportContactBar.vue'
import { useAppStore } from '@/stores/app'

const clipboardMock = vi.hoisted(() => ({
  copied: { value: false },
  copyToClipboard: vi.fn().mockResolvedValue(true)
}))

const messages: Record<string, string> = {
  'supportContact.dashboardLabel': 'Support',
  'supportContact.paymentLabel': 'Payment or credit issue? Contact support',
  'supportContact.copy': 'Copy contact',
  'supportContact.copied': 'Copied',
  'supportContact.copySuccess': 'Support contact copied',
  'common.copy': 'Copy'
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key
    })
  }
})

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copied: clipboardMock.copied,
    copyToClipboard: clipboardMock.copyToClipboard
  })
}))

let pinia: ReturnType<typeof createPinia>

function mountBar(props: { context?: 'dashboard' | 'payment' } = {}) {
  return mount(SupportContactBar, {
    props,
    global: {
      plugins: [pinia],
      stubs: {
        Icon: true
      }
    }
  })
}

describe('SupportContactBar', () => {
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    clipboardMock.copied.value = false
    clipboardMock.copyToClipboard.mockClear()
  })

  it('stays hidden when support contact is not configured', () => {
    const appStore = useAppStore()
    appStore.contactInfo = ''
    appStore.publicSettingsLoaded = true

    const wrapper = mountBar()

    expect(wrapper.find('[role="note"]').exists()).toBe(false)
  })

  it('fetches public settings when the cache has not loaded yet', () => {
    const appStore = useAppStore()
    appStore.publicSettingsLoaded = false
    const fetchPublicSettings = vi.spyOn(appStore, 'fetchPublicSettings').mockResolvedValue(null)

    mountBar()

    expect(fetchPublicSettings).toHaveBeenCalledTimes(1)
  })

  it('renders the configured contact and copies the trimmed value', async () => {
    const appStore = useAppStore()
    appStore.contactInfo = '  QQ: 123456789  '
    appStore.publicSettingsLoaded = true

    const wrapper = mountBar({ context: 'payment' })

    expect(wrapper.get('[role="note"]').text()).toContain('Payment or credit issue? Contact support')
    expect(wrapper.get('[role="note"]').text()).toContain('QQ: 123456789')

    await wrapper.get('button').trigger('click')

    expect(clipboardMock.copyToClipboard).toHaveBeenCalledWith(
      'QQ: 123456789',
      'Support contact copied'
    )
  })
})
