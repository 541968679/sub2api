import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ModelPlazaView from '@/views/ModelPlazaView.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

vi.mock('@/api/modelPlaza', () => ({
  getModelPlaza: vi.fn().mockResolvedValue({
    description: 'Showcase',
    groups: [
      {
        id: 1,
        name: 'public-standard',
        description: '',
        platform: 'anthropic',
        subscription_type: 'standard',
        rate_multiplier: 1,
        is_exclusive: false,
        models: [
          {
            name: 'claude-sonnet',
            platform: 'anthropic',
            pricing: { input_price: 3e-6, output_price: 1.5e-5, cache_read_price: 3e-7 },
            official_pricing: { input_price: 3e-6 }
          }
        ]
      }
    ]
  })
}))

describe('ModelPlazaView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders display prices for public groups', async () => {
    const wrapper = mount(ModelPlazaView)
    await flushPromises()
    expect(wrapper.text()).toContain('public-standard')
    expect(wrapper.text()).toContain('claude-sonnet')
    expect(wrapper.text()).toContain('$3.00 / MTok')
  })
})
