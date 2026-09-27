import { describe, expect, it } from 'vitest'
import {
  domesticFamilyOf,
  domesticSectionPlatform,
  getKeyGroupProvider
} from '../keyGroupProviders'

describe('getKeyGroupProvider', () => {
  it('keeps Claude and OpenAI on their own buttons', () => {
    expect(getKeyGroupProvider('anthropic')).toBe('anthropic')
    expect(getKeyGroupProvider('openai')).toBe('openai')
  })

  it('groups domestic coding platforms together', () => {
    expect(getKeyGroupProvider('deepseek')).toBe('domestic')
    expect(getKeyGroupProvider('kimi')).toBe('domestic')
    expect(getKeyGroupProvider('zhipu')).toBe('domestic')
    expect(getKeyGroupProvider('minimax')).toBe('domestic')
  })

  it('puts Gemini, Grok, and Antigravity under other', () => {
    expect(getKeyGroupProvider('gemini')).toBe('other')
    expect(getKeyGroupProvider('grok')).toBe('other')
    expect(getKeyGroupProvider('antigravity')).toBe('other')
  })

  it('does not classify OpenAI groups from their display name or CCS default', () => {
    expect(getKeyGroupProvider({ platform: 'openai', name: '国产模型：deepseek专用分组' })).toBe('openai')
    expect(getKeyGroupProvider({ platform: 'openai', name: '国产模型：聚合渠道', ccs_import_default_model: 'glm-5.3' })).toBe('openai')
    expect(getKeyGroupProvider({ platform: 'openai', name: 'kimi k3 自部署' })).toBe('openai')
    expect(getKeyGroupProvider({ platform: 'anthropic', name: 'gpt 每月3000刀' })).toBe('anthropic')
  })

  it('moves an OpenAI group only when its enabled model list is one domestic family', () => {
    expect(getKeyGroupProvider({
      platform: 'openai',
      name: '渠道 A',
      models_list_config: { enabled: true, models: ['deepseek-v4-pro', 'deepseek-v4-flash'] }
    })).toBe('domestic')
  })
})

describe('domesticFamilyOf', () => {
  it('ignores names and the CCS import default', () => {
    expect(domesticFamilyOf({
      platform: 'openai',
      name: '国产模型：聚合渠道',
      ccs_import_default_model: 'glm-5.3'
    })).toBeNull()
    expect(domesticFamilyOf({
      platform: 'openai',
      name: 'deepseek v4.1专用分组',
      ccs_import_default_model: 'deepseek-v4.1-flash'
    })).toBeNull()
  })

  it('uses an enabled custom model list of a single family', () => {
    expect(domesticFamilyOf({
      platform: 'openai',
      name: '渠道 A',
      models_list_config: { enabled: true, models: ['glm-5.3', 'glm-5.3-flash'] }
    })).toBe('glm')
    expect(domesticSectionPlatform({
      platform: 'openai',
      name: '渠道 A',
      models_list_config: { enabled: true, models: ['glm-5.3'] }
    })).toBe('zhipu')
  })

  it('keeps a mixed enabled list out of a single vendor section', () => {
    expect(domesticFamilyOf({
      platform: 'openai',
      models_list_config: { enabled: true, models: ['glm-5.3', 'deepseek-v4-pro'] }
    })).toBe('mixed')
    expect(domesticSectionPlatform({
      platform: 'openai',
      models_list_config: { enabled: true, models: ['glm-5.3', 'deepseek-v4-pro'] }
    })).toBe('domestic-mixed')
  })

  it('ignores a disabled model list that still contains domestic IDs', () => {
    expect(domesticFamilyOf({
      platform: 'openai',
      name: 'gpt 月卡',
      models_list_config: { enabled: false, models: ['glm-5.3', 'gpt-5.5'] }
    })).toBeNull()
  })
})
