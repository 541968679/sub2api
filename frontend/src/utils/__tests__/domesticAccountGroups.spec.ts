import { describe, expect, it } from 'vitest'
import { isDomesticAccountGroup } from '../domesticAccountGroups'

describe('isDomesticAccountGroup', () => {
  it('keeps GPT OpenAI groups out of the domestic list', () => {
    for (const name of [
      'gpt 每月2000刀',
      'gpt image 2 高质量',
      'GPT PRO 每日100刀（一个月）',
      '管理员专属',
      'gpt pro 草哥专属！'
    ]) {
      expect(isDomesticAccountGroup({ platform: 'openai', name })).toBe(false)
    }
  })

  it('matches domestic groups that share the OpenAI platform', () => {
    for (const name of [
      '国产模型：聚合渠道',
      '国产模型：deepseek专用分组',
      '国产模型：qwen系列',
      'kimi k3 官key',
      'kimi k3 自部署b300',
      'glm 高速分组（官转限时特惠）',
      'glm自部署',
      'glm 官转高质量分组',
      '其他国模'
    ]) {
      expect(isDomesticAccountGroup({ platform: 'openai', name })).toBe(true)
    }
  })

  it('does not pull other platforms in by a similar name', () => {
    expect(isDomesticAccountGroup({ platform: 'anthropic', name: 'claude特价不降智' })).toBe(false)
    expect(isDomesticAccountGroup({ platform: 'grok', name: 'grok测试' })).toBe(false)
    expect(isDomesticAccountGroup({ platform: 'antigravity', name: 'claude 每日800刀' })).toBe(false)
  })

  it('treats dedicated domestic platforms and mapped domestic model ids as domestic', () => {
    expect(isDomesticAccountGroup({ platform: 'kimi', name: '线路' })).toBe(true)
    expect(isDomesticAccountGroup({
      platform: 'openai',
      name: '高速渠道',
      default_mapped_model: 'glm-5.3'
    })).toBe(true)
    expect(isDomesticAccountGroup({
      platform: 'openai',
      name: '高速渠道',
      allowed_models: ['deepseek-v4-flash']
    })).toBe(true)
  })
})
