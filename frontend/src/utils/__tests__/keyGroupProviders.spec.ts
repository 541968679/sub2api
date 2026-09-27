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

  it('moves OpenAI-platform domestic groups out of the OpenAI button', () => {
    expect(getKeyGroupProvider({ platform: 'openai', name: '国产模型：deepseek专用分组' })).toBe('domestic')
    expect(getKeyGroupProvider({ platform: 'openai', name: 'kimi k3 自部署' })).toBe('domestic')
    expect(getKeyGroupProvider({ platform: 'openai', name: 'gpt 每日 300刀' })).toBe('openai')
    expect(getKeyGroupProvider({ platform: 'anthropic', name: 'gpt 每月3000刀' })).toBe('anthropic')
  })
})

describe('domesticFamilyOf', () => {
  it('reads the family from the group name before the CCS default model', () => {
    expect(domesticFamilyOf({
      platform: 'openai',
      name: '国产模型：聚合渠道',
      ccs_import_default_model: 'glm-5.3'
    })).toBe('mixed')
    expect(domesticFamilyOf({
      platform: 'openai',
      name: 'deepseek v4.1专用分组（即将废弃）',
      ccs_import_default_model: 'deepseek-v4.1-flash'
    })).toBe('deepseek')
  })

  it('uses an enabled custom model list when the name is generic', () => {
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

  it('ignores a disabled model list that still contains domestic IDs', () => {
    expect(domesticFamilyOf({
      platform: 'openai',
      name: 'gpt 月卡',
      models_list_config: { enabled: false, models: ['glm-5.3', 'gpt-5.5'] }
    })).toBeNull()
  })
})
