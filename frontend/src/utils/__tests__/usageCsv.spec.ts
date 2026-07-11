import { describe, expect, it } from 'vitest'

import { buildUserUsageCsvBytes } from '../usageCsv'

describe('buildUserUsageCsvBytes', () => {
  it('writes an UTF-8 BOM and preserves Chinese text for Excel', () => {
    const bytes = buildUserUsageCsvBytes(
      ['时间', '模型', '缓存读取', '已计费费用'],
      [['2026-07-11 12:00', '模型-测试', 13, 0.75]],
    )

    expect([...bytes.slice(0, 3)]).toEqual([0xef, 0xbb, 0xbf])
    expect(new TextDecoder().decode(bytes.slice(3))).toBe(
      '时间,模型,缓存读取,已计费费用\r\n2026-07-11 12:00,模型-测试,13,0.75',
    )
  })

  it('escapes CSV delimiters and spreadsheet formulas', () => {
    const bytes = buildUserUsageCsvBytes(
      ['Model', 'Endpoint'],
      [['=cmd|test', '/v1/messages,stream']],
    )
    const csv = new TextDecoder().decode(bytes.slice(3))

    expect(csv).toContain("'=cmd|test")
    expect(csv).toContain('"/v1/messages,stream"')
    expect(csv).not.toContain('Actual Cost')
    expect(csv).not.toContain('Account Cost')
  })
})
