import { describe, expect, it } from 'vitest'
import { extractFirstHttpUrl, linkifyHttpText } from '../httpTextLinks'

const productionNotice = `线上支付渠道暂不可用
请直接从
https://pay.ldxp.cn/shop/8AJ8M782
购买兑换码进行兑换即可
谢谢各位老板支持！`

describe('httpTextLinks', () => {
  it('reads the first http(s) url from a card-shop notice', () => {
    expect(extractFirstHttpUrl(productionNotice)).toBe('https://pay.ldxp.cn/shop/8AJ8M782')
  })

  it('drops trailing punctuation without keeping it in the href', () => {
    expect(extractFirstHttpUrl('购买：https://pay.example.com/shop/abc。')).toBe(
      'https://pay.example.com/shop/abc',
    )
  })

  it('ignores credentialed and non-http urls', () => {
    expect(extractFirstHttpUrl('https://user:pass@pay.example.com/shop')).toBeNull()
    expect(extractFirstHttpUrl('javascript:alert(1)')).toBeNull()
    expect(extractFirstHttpUrl('请联系客服')).toBeNull()
  })

  it('splits notice text into plain parts and safe links', () => {
    const parts = linkifyHttpText('打开 https://pay.example.com/shop/abc。 然后兑换')
    expect(parts).toEqual([
      { text: '打开 ' },
      { text: 'https://pay.example.com/shop/abc', href: 'https://pay.example.com/shop/abc' },
      { text: '。' },
      { text: ' 然后兑换' },
    ])
  })
})
