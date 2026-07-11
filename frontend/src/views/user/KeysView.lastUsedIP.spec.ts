import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(dir, 'KeysView.vue'), 'utf8')

describe('KeysView last-used IP', () => {
  it('exposes the API field as a non-sortable column with an empty fallback', () => {
    expect(source).toContain("{ key: 'last_used_ip', label: t('keys.lastUsedIP'), sortable: false }")
    expect(source).toContain('<template #cell-last_used_ip="{ value }">')
    expect(source).toContain('<span v-if="value"')
  })
})
