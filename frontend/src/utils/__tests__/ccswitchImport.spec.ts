import { describe, expect, it } from 'vitest'
import {
  GROK_CC_SWITCH_CODEX_MODEL,
  buildCcSwitchImportDeeplink,
  resolveCcSwitchImportConfig
} from '@/utils/ccswitchImport'
import type { GroupPlatform } from '@/types'

function paramsFromDeeplink(deeplink: string): URLSearchParams {
  const query = deeplink.split('?')[1] || ''
  return new URLSearchParams(query)
}

const baseInput = {
  baseUrl: 'https://api.example.com',
  providerName: 'Sub2API',
  apiKey: 'sk-test',
  usageScript: 'return true'
}

describe('ccswitchImport utils', () => {
  it('defaults Grok CC Switch imports to grok-4.5 on Codex', () => {
    expect(GROK_CC_SWITCH_CODEX_MODEL).toBe('grok-4.5')
  })

  it('resolves Grok platform to Codex app with Grok model (not Anthropic codex model)', () => {
    const config = resolveCcSwitchImportConfig('grok', 'codex', baseInput.baseUrl, {
      openaiCodexModel: 'gpt-5-codex',
      anthropicCodexModel: 'claude-sonnet-4-5'
    })
    expect(config).toEqual({
      app: 'codex',
      endpoint: baseInput.baseUrl,
      model: 'grok-4.5'
    })
  })

  it('adds model=grok-4.5 for Grok-group CCS deeplinks', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'grok',
        clientType: 'codex',
        modelOptions: {
          openaiCodexModel: 'gpt-5-codex',
          anthropicCodexModel: 'claude-sonnet-4-5'
        }
      })
    )

    expect(params.get('app')).toBe('codex')
    expect(params.get('endpoint')).toBe(baseInput.baseUrl)
    expect(params.get('model')).toBe(GROK_CC_SWITCH_CODEX_MODEL)
    expect(params.get('model')).not.toBe('claude-sonnet-4-5')
    expect(atob(params.get('usageScript') || '')).toBe(baseInput.usageScript)
  })

  it('uses admin OpenAI Codex model for OpenAI imports when set', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'openai',
        clientType: 'codex',
        modelOptions: { openaiCodexModel: 'gpt-5-codex' }
      })
    )
    expect(params.get('app')).toBe('codex')
    expect(params.get('model')).toBe('gpt-5-codex')
  })

  it('omits model for OpenAI when admin setting is empty', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'openai',
        clientType: 'codex',
        modelOptions: { openaiCodexModel: '  ' }
      })
    )
    expect(params.get('app')).toBe('codex')
    expect(params.has('model')).toBe(false)
  })

  it('uses anthropic Codex model only for Anthropic→Codex, not Grok', () => {
    const anthropic = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'anthropic',
        clientType: 'codex',
        modelOptions: { anthropicCodexModel: 'claude-sonnet-4-5' }
      })
    )
    expect(anthropic.get('app')).toBe('codex')
    expect(anthropic.get('model')).toBe('claude-sonnet-4-5')

    const grok = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'grok',
        clientType: 'codex',
        modelOptions: { anthropicCodexModel: 'claude-sonnet-4-5' }
      })
    )
    expect(grok.get('model')).toBe('grok-4.5')
  })

  it.each([
    { platform: 'anthropic' as GroupPlatform, clientType: 'claude' as const, app: 'claude' },
    { platform: 'gemini' as GroupPlatform, clientType: 'gemini' as const, app: 'gemini' }
  ])('does not add a model parameter for $platform $clientType imports', ({ platform, clientType, app }) => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform,
        clientType
      })
    )
    expect(params.get('app')).toBe(app)
    expect(params.get('endpoint')).toBe(baseInput.baseUrl)
    expect(params.has('model')).toBe(false)
  })

  it('keeps Antigravity imports on the selected client endpoint without a model by default', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'antigravity',
        clientType: 'gemini'
      })
    )
    expect(params.get('app')).toBe('gemini')
    expect(params.get('endpoint')).toBe(`${baseInput.baseUrl}/antigravity`)
    expect(params.has('model')).toBe(false)
  })
})
