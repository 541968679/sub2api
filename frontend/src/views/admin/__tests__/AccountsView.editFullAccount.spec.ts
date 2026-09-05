import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const viewSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../AccountsView.vue'),
  'utf8'
)

describe('admin AccountsView edit uses full account', () => {
  it('loads GET /admin/accounts/:id before opening the editor', () => {
    expect(viewSource).toMatch(/handleEdit[\s\S]*accounts\.getById/)
  })

  it('does not assign a lite list row directly into the editor', () => {
    expect(viewSource).not.toMatch(/const handleEdit = \(a: Account\) => \{ edAcc\.value = a; showEdit\.value = true \}/)
  })
})
