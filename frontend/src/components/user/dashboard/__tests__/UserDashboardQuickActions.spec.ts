import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'

import UserDashboardQuickActions from '../UserDashboardQuickActions.vue'

const push = vi.hoisted(() => vi.fn())
const appState = vi.hoisted(() => ({
  payment_enabled: true,
  tutorial_url: ''
}))
const authState = vi.hoisted(() => ({
  isSimpleMode: false
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    cachedPublicSettings: appState,
    tutorialUrl: ''
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState
}))

vi.mock('@/components/common/SupportContactBar.vue', () => ({
  default: defineComponent({ name: 'SupportContactBar', setup: () => () => h('div') })
}))

vi.mock('@/components/icons/Icon.vue', () => ({
  default: defineComponent({ name: 'Icon', setup: () => () => h('span') })
}))

describe('UserDashboardQuickActions', () => {
  beforeEach(() => {
    push.mockReset()
    appState.payment_enabled = true
    appState.tutorial_url = ''
    authState.isSimpleMode = false
  })

  it('shows purchase, billing rules, and API access as primary cards', () => {
    const wrapper = mount(UserDashboardQuickActions)
    expect(wrapper.find('[data-test="dashboard-primary-purchase"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="dashboard-primary-pricing"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="dashboard-primary-keys"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="dashboard-secondary-agent"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('dashboard.agentEntryEyebrow')
  })

  it('keeps agent entry secondary and navigates to distribution', async () => {
    const wrapper = mount(UserDashboardQuickActions)
    await wrapper.get('[data-test="dashboard-secondary-agent"]').trigger('click')
    expect(push).toHaveBeenCalledWith('/distribution')
  })
})
