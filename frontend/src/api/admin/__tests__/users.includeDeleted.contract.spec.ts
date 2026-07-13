import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(
  resolve(process.cwd(), 'src/api/admin/users.ts'),
  'utf8'
)

describe('admin users API include-deleted contract', () => {
  it('can request a soft-deleted user explicitly without changing the default path', () => {
    expect(source).toContain('export async function getById(id: number, includeDeleted = false)')
    expect(source).toContain('includeDeleted ? `/admin/users/${id}?include_deleted=true`')
  })
})
