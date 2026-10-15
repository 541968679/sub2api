import { describe, expect, it } from 'vitest'
import { resolveDomesticGroupCategory } from '../domesticGroupCategory'

describe('resolveDomesticGroupCategory', () => {
  it('keeps an explicit category', () => {
    expect(resolveDomesticGroupCategory({ domestic_category: 'qwen', name: 'kimi 线路' })).toBe('qwen')
  })

  it('reads the category from the group name when none is stored', () => {
    expect(resolveDomesticGroupCategory({ name: 'kimi k3 官key' })).toBe('kimi')
    expect(resolveDomesticGroupCategory({ name: 'glm 高速' })).toBe('glm')
    expect(resolveDomesticGroupCategory({ name: '通义千问' })).toBe('qwen')
    expect(resolveDomesticGroupCategory({ name: '国产模型：聚合渠道' })).toBe('mixed')
  })
})
