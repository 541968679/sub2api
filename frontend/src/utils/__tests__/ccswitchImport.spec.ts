import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  CCS_IMPORT_PRESET_GLM,
  CCS_IMPORT_PRESET_GPT,
  GROK_CC_SWITCH_CODEX_MODEL,
  buildCcSwitchImportDeeplink,
  buildCcSwitchProviderName,
  launchCcSwitchImportDeeplink,
  mergeCcsImportModelOptions,
  parseGatewayModelsList,
  resolveCcSwitchImportConfig,
  compactCcsImportModelIDs,
  filterCcsImportModelIDs,
  resolveCcsImportPickerIDs,
  shouldShowCcsCodexModelPicker
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

  it('builds CCS provider names as site-key so keys stay distinguishable', () => {
    expect(buildCcSwitchProviderName('ZeroCode', 'Claude生产')).toBe('ZeroCode-Claude生产')
    expect(buildCcSwitchProviderName('  ZeroCode  ', '  Codex 日常  ')).toBe('ZeroCode-Codex 日常')
    expect(buildCcSwitchProviderName('', 'my-key')).toBe('sub2api-my-key')
    expect(buildCcSwitchProviderName('ZeroCode', '   ')).toBe('ZeroCode')
    expect(buildCcSwitchProviderName(undefined, undefined)).toBe('sub2api')
  })

  it('writes the provider name into the CCS deeplink name param', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'openai',
        clientType: 'codex',
        providerName: buildCcSwitchProviderName('ZeroCode', 'Claude生产')
      })
    )
    expect(params.get('name')).toBe('ZeroCode-Claude生产')
  })

  it('exposes CCS import presets gpt-6-astra and glm-5.3', () => {
    expect(CCS_IMPORT_PRESET_GPT).toBe('gpt-6-astra')
    expect(CCS_IMPORT_PRESET_GLM).toBe('glm-5.3')
  })

  it('only shows the Codex model picker when the group switch is on', () => {
    expect(shouldShowCcsCodexModelPicker(true, 'codex')).toBe(true)
    expect(shouldShowCcsCodexModelPicker(false, 'codex')).toBe(false)
    expect(shouldShowCcsCodexModelPicker(true, 'claude')).toBe(false)
    expect(shouldShowCcsCodexModelPicker(undefined, 'codex')).toBe(false)
  })

  it('pins a missing default model at the top of /v1/models options', () => {
    expect(mergeCcsImportModelOptions('glm-5.3', ['gpt-6-astra', 'gpt-5.4'])).toEqual([
      'glm-5.3',
      'gpt-6-astra',
      'gpt-5.4'
    ])
    expect(mergeCcsImportModelOptions('gpt-6-astra', ['gpt-6-astra', 'gpt-5.4'])).toEqual([
      'gpt-6-astra',
      'gpt-5.4'
    ])
  })

  it('ignores the stock GPT catalog and uses the default-model family', () => {
    expect(
      resolveCcsImportPickerIDs({
        defaultModel: 'glm-5.3',
        fetchedIDs: ['gpt-6-astra', 'gpt-5.4', 'grok-4.5']
      })
    ).toEqual(['glm-5.3', 'glm-5.3-flash'])
  })

  it('keeps a mixed custom /v1/models list without dropping older IDs', () => {
    expect(
      resolveCcsImportPickerIDs({
        defaultModel: 'glm-5.3',
        fetchedIDs: ['glm-5.3', 'glm-4.7', 'kimi-k2.5', 'chatglm_turbo']
      })
    ).toEqual(['glm-5.3', 'glm-4.7', 'kimi-k2.5', 'chatglm_turbo'])
  })

  it('prefers account mapping keys over a GPT catalog and keeps mapped IDs', () => {
    expect(
      resolveCcsImportPickerIDs({
        defaultModel: 'glm-5.3',
        fetchedIDs: ['gpt-6-astra', 'gpt-5.4'],
        accountIDs: ['glm-5.3', 'glm-5.3-flash', 'glm-4.7']
      })
    ).toEqual(['glm-5.3', 'glm-5.3-flash', 'glm-4.7'])
  })

  it('keeps legacy account IDs when they are the only mapped models', () => {
    expect(
      resolveCcsImportPickerIDs({
        defaultModel: 'glm-4.7',
        accountIDs: ['glm-4.7']
      })
    ).toEqual(['glm-4.7'])
  })

  it('filters picker IDs by typed query', () => {
    expect(filterCcsImportModelIDs(['glm-5.3', 'glm-5.3-flash', 'kimi-k2.5'], 'k2')).toEqual([
      'kimi-k2.5'
    ])
    expect(compactCcsImportModelIDs(['glm-5.3', 'glm-4.7'])).toEqual(['glm-5.3'])
  })

  it('parses GET /v1/models data[].id values', () => {
    expect(
      parseGatewayModelsList({
        object: 'list',
        data: [{ id: 'glm-5.3' }, { id: ' gpt-6-astra ' }, { object: 'model' }]
      })
    ).toEqual(['glm-5.3', 'gpt-6-astra'])
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

  describe('launchCcSwitchImportDeeplink', () => {
    afterEach(() => {
      vi.restoreAllMocks()
    })

    it('clicks a temporary anchor with the deeplink (no focus-heuristic failure path)', () => {
      const click = vi.fn()
      const remove = vi.fn()
      const appendChild = vi.spyOn(document.body, 'appendChild').mockImplementation((node) => node)
      const createElement = vi.spyOn(document, 'createElement').mockReturnValue({
        href: '',
        rel: '',
        style: { display: '' },
        click,
        remove
      } as unknown as HTMLAnchorElement)

      const deeplink = 'ccswitch://v1/import?resource=provider&app=codex'
      launchCcSwitchImportDeeplink(deeplink)

      expect(createElement).toHaveBeenCalledWith('a')
      expect(appendChild).toHaveBeenCalled()
      const anchor = createElement.mock.results[0]?.value as HTMLAnchorElement
      expect(anchor.href).toBe(deeplink)
      expect(anchor.rel).toBe('noopener noreferrer')
      expect(click).toHaveBeenCalledTimes(1)
      expect(remove).toHaveBeenCalledTimes(1)
    })
  })
})
