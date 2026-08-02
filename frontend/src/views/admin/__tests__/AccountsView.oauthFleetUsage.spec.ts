import { describe, expect, it } from 'vitest'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'

describe('AccountsView oauth fleet usage i18n', () => {
  it('labels used (not remaining) and explains weighted sum + bars', () => {
    expect(en.admin.accounts.oauthFleetUsage.title).toContain('OAI')
    expect(zh.admin.accounts.oauthFleetUsage.title).toContain('OAI')
    expect(en.admin.accounts.oauthFleetUsage.usedBadge).toMatch(/Used/i)
    expect(zh.admin.accounts.oauthFleetUsage.usedBadge).toBe('已用')
    expect(en.admin.accounts.oauthFleetUsage.tooltip).toMatch(/used percentage \(not remaining\)/i)
    expect(zh.admin.accounts.oauthFleetUsage.tooltip).toMatch(/已用.*非剩余/)
    expect(en.admin.accounts.oauthFleetUsage.tooltip).toMatch(/Prolite/i)
    expect(zh.admin.accounts.oauthFleetUsage.tooltip).toMatch(/Prolite|1\/4/)
    expect(en.admin.accounts.oauthFleetUsage.detail).toContain('prolite')
    expect(zh.admin.accounts.oauthFleetUsage.detail).toContain('prolite')
  })
})
