import { describe, expect, it, vi } from 'vitest'
import { DOMWrapper, mount } from '@vue/test-utils'
import Select from '../Select.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const options = [
  { kind: 'group', value: 'platform:openai', label: 'OpenAI', disabled: true, platform: 'openai' },
  { value: 1, label: '优质', description: 'fast', platform: 'openai' },
  { value: 2, label: '经济', platform: 'openai' },
  { kind: 'group', value: 'platform:anthropic', label: 'Anthropic', disabled: true, platform: 'anthropic' },
  { value: 3, label: 'Claude', platform: 'anthropic' }
]

function mountSelect() {
  return mount(Select, {
    props: {
      modelValue: null,
      searchable: true,
      options
    },
    global: {
      stubs: {
        Icon: true
      }
    },
    attachTo: document.body
  })
}

describe('Select platform group headers', () => {
  it('renders a heading per platform and does not select the heading', async () => {
    const wrapper = mountSelect()
    await wrapper.get('button').trigger('click')

    const headings = document.body.querySelectorAll('.select-option-group')
    expect(Array.from(headings).map((node) => node.textContent?.trim())).toEqual(['OpenAI', 'Anthropic'])

    await new DOMWrapper(headings[0] as HTMLElement).trigger('click')
    expect(wrapper.emitted('update:modelValue')).toBeFalsy()

    wrapper.unmount()
  })

  it('keeps the platform heading when a group name matches and hides other platforms', async () => {
    const wrapper = mountSelect()
    await wrapper.get('button').trigger('click')
    const input = new DOMWrapper(document.body.querySelector('input') as HTMLInputElement)
    await input.setValue('优质')

    const text = document.body.textContent ?? ''
    expect(text).toContain('OpenAI')
    expect(text).toContain('优质')
    expect(text).not.toContain('经济')
    expect(text).not.toContain('Claude')
    expect(text).not.toContain('Anthropic')

    wrapper.unmount()
  })
})
