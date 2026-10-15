import { describe, expect, it } from 'vitest'
import {
  domesticPlatformDefaultBaseURL,
  groupPlatformChoice,
  isDomesticStoredPlatform,
  storedPlatformForGroupChoice
} from '../domesticPlatformChoice'

describe('domesticPlatformChoice', () => {
  it('treats the four vendor platforms as one 国产模型 choice', () => {
    expect(isDomesticStoredPlatform('kimi')).toBe(true)
    expect(isDomesticStoredPlatform('OpenAI')).toBe(false)
    expect(groupPlatformChoice('zhipu')).toBe('domestic')
    expect(groupPlatformChoice('openai')).toBe('openai')
  })

  it('stores openai when the 国产模型 choice is selected', () => {
    expect(storedPlatformForGroupChoice('domestic', 'anthropic')).toBe('openai')
    expect(storedPlatformForGroupChoice('domestic', 'kimi')).toBe('openai')
    expect(storedPlatformForGroupChoice('openai', 'kimi')).toBe('openai')
    expect(storedPlatformForGroupChoice('domestic-nope', 'gemini')).toBe('gemini')
    expect(groupPlatformChoice('openai', 'kimi')).toBe('domestic')
    expect(groupPlatformChoice('openai', '')).toBe('openai')
    expect(groupPlatformChoice('kimi')).toBe('domestic')
  })

  it('uses the official pay-as-you-go base URL for each vendor', () => {
    expect(domesticPlatformDefaultBaseURL('kimi')).toBe('https://api.moonshot.cn/v1')
    expect(domesticPlatformDefaultBaseURL('zhipu')).toBe('https://open.bigmodel.cn/api/paas/v4')
    expect(domesticPlatformDefaultBaseURL('deepseek')).toBe('https://api.deepseek.com')
    expect(domesticPlatformDefaultBaseURL('minimax')).toBe('https://api.minimaxi.com/v1')
    expect(domesticPlatformDefaultBaseURL('openai')).toBeUndefined()
  })
})
