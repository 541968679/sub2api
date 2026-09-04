import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const viewSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../AccountsView.vue'),
  'utf8'
)

describe('admin AccountsView Codex session import entry', () => {
  it('does not keep a toolbar button or modal for Codex session import', () => {
    expect(viewSource).not.toContain('showCodexSessionImport')
    expect(viewSource).not.toContain('CodexSessionImportModal')
    expect(viewSource).not.toContain('codexSessionImport.action')
    expect(viewSource).toContain('admin.accounts.dataImport')
  })
})
