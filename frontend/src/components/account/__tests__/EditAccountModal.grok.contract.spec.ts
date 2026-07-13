import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(
  resolve(process.cwd(), 'src/components/account/EditAccountModal.vue'),
  'utf8'
)

describe('EditAccountModal Grok API-key defaults', () => {
  it('uses the official xAI base URL and key placeholder', () => {
    expect(source).toContain("account.platform === 'grok'")
    expect(source).toContain("? 'https://api.x.ai/v1'")
    expect(source).toContain("? 'xai-...'")
  })
})
