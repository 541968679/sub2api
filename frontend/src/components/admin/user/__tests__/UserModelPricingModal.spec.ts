import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const {
  getUserModelPricing,
  batchUpsertUserModelPricing,
  deleteUserModelPricing,
  listModelPricing,
  getPlatformModelCatalog,
  updateUser,
} = vi.hoisted(() => ({
  getUserModelPricing: vi.fn(),
  batchUpsertUserModelPricing: vi.fn(),
  deleteUserModelPricing: vi.fn(),
  listModelPricing: vi.fn(),
  getPlatformModelCatalog: vi.fn(),
  updateUser: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    modelPricing: {
      list: listModelPricing,
    },
  },
}))

vi.mock('@/api/admin/userModelPricing', () => ({
  getUserModelPricing,
  batchUpsertUserModelPricing,
  deleteUserModelPricing,
}))

vi.mock('@/api/admin/users', () => ({
  update: updateUser,
}))

vi.mock('@/api/admin/modelCatalog', () => ({
  getPlatformModelCatalog,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: {
    name: 'BaseDialog',
    props: ['show', 'title', 'width'],
    template: '<div v-if="show"><slot /><slot name="footer" /></div>',
  },
}))

vi.mock('@/components/common/Select.vue', () => ({
  default: {
    name: 'Select',
    props: ['modelValue', 'options', 'placeholder', 'searchable'],
    emits: ['update:modelValue'],
    template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))

import UserModelPricingModal from '../UserModelPricingModal.vue'

function makeUser() {
  return { id: 7, email: 'u@example.com' } as any
}

async function mountAndOpen() {
  const wrapper = mount(UserModelPricingModal, {
    props: { show: false, user: makeUser() },
  })
  await wrapper.setProps({ show: true })
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  updateUser.mockResolvedValue({})
  batchUpsertUserModelPricing.mockResolvedValue([])
  deleteUserModelPricing.mockResolvedValue({})
  listModelPricing.mockResolvedValue({
    items: [
      {
        model: 'claude-opus-4-8',
        provider: 'anthropic',
        litellm_prices: {
          input_price: 5e-6,
          output_price: 2.5e-5,
          cache_write_price: 6.25e-6,
          cache_write_1h_price: 1e-5,
          cache_read_price: 5e-7,
        },
      },
      {
        model: 'claude-sonnet-4-6',
        provider: 'anthropic',
        litellm_prices: {
          input_price: 3e-6,
          output_price: 1.5e-5,
          cache_write_price: null,
          cache_write_1h_price: null,
          cache_read_price: null,
        },
      },
    ],
    pagination: { total: 2, page: 1, page_size: 1000, pages: 1 },
    stats: { total_models: 2, global_override_count: 0, channel_override_count: 0 },
  })
  getPlatformModelCatalog.mockImplementation(async (platform: string) => {
    if (platform === 'anthropic') {
      return { display_models: ['claude-opus-4-8', 'claude-sonnet-4-6'], whitelist_models: [] }
    }
    return { display_models: [], whitelist_models: [] }
  })
  getUserModelPricing.mockResolvedValue([
    {
      id: 1,
      user_id: 7,
      model: 'claude-opus-4-8',
      input_price: null,
      output_price: null,
      cache_write_price: null,
      cache_write_1h_price: null,
      cache_read_price: null,
      display_input_price: null,
      display_output_price: null,
      display_cache_read_price: null,
      display_cache_creation_price: null,
      display_cache_creation_1h_price: null,
      enabled: true,
      notes: 'keep',
    },
    {
      id: 2,
      user_id: 7,
      model: 'mystery-model',
      input_price: null,
      output_price: null,
      cache_write_price: null,
      cache_write_1h_price: null,
      cache_read_price: null,
      display_input_price: null,
      display_output_price: null,
      display_cache_read_price: null,
      display_cache_creation_price: null,
      display_cache_creation_1h_price: null,
      enabled: true,
      notes: '',
    },
  ])
})

describe('UserModelPricingModal', () => {
  it('filters existing overrides by platform tab and keeps unclassified rows under other', async () => {
    const wrapper = await mountAndOpen()
    const anthropicModels = wrapper.findAll('[data-test="override-row"]').map((row) => row.attributes('data-model'))
    expect(anthropicModels).toEqual(['claude-opus-4-8'])

    await wrapper.get('[data-test="platform-tab-other"]').trigger('click')
    await flushPromises()
    const otherModels = wrapper.findAll('[data-test="override-row"]').map((row) => row.attributes('data-model'))
    expect(otherModels).toEqual(['mystery-model'])
  })

  it('bulk-applies LiteLLM suggested billing and display to the platform curated list', async () => {
    const wrapper = await mountAndOpen()
    await wrapper.get('[data-test="bulk-apply-suggested"]').trigger('click')
    await flushPromises()

    const models = wrapper.findAll('[data-test="override-row"]').map((row) => row.attributes('data-model'))
    expect(models).toEqual(['claude-opus-4-8', 'claude-sonnet-4-6'])

    await wrapper.get('[data-test="save-model-pricing"]').trigger('click')
    await flushPromises()

    expect(batchUpsertUserModelPricing).toHaveBeenCalledTimes(1)
    const payload = batchUpsertUserModelPricing.mock.calls[0][1] as Array<{
      model: string
      input_price: number | null
      display_input_price: number | null
      notes: string
    }>
    const byModel = Object.fromEntries(payload.map((row) => [row.model, row]))
    expect(byModel['claude-opus-4-8'].input_price).toBeCloseTo(5e-6)
    expect(byModel['claude-opus-4-8'].display_input_price).toBeCloseTo(5e-6)
    expect(byModel['claude-opus-4-8'].notes).toBe('keep')
    expect(byModel['claude-sonnet-4-6'].input_price).toBeCloseTo(3e-6)
    expect(byModel['claude-sonnet-4-6'].display_input_price).toBeCloseTo(3e-6)
    expect(byModel['mystery-model']).toBeTruthy()
    expect(deleteUserModelPricing).not.toHaveBeenCalled()
  })
})
