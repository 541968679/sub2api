import { describe, expect, it } from 'vitest'
import { accountListQueryForPlatformView } from '../accountPlatformView'

const groups = [
  { id: 9, platform: 'openai', name: 'gpt 每月2000刀' },
  { id: 46, platform: 'openai', name: '国产模型：聚合渠道' },
  { id: 48, platform: 'openai', name: 'kimi k3 官key' }
]

describe('accountListQueryForPlatformView', () => {
  it('asks for domestic group ids without sending platform=domestic', () => {
    expect(accountListQueryForPlatformView({ platform: 'domestic', type: 'apikey' }, groups)).toEqual({
      type: 'apikey',
      group: '',
      group_ids: '46,48'
    })
  })

  it('keeps a chosen domestic group on the existing group query', () => {
    expect(accountListQueryForPlatformView({ platform: 'domestic', group: '48' }, groups)).toEqual({
      group: '48'
    })
  })

  it('hides domestic groups from the OpenAI all-groups view', () => {
    expect(accountListQueryForPlatformView({ platform: 'openai' }, groups)).toMatchObject({
      platform: 'openai',
      exclude_group_ids: '46,48'
    })
  })

  it('leaves a chosen OpenAI group alone', () => {
    expect(accountListQueryForPlatformView({ platform: 'openai', group: '9' }, groups)).toEqual({
      platform: 'openai',
      group: '9'
    })
  })

  it('matches nothing when the domestic view has no groups', () => {
    expect(accountListQueryForPlatformView({ platform: 'domestic' }, [])).toMatchObject({
      group: '',
      group_ids: '0'
    })
  })
})
