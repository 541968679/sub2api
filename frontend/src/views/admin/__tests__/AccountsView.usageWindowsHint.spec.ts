import { describe, expect, it } from 'vitest'

import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'

describe('admin account usage window help', () => {
  it('keeps bilingual explanatory copy for the 5h/7d upstream windows', () => {
    expect(en.admin.accounts.usageWindowsHint).toContain('5h / 7d')
    expect(zh.admin.accounts.usageWindowsHint).toContain('5h / 7d')
  })
})
