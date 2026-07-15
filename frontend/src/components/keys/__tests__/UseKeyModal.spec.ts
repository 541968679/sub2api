import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn().mockResolvedValue(true)
  })
}))

import UseKeyModal from '../UseKeyModal.vue'

describe('UseKeyModal', () => {
  it('renders GPT-5.4 mini entry in OpenCode config', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const opencodeTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.opencode')
    )

    expect(opencodeTab).toBeDefined()
    await opencodeTab!.trigger('click')
    await nextTick()

    const codeBlock = wrapper.find('pre code')
    expect(codeBlock.exists()).toBe(true)
    expect(codeBlock.text()).toContain('"name": "GPT-5.6 Sol"')
    expect(codeBlock.text()).toContain('"name": "GPT-5.6 Terra"')
    expect(codeBlock.text()).toContain('"name": "GPT-5.6 Luna"')
    expect(codeBlock.text()).toContain('"name": "GPT-5.4 Mini"')
    expect(codeBlock.text()).not.toContain('"name": "GPT-5.4 Nano"')

    const parsed = JSON.parse(codeBlock.text())
    const sol = parsed.provider.openai.models['gpt-5.6-sol']
    expect(sol.limit).toEqual({ context: 1050000, output: 128000 })
    expect(sol.options.store).toBe(false)
    expect(sol.variants).toHaveProperty('xhigh')
  })

  it('defaults Grok keys to Codex config with catalog metadata', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-grok-test',
        baseUrl: 'https://example.com/v1',
        platform: 'grok'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    await nextTick()

    const codeBlocks = wrapper.findAll('pre code').map((code) => code.text())
    const configToml = codeBlocks.find((content) => content.includes('model = "grok-4.5"'))
    const catalog = codeBlocks.find((content) => content.includes('"slug": "grok-4.5"'))

    expect(configToml).toBeDefined()
    expect(configToml).toContain('model_context_window = 1000000')
    expect(configToml).toContain('model_catalog_json = "model-catalog-grok.json"')
    expect(configToml).toContain('wire_api = "responses"')
    expect(configToml).not.toContain('claude-sonnet')

    expect(catalog).toBeDefined()
    const parsed = JSON.parse(catalog!)
    expect(parsed.models[0].slug).toBe('grok-4.5')
    expect(parsed.models[0].base_instructions).toBeTruthy()
  })

  it('renders Claude Fable 5 OpenCode config with adaptive thinking', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'antigravity'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const opencodeTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.opencode')
    )

    expect(opencodeTab).toBeDefined()
    await opencodeTab!.trigger('click')
    await nextTick()

    const claudeConfig = wrapper.findAll('pre code')
      .map((code) => code.text())
      .find((content) => content.includes('"antigravity-claude"'))

    expect(claudeConfig).toBeDefined()
    const parsed = JSON.parse(claudeConfig!)
    const fable = parsed.provider['antigravity-claude'].models['claude-fable-5']

    expect(fable.name).toBe('Claude Fable 5')
    expect(fable.limit).toEqual({ context: 1048576, output: 128000 })
    expect(fable.options.thinking).toEqual({ type: 'adaptive' })
    expect(fable.options.thinking).not.toHaveProperty('budgetTokens')
  })
})
