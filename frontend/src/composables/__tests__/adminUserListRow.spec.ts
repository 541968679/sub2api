import { describe, expect, it } from 'vitest'
import { smartScheduleSummaryFromDrafts } from '../adminUserListRow'

describe('smartScheduleSummaryFromDrafts', () => {
  it('only lists enabled platforms that have pool members', () => {
    const summary = smartScheduleSummaryFromDrafts(['anthropic', 'openai'], {
      anthropic: { enabled: true, accounts: [{}, {}] },
      openai: { enabled: true, accounts: [] }
    })
    expect(summary.enabled_platforms).toEqual(['anthropic'])
    expect(summary.pool_counts).toEqual({ anthropic: 2, openai: 0 })
  })
})
