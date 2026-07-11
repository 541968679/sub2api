import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(dir, 'KeysView.vue'), 'utf8')

describe('KeysView column settings contract', () => {
  it('keeps identity/action columns visible and hides low-frequency columns by default', () => {
    expect(source).toContain("const ALWAYS_VISIBLE_COLUMNS = new Set(['name', 'actions'])")
    expect(source).toContain("const DEFAULT_HIDDEN_COLUMNS = ['rate_limit', 'last_used_at', 'last_used_ip']")
    expect(source).toContain("const COLUMN_SETTINGS_VERSION = 2")
    expect(source).toContain("2: ['last_used_ip']")
  })

  it('persists validated hidden columns and exposes a settings menu', () => {
    expect(source).toContain("localStorage.setItem(HIDDEN_COLUMNS_KEY")
    expect(source).toContain("Array.isArray(parsed)")
    expect(source).toContain("!ALWAYS_VISIBLE_COLUMNS.has(key)")
    expect(source).toContain("@click=\"toggleColumn(col.key)\"")
    expect(source).toContain("<Icon name=\"grid\"")
  })
})
