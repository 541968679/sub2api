import { describe, expect, it } from 'vitest'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'

describe('AccountsView oauth fleet usage i18n', () => {
  it('labels used/capacity pool model and fraction display', () => {
    expect(en.admin.accounts.oauthFleetUsage.title).toContain('OAI')
    expect(zh.admin.accounts.oauthFleetUsage.title).toContain('OAI')
    expect(en.admin.accounts.oauthFleetUsage.usedBadge).toMatch(/Used\/Cap/i)
    expect(zh.admin.accounts.oauthFleetUsage.usedBadge).toMatch(/已用\/容量/)
    expect(en.admin.accounts.oauthFleetUsage.fraction).toContain('{used}/{capacity}')
    expect(zh.admin.accounts.oauthFleetUsage.fraction).toContain('{used}/{capacity}')
    expect(en.admin.accounts.oauthFleetUsage.tooltip).toMatch(/375\/725/)
    expect(zh.admin.accounts.oauthFleetUsage.tooltip).toMatch(/375\/725/)
    expect(en.admin.accounts.oauthFleetUsage.tooltip).toMatch(/not remaining/i)
    expect(zh.admin.accounts.oauthFleetUsage.tooltip).toMatch(/非剩余/)
    expect(en.admin.accounts.oauthFleetUsage.detail).toContain('prolite')
    expect(zh.admin.accounts.oauthFleetUsage.detail).toContain('prolite')
  })
})
