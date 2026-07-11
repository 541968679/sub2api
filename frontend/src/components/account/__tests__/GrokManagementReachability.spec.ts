import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = (path: string) =>
  readFileSync(resolve(process.cwd(), 'src', path), 'utf8')

describe('Grok management reachability', () => {
  it('registers Grok as an account and group platform with dedicated presentation', () => {
    const types = source('types/index.ts')
    const platformIcon = source('components/common/PlatformIcon.vue')
    const platformBadge = source('components/common/PlatformTypeBadge.vue')
    const platformColors = source('utils/platformColors.ts')

    expect(types).toMatch(/GroupPlatform[^\n]*'grok'/)
    expect(types).toMatch(/AccountPlatform[^\n]*'grok'/)
    expect(platformIcon).toContain("platform === 'grok'")
    expect(platformBadge).toContain("props.platform === 'grok'")
    expect(platformColors).toContain("case 'grok': return 'Grok'")
  })

  it('keeps Grok OAuth reachable from create, edit, and reauthorization flows', () => {
    const createModal = source('components/account/CreateAccountModal.vue')
    const editModal = source('components/account/EditAccountModal.vue')
    const reauthModal = source('components/account/ReAuthAccountModal.vue')
    const oauthFlow = source('components/account/OAuthAuthorizationFlow.vue')
    const adminApi = source('api/admin/index.ts')

    expect(adminApi).toContain("import grokAPI from './grok'")
    expect(createModal).toContain("form.platform = 'grok'")
    expect(createModal).toContain('useGrokOAuth')
    expect(editModal).toContain("account.platform === 'grok'")
    expect(reauthModal).toContain("account.platform === 'grok'")
    expect(oauthFlow).toContain("props.platform === 'grok'")
  })

  it('shows Grok quota and group/model configuration without dropping fork controls', () => {
    const usageCell = source('components/account/AccountUsageCell.vue')
    const groupsView = source('views/admin/GroupsView.vue')
    const createModal = source('components/account/CreateAccountModal.vue')
    const editModal = source('components/account/EditAccountModal.vue')

    expect(usageCell).toContain('GrokQuotaProbeCell')
    expect(groupsView).toContain('value: "grok"')
    expect(groupsView).toContain('value: "openai"')
    expect(createModal).toContain('openai_images_endpoint_enabled')
    expect(createModal).toContain('openai_claude_gpt_bridge_enabled')
    expect(editModal).toContain('openai_images_endpoint_enabled')
    expect(editModal).toContain('openai_claude_gpt_bridge_enabled')
  })

  it('keeps Grok copy in both monolithic locale files', () => {
    expect(source('i18n/locales/zh.ts')).toContain('grok:')
    expect(source('i18n/locales/en.ts')).toContain('grok:')
  })
})
