import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const createSource = readFileSync(resolve(dir, 'UserCreateModal.vue'), 'utf8')
const editSource = readFileSync(resolve(dir, 'UserEditModal.vue'), 'utf8')

describe('admin user role management', () => {
  it('defaults new users to user and offers only user/admin roles', () => {
    expect(createSource).toContain("role: 'user' as 'admin' | 'user'")
    expect(createSource).toContain('<option value="user">')
    expect(createSource).toContain('<option value="admin">')
    expect(createSource).toContain("role: 'user'")
  })

  it('loads and submits the edited role', () => {
    expect(editSource).toContain('role: u.role')
    expect(editSource).toContain('role: form.role')
    expect(editSource).toContain('v-model="form.role"')
  })
})
