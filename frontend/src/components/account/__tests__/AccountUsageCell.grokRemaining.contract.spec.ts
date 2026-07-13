import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(
  resolve(process.cwd(), 'src/components/account/AccountUsageCell.vue'),
  'utf8'
)

describe('AccountUsageCell Grok quota presentation', () => {
  it('passes remaining-capacity semantics and computes remaining percentage', () => {
    expect(source.match(/:remaining-capacity="true"/g)).toHaveLength(2)
    expect(source).toContain('const remaining = Math.min(quota.limit, Math.max(0, quota.remaining))')
    expect(source).toContain('utilization: (remaining / quota.limit) * 100')
  })
})
