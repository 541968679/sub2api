import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import GroupSelector from '../GroupSelector.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params?.count === undefined ? key : `${key}:${params.count}`
    })
  }
})

const groups = [
  {
    id: 1,
    name: 'Anthropic',
    platform: 'anthropic',
    subscription_type: 'standard',
    rate_multiplier: 1,
    account_count: 1
  },
  {
    id: 2,
    name: 'OpenAI',
    platform: 'openai',
    subscription_type: 'standard',
    rate_multiplier: 1,
    account_count: 2
  },
  {
    id: 3,
    name: 'OpenAI Plus',
    platform: 'openai',
    subscription_type: 'subscription',
    rate_multiplier: 1,
    account_count: 3
  }
] as any

function mountSelector(modelValue: number[] = [], extraProps: Record<string, unknown> = {}) {
  return mount(GroupSelector, {
    props: {
      modelValue,
      groups,
      ...extraProps
    },
    global: {
      stubs: {
        GroupBadge: {
          props: ['name'],
          template: '<span>{{ name }}</span>'
        },
        Icon: true
      }
    }
  })
}

describe('GroupSelector', () => {
  it('hides the select-all control by default for non-account reuse', () => {
    const wrapper = mountSelector([1], { platform: 'openai' })

    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('selects all currently filterable groups without dropping hidden selections', async () => {
    const wrapper = mountSelector([1], { platform: 'openai', showToggleAll: true })

    await wrapper.get('button').trigger('click')

    expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toEqual([1, 2, 3])
  })

  it('deselects only currently filterable groups when all are selected', async () => {
    const wrapper = mountSelector([1, 2, 3], { platform: 'openai', showToggleAll: true })

    await wrapper.get('button').trigger('click')

    expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toEqual([1])
  })
})
